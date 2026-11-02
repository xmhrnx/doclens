package scan

import (
	"image"
	"image/draw"
	"math"
	"runtime"
	"sync"
)

// plane is one float channel of an image, used for analysis.
type plane struct {
	W, H int
	Pix  []float32
}

func newPlane(w, h int) *plane { return &plane{W: w, H: h, Pix: make([]float32, w*h)} }

func (p *plane) at(x, y int) float32 {
	if x < 0 {
		x = 0
	} else if x >= p.W {
		x = p.W - 1
	}
	if y < 0 {
		y = 0
	} else if y >= p.H {
		y = p.H - 1
	}
	return p.Pix[y*p.W+x]
}

// bilinear samples the plane at a fractional position.
func (p *plane) bilinear(x, y float64) float32 {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := float32(x-float64(x0)), float32(y-float64(y0))
	a := p.at(x0, y0)*(1-fx) + p.at(x0+1, y0)*fx
	b := p.at(x0, y0+1)*(1-fx) + p.at(x0+1, y0+1)*fx
	return a*(1-fy) + b*fy
}

func (p *plane) clone() *plane {
	q := newPlane(p.W, p.H)
	copy(q.Pix, p.Pix)
	return q
}

// percentile returns the value below which frac of the plane's values
// lie, ignoring values <= floor. It uses a fine histogram, so it costs
// one pass instead of a sort.
func (p *plane) percentile(frac float64, floor float32) float32 {
	hi := floor
	n := 0
	for _, v := range p.Pix {
		if v > floor {
			n++
			if v > hi {
				hi = v
			}
		}
	}
	if n == 0 {
		return 0
	}
	const bins = 4096
	var hist [bins]int32
	scale := float32(bins-1) / (hi - floor)
	for _, v := range p.Pix {
		if v > floor {
			hist[int((v-floor)*scale)]++
		}
	}
	target := int(frac * float64(n-1))
	seen := 0
	for b, c := range hist {
		seen += int(c)
		if seen > target {
			return floor + (float32(b)+0.5)/scale
		}
	}
	return hi
}

// rgbPlanes is an image split into float RGB planes in [0, 1].
type rgbPlanes [3]*plane

// toRGBA converts any image to *image.RGBA with its origin at 0,0.
func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok && r.Rect.Min == (image.Point{}) {
		return r
	}
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Rect, img, b.Min, draw.Src)
	return out
}

// downscaleRGB area-averages img so its longer side is at most
// maxSide, returning the planes and the scale factor applied
// (planes = image * scale).
func downscaleRGB(img *image.RGBA, maxSide int) (rgbPlanes, float64) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	scale := 1.0
	if m := max(w, h); m > maxSide {
		scale = float64(maxSide) / float64(m)
	}
	ow, oh := max(1, int(math.Round(float64(w)*scale))), max(1, int(math.Round(float64(h)*scale)))
	sx, sy := float64(w)/float64(ow), float64(h)/float64(oh)
	var out rgbPlanes
	for c := range out {
		out[c] = newPlane(ow, oh)
	}
	parallelRows(oh, func(y0, y1 int) {
		for oy := y0; oy < y1; oy++ {
			ya, yb := int(float64(oy)*sy), int(float64(oy+1)*sy)
			if yb <= ya {
				yb = ya + 1
			}
			yb = min(yb, h)
			for ox := 0; ox < ow; ox++ {
				xa, xb := int(float64(ox)*sx), int(float64(ox+1)*sx)
				if xb <= xa {
					xb = xa + 1
				}
				xb = min(xb, w)
				var r, g, b, n uint32
				for y := ya; y < yb; y++ {
					row := img.Pix[y*img.Stride:]
					for x := xa; x < xb; x++ {
						i := x * 4
						r += uint32(row[i])
						g += uint32(row[i+1])
						b += uint32(row[i+2])
						n++
					}
				}
				inv := 1 / (255 * float32(n))
				i := oy*ow + ox
				out[0].Pix[i] = float32(r) * inv
				out[1].Pix[i] = float32(g) * inv
				out[2].Pix[i] = float32(b) * inv
			}
		}
	})
	return out, float64(ow) / float64(w)
}

func (p rgbPlanes) luma() *plane {
	out := newPlane(p[0].W, p[0].H)
	for i := range out.Pix {
		out.Pix[i] = 0.299*p[0].Pix[i] + 0.587*p[1].Pix[i] + 0.114*p[2].Pix[i]
	}
	return out
}

// gaussianBlur blurs the plane with a separable Gaussian kernel.
func gaussianBlur(p *plane, sigma float64) *plane {
	if sigma <= 0 {
		return p.clone()
	}
	r := int(math.Ceil(sigma * 3))
	k := make([]float32, 2*r+1)
	sum := float32(0)
	for i := -r; i <= r; i++ {
		v := float32(math.Exp(-float64(i*i) / (2 * sigma * sigma)))
		k[i+r] = v
		sum += v
	}
	for i := range k {
		k[i] /= sum
	}
	tmp := newPlane(p.W, p.H)
	parallelRows(p.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < p.W; x++ {
				s := float32(0)
				for i := -r; i <= r; i++ {
					s += k[i+r] * p.at(x+i, y)
				}
				tmp.Pix[y*p.W+x] = s
			}
		}
	})
	out := newPlane(p.W, p.H)
	parallelRows(p.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < p.W; x++ {
				s := float32(0)
				for i := -r; i <= r; i++ {
					s += k[i+r] * tmp.at(x, y+i)
				}
				out.Pix[y*p.W+x] = s
			}
		}
	})
	return out
}

// morph applies a separable square max (dilate) or min (erode) filter
// of the given radius, with the van Herk/Gil-Werman algorithm: three
// comparisons per pixel whatever the radius.
func morph(p *plane, r int, dilate bool) *plane {
	w, h := p.W, p.H
	tmp := newPlane(w, h)
	parallelRows(h, func(y0, y1 int) {
		buf := newLineFilter(w, r, dilate)
		for y := y0; y < y1; y++ {
			buf.run(p.Pix[y*w:(y+1)*w], tmp.Pix[y*w:(y+1)*w])
		}
	})
	out := newPlane(w, h)
	// Columns: gather each into a line, filter, scatter back.
	parallelRows(w, func(x0, x1 int) {
		buf := newLineFilter(h, r, dilate)
		col, res := make([]float32, h), make([]float32, h)
		for x := x0; x < x1; x++ {
			for y := 0; y < h; y++ {
				col[y] = tmp.Pix[y*w+x]
			}
			buf.run(col, res)
			for y := 0; y < h; y++ {
				out.Pix[y*w+x] = res[y]
			}
		}
	})
	return out
}

// lineFilter is a running max or min over a window of 2r+1 values.
type lineFilter struct {
	r      int
	dilate bool
	pre    []float32 // prefix extremes within each block
	suf    []float32 // suffix extremes within each block
	pad    []float32
}

func newLineFilter(n, r int, dilate bool) *lineFilter {
	m := n + 2*r
	return &lineFilter{r: r, dilate: dilate, pre: make([]float32, m), suf: make([]float32, m), pad: make([]float32, m)}
}

func (f *lineFilter) pick(a, b float32) float32 {
	if f.dilate == (b > a) {
		return b
	}
	return a
}

func (f *lineFilter) run(in, out []float32) {
	r, n := f.r, len(in)
	k := 2*r + 1
	m := n + 2*r
	pad := f.pad[:m]
	// Edges repeat the end values.
	for i := 0; i < r; i++ {
		pad[i], pad[m-1-i] = in[0], in[n-1]
	}
	copy(pad[r:], in)
	for i := 0; i < m; i++ {
		if i%k == 0 {
			f.pre[i] = pad[i]
		} else {
			f.pre[i] = f.pick(f.pre[i-1], pad[i])
		}
	}
	for i := m - 1; i >= 0; i-- {
		if i == m-1 || (i+1)%k == 0 {
			f.suf[i] = pad[i]
		} else {
			f.suf[i] = f.pick(f.suf[i+1], pad[i])
		}
	}
	// The window [i, i+k) spans at most two blocks.
	for i := 0; i < n; i++ {
		out[i] = f.pick(f.suf[i], f.pre[i+k-1])
	}
}

// closing removes small dark details (text, lines) while keeping
// large shapes and their edges.
func closing(p *plane, r int) *plane { return morph(morph(p, r, true), r, false) }

// parallelRows splits [0, h) into bands and runs fn on them
// concurrently.
func parallelRows(h int, fn func(y0, y1 int)) {
	n := runtime.GOMAXPROCS(0)
	if n > h {
		n = h
	}
	if n <= 1 || h < 64 {
		fn(0, h)
		return
	}
	var wg sync.WaitGroup
	step := (h + n - 1) / n
	for y := 0; y < h; y += step {
		wg.Add(1)
		go func(a, b int) {
			defer wg.Done()
			fn(a, b)
		}(y, min(h, y+step))
	}
	wg.Wait()
}

// otsu returns the threshold that best splits values in [0, 1] into
// two classes.
func otsu(vals []float32) float32 {
	var hist [256]float64
	for _, v := range vals {
		i := int(v * 255)
		if i < 0 {
			i = 0
		} else if i > 255 {
			i = 255
		}
		hist[i]++
	}
	total := float64(len(vals))
	sumAll := 0.0
	for i, c := range hist {
		sumAll += float64(i) * c
	}
	var wB, sumB, best float64
	thr := 128
	for i, c := range hist {
		wB += c
		if wB == 0 {
			continue
		}
		wF := total - wB
		if wF == 0 {
			break
		}
		sumB += float64(i) * c
		mB, mF := sumB/wB, (sumAll-sumB)/wF
		between := wB * wF * (mB - mF) * (mB - mF)
		if between > best {
			best, thr = between, i
		}
	}
	return float32(thr) / 255
}
