package scan

import (
	"image"
	"math"
	"sort"
)

// Filter is a look applied to a flattened page.
type Filter string

const (
	// FilterOriginal keeps the photo's colors.
	FilterOriginal Filter = "original"
	// FilterMagic evens out the lighting, whitens the paper, deepens
	// the ink and lifts colors: the usual choice for documents.
	FilterMagic Filter = "magic"
	// FilterNoShadow evens out the lighting and removes shadows while
	// keeping natural colors, for photos, drawings and colored paper.
	FilterNoShadow Filter = "noshadow"
	// FilterGray is a clean grayscale page.
	FilterGray Filter = "gray"
	// FilterBW is crisp black text on white, the smallest files and
	// best for printing and faxing.
	FilterBW Filter = "bw"
)

// Filters lists every filter in display order.
var Filters = []Filter{FilterMagic, FilterOriginal, FilterNoShadow, FilterGray, FilterBW}

// Adjust are manual corrections applied after the filter.
type Adjust struct {
	Brightness float64 // -1..1
	Contrast   float64 // -1..1
}

// IsGray reports whether the filter produces a grayscale page.
func (f Filter) IsGray() bool { return f == FilterGray || f == FilterBW }

// Enhance applies a filter and adjustments to a flattened page.
func Enhance(img image.Image, f Filter, adj Adjust) image.Image {
	src := toRGBA(img)
	var out image.Image
	switch f {
	case FilterMagic:
		out = magicColor(src)
	case FilterNoShadow:
		out = noShadow(src)
	case FilterGray:
		out = grayscale(src)
	case FilterBW:
		out = blackWhite(src)
	default:
		out = cloneRGBA(src)
	}
	if adj != (Adjust{}) {
		applyAdjust(out, adj)
	}
	return out
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Rect)
	copy(out.Pix, src.Pix)
	return out
}

// illumination estimates the paper's color under the photo's light at
// every pixel: the bright background with the ink taken away. It works
// on a coarse grid - block means, then a max filter that removes text
// strokes, a min filter that undoes the growth of large dark areas, and
// a blur - and is sampled bilinearly at full resolution.
type illumination struct {
	grid  rgbPlanes
	sx    float64 // grid cells per pixel
	sy    float64
	paper [3]float32 // the overall paper color
}

func estimateIllumination(src *image.RGBA) *illumination {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	side := max(64, min(220, max(w, h)/6))
	planes, _ := downscaleRGB(src, side)
	gw, gh := planes[0].W, planes[0].H
	// Pick, per window, the brightest cell's color (by luma), so
	// colors stay consistent across channels.
	l := planes.luma()
	r := max(2, max(gw, gh)/40)
	var lit rgbPlanes
	for c := range lit {
		lit[c] = newPlane(gw, gh)
	}
	idx := brightestIndex(l, r)
	for i, j := range idx {
		for c := range lit {
			lit[c].Pix[i] = planes[c].Pix[j]
		}
	}
	for c := range lit {
		lit[c] = gaussianBlur(morph(lit[c], r/2+1, false), float64(r)*0.8)
	}
	// Paper color: the bright end of the estimate.
	ll := lit.luma()
	thr := ll.percentile(0.85, 0)
	var paper [3]float32
	n := float32(0)
	for i, v := range ll.Pix {
		if v >= thr {
			for c := range paper {
				paper[c] += lit[c].Pix[i]
			}
			n++
		}
	}
	for c := range paper {
		paper[c] /= max(n, 1)
	}
	// Large dark areas (photos, dark headers) are not shadows: don't
	// let the estimate sink far below the paper there.
	for i, v := range ll.Pix {
		floor := float32(0.5) * thr
		if v < floor && v > 0 {
			k := floor / v
			for c := range lit {
				lit[c].Pix[i] = min(lit[c].Pix[i]*k, paper[c])
			}
		}
	}
	return &illumination{grid: lit, sx: float64(gw) / float64(w), sy: float64(gh) / float64(h), paper: paper}
}

// brightestIndex returns, for every cell, the index of the brightest
// cell within radius r.
func brightestIndex(l *plane, r int) []int {
	w, h := l.W, l.H
	tmp := make([]int, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			best := y*w + x
			for dx := -r; dx <= r; dx++ {
				nx := x + dx
				if nx < 0 || nx >= w {
					continue
				}
				if j := y*w + nx; l.Pix[j] > l.Pix[best] {
					best = j
				}
			}
			tmp[y*w+x] = best
		}
	}
	out := make([]int, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			best := tmp[y*w+x]
			for dy := -r; dy <= r; dy++ {
				ny := y + dy
				if ny < 0 || ny >= h {
					continue
				}
				if j := tmp[ny*w+x]; l.Pix[j] > l.Pix[best] {
					best = j
				}
			}
			out[y*w+x] = best
		}
	}
	return out
}

// normalize divides every pixel by the light falling on it, so the
// paper becomes uniformly white (1.0) and shadows disappear. It calls
// fn with each pixel's normalized RGB.
func (il *illumination) normalize(src *image.RGBA, fn func(i int, r, g, b float32)) {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	parallelRows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			gy := (float64(y)+0.5)*il.sy - 0.5
			row := src.Pix[y*src.Stride:]
			for x := 0; x < w; x++ {
				gx := (float64(x)+0.5)*il.sx - 0.5
				br := max(il.grid[0].bilinear(gx, gy), 0.04)
				bg := max(il.grid[1].bilinear(gx, gy), 0.04)
				bb := max(il.grid[2].bilinear(gx, gy), 0.04)
				p := row[x*4:]
				fn(y*w+x, float32(p[0])/255/br, float32(p[1])/255/bg, float32(p[2])/255/bb)
			}
		}
	})
}

// inkLevel returns the normalized luminance below which the darkest
// half percent of the page lies: the ink's own black.
func inkLevel(lum []float32) float32 {
	step := max(1, len(lum)/200000)
	sample := make([]float32, 0, len(lum)/step+1)
	for i := 0; i < len(lum); i += step {
		sample = append(sample, lum[i])
	}
	sort.Slice(sample, func(i, j int) bool { return sample[i] < sample[j] })
	return sample[len(sample)/200]
}

func magicColor(src *image.RGBA) *image.RGBA {
	il := estimateIllumination(src)
	w, h := src.Rect.Dx(), src.Rect.Dy()
	rgb := make([]float32, w*h*3)
	lum := make([]float32, w*h)
	il.normalize(src, func(i int, r, g, b float32) {
		rgb[i*3], rgb[i*3+1], rgb[i*3+2] = r, g, b
		lum[i] = 0.299*r + 0.587*g + 0.114*b
	})
	black := min(inkLevel(lum), 0.45)
	const white = 0.9
	out := image.NewRGBA(src.Rect)
	curve := levelCurve(black, white, 1.25)
	parallelRows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				i := y*w + x
				r, g, b := rgb[i*3], rgb[i*3+1], rgb[i*3+2]
				l := lum[i]
				// Lift saturation of colored ink and stamps.
				const sat = 1.45
				r, g, b = l+(r-l)*sat, l+(g-l)*sat, l+(b-l)*sat
				o := out.Pix[y*out.Stride+x*4:]
				o[0], o[1], o[2], o[3] = curve(r), curve(g), curve(b), 255
			}
		}
	})
	sharpen(out, 0.55)
	return out
}

func noShadow(src *image.RGBA) *image.RGBA {
	il := estimateIllumination(src)
	w := src.Rect.Dx()
	out := image.NewRGBA(src.Rect)
	// Bring the paper back to its own color rather than pure white,
	// slightly brightened.
	pr, pg, pb := il.paper[0], il.paper[1], il.paper[2]
	pl := 0.299*pr + 0.587*pg + 0.114*pb
	lift := min(1/max(pl, 0.01), 1.18)
	il.normalize(src, func(i int, r, g, b float32) {
		x, y := i%w, i/w
		o := out.Pix[y*out.Stride+x*4:]
		o[0] = to8(r * pr * lift)
		o[1] = to8(g * pg * lift)
		o[2] = to8(b * pb * lift)
		o[3] = 255
	})
	return out
}

func grayscale(src *image.RGBA) *image.Gray {
	il := estimateIllumination(src)
	w, h := src.Rect.Dx(), src.Rect.Dy()
	lum := make([]float32, w*h)
	il.normalize(src, func(i int, r, g, b float32) {
		lum[i] = 0.299*r + 0.587*g + 0.114*b
	})
	black := min(inkLevel(lum), 0.45)
	curve := levelCurve(black, 0.92, 1.15)
	out := image.NewGray(src.Rect)
	for i, v := range lum {
		x, y := i%w, i/w
		out.Pix[y*out.Stride+x] = curve(v)
	}
	sharpenGray(out, 0.5)
	return out
}

func blackWhite(src *image.RGBA) *image.Gray {
	il := estimateIllumination(src)
	w, h := src.Rect.Dx(), src.Rect.Dy()
	lum := make([]float32, w*h)
	il.normalize(src, func(i int, r, g, b float32) {
		lum[i] = min(0.299*r+0.587*g+0.114*b, 1)
	})
	// After normalization the paper sits near 1 and ink well below: a
	// global split works, clamped to sane values for nearly blank or
	// nearly full pages.
	step := max(1, len(lum)/200000)
	sample := make([]float32, 0, len(lum)/step+1)
	for i := 0; i < len(lum); i += step {
		sample = append(sample, lum[i])
	}
	t := otsu(sample)
	t = max(0.55, min(0.82, t))
	out := image.NewGray(src.Rect)
	// A narrow ramp instead of a hard step keeps letter edges smooth.
	lo, hi := t-0.07, t+0.02
	for i, v := range lum {
		x, y := i%w, i/w
		var o float32
		switch {
		case v <= lo:
			o = 0
		case v >= hi:
			o = 1
		default:
			s := (v - lo) / (hi - lo)
			o = s * s * (3 - 2*s)
		}
		out.Pix[y*out.Stride+x] = to8(o)
	}
	return out
}

// levelCurve maps black..white to 0..1 with a gamma that deepens
// midtones, as an 8-bit lookup.
func levelCurve(black, white float32, gamma float64) func(float32) uint8 {
	var lut [1024]uint8
	for i := range lut {
		v := float64(i) / float64(len(lut)-1) * 1.5
		t := (v - float64(black)) / float64(white-black)
		t = math.Max(0, math.Min(1, t))
		lut[i] = uint8(math.Pow(t, gamma)*255 + 0.5)
	}
	return func(v float32) uint8 {
		i := int(v / 1.5 * float32(len(lut)-1))
		if i < 0 {
			i = 0
		} else if i >= len(lut) {
			i = len(lut) - 1
		}
		return lut[i]
	}
}

func to8(v float32) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 1:
		return 255
	}
	return uint8(v*255 + 0.5)
}

// sharpen is an unsharp mask with a 3x3 box blur.
func sharpen(img *image.RGBA, amount float32) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	src := make([]uint8, len(img.Pix))
	copy(src, img.Pix)
	parallelRows(h, func(y0, y1 int) {
		for y := max(y0, 1); y < min(y1, h-1); y++ {
			for x := 1; x < w-1; x++ {
				for c := 0; c < 3; c++ {
					sum := float32(0)
					for dy := -1; dy <= 1; dy++ {
						row := (y + dy) * img.Stride
						for dx := -1; dx <= 1; dx++ {
							sum += float32(src[row+(x+dx)*4+c])
						}
					}
					v := float32(src[y*img.Stride+x*4+c])
					img.Pix[y*img.Stride+x*4+c] = to8((v + amount*(v-sum/9)) / 255)
				}
			}
		}
	})
}

func sharpenGray(img *image.Gray, amount float32) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	src := make([]uint8, len(img.Pix))
	copy(src, img.Pix)
	parallelRows(h, func(y0, y1 int) {
		for y := max(y0, 1); y < min(y1, h-1); y++ {
			for x := 1; x < w-1; x++ {
				sum := float32(0)
				for dy := -1; dy <= 1; dy++ {
					row := (y + dy) * img.Stride
					for dx := -1; dx <= 1; dx++ {
						sum += float32(src[row+x+dx])
					}
				}
				v := float32(src[y*img.Stride+x])
				img.Pix[y*img.Stride+x] = to8((v + amount*(v-sum/9)) / 255)
			}
		}
	})
}

// applyAdjust applies brightness and contrast in place.
func applyAdjust(img image.Image, adj Adjust) {
	var lut [256]uint8
	c := 1 + adj.Contrast
	if adj.Contrast > 0 {
		c = 1 + adj.Contrast*2
	}
	for i := range lut {
		v := float64(i) / 255
		v = (v-0.5)*c + 0.5 + adj.Brightness*0.35
		lut[i] = uint8(math.Max(0, math.Min(1, v))*255 + 0.5)
	}
	switch m := img.(type) {
	case *image.RGBA:
		for i := 0; i < len(m.Pix); i += 4 {
			m.Pix[i], m.Pix[i+1], m.Pix[i+2] = lut[m.Pix[i]], lut[m.Pix[i+1]], lut[m.Pix[i+2]]
		}
	case *image.Gray:
		for i, v := range m.Pix {
			m.Pix[i] = lut[v]
		}
	}
}
