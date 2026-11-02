package scan

import (
	"image"
	"math"
)

// homography maps output pixels to source pixels.
type homography [9]float64

func (h homography) apply(x, y float64) (float64, float64) {
	w := h[6]*x + h[7]*y + h[8]
	return (h[0]*x + h[1]*y + h[2]) / w, (h[3]*x + h[4]*y + h[5]) / w
}

// homographyFromRect maps the rectangle (0,0)-(w,h) onto q.
func homographyFromRect(w, h float64, q Quad) (homography, bool) {
	src := [4]Point{{0, 0}, {w, 0}, {w, h}, {0, h}}
	var a [8][9]float64
	for i := 0; i < 4; i++ {
		x, y := src[i].X, src[i].Y
		u, v := q[i].X, q[i].Y
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -u * x, -u * y, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -v * x, -v * y, v}
	}
	// Gaussian elimination with partial pivoting.
	for c := 0; c < 8; c++ {
		p := c
		for r := c + 1; r < 8; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		if math.Abs(a[p][c]) < 1e-12 {
			return homography{}, false
		}
		a[c], a[p] = a[p], a[c]
		for r := 0; r < 8; r++ {
			if r == c {
				continue
			}
			f := a[r][c] / a[c][c]
			for k := c; k < 9; k++ {
				a[r][k] -= f * a[c][k]
			}
		}
	}
	var hm homography
	for i := 0; i < 8; i++ {
		hm[i] = a[i][8] / a[i][i]
	}
	hm[8] = 1
	return hm, true
}

// paperRatios are common height/width ratios a page snaps to when its
// measured ratio is very close.
var paperRatios = []float64{
	math.Sqrt2,   // ISO A-series
	11.0 / 8.5,   // US Letter
	14.0 / 8.5,   // US Legal
	85.6 / 53.98, // ID-1 card (landscape is handled by inversion)
}

// OutputSize estimates the true width and height of the flat page
// outlined by q in a w x h photo. It recovers the page's aspect ratio
// from the perspective (Zhang & He's whiteboard method, with the
// camera's principal point at the photo center), falling back to the
// mean side lengths when the view is nearly head-on.
func OutputSize(q Quad, w, h int) (int, int) {
	top, bottom := q[0].Dist(q[1]), q[3].Dist(q[2])
	left, right := q[0].Dist(q[3]), q[1].Dist(q[2])
	mw, mh := (top+bottom)/2, (left+right)/2
	if mw < 1 || mh < 1 {
		return 1, 1
	}
	ratio := mh / mw // height / width
	if r, ok := perspectiveRatio(q, float64(w)/2, float64(h)/2); ok && r/ratio < 1.6 && ratio/r < 1.6 {
		ratio = r
	}
	for _, p := range paperRatios {
		if math.Abs(ratio/p-1) < 0.025 {
			ratio = p
		} else if math.Abs(ratio*p-1) < 0.025 {
			ratio = 1 / p
		}
	}
	// Keep the resolution the photo actually has.
	outW := math.Max(top, bottom)
	outH := outW * ratio
	if lh := math.Max(left, right); outH < lh {
		outH = lh
		outW = outH / ratio
	}
	const maxPixels = 16e6
	if px := outW * outH; px > maxPixels {
		s := math.Sqrt(maxPixels / px)
		outW, outH = outW*s, outH*s
	}
	return max(1, int(math.Round(outW))), max(1, int(math.Round(outH)))
}

func perspectiveRatio(q Quad, u0, v0 float64) (float64, bool) {
	type v3 [3]float64
	cross := func(a, b v3) v3 {
		return v3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
	}
	dot := func(a, b v3) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
	m := func(p Point) v3 { return v3{p.X - u0, p.Y - v0, 1} }
	m1, m2, m3, m4 := m(q[0]), m(q[1]), m(q[3]), m(q[2]) // TL, TR, BL, BR
	k2 := dot(cross(m1, m4), m3) / dot(cross(m2, m4), m3)
	k3 := dot(cross(m1, m4), m2) / dot(cross(m3, m4), m2)
	n2 := v3{k2*m2[0] - m1[0], k2*m2[1] - m1[1], k2*m2[2] - m1[2]}
	n3 := v3{k3*m3[0] - m1[0], k3*m3[1] - m1[1], k3*m3[2] - m1[2]}
	// The focal length follows from the perspective, except when a pair
	// of sides is parallel in the photo (the page is level with the
	// camera along one axis); a phone's main camera then gives a good
	// guess: about 0.6 times the photo diagonal (a 26mm equivalent
	// lens).
	guess := 0.6 * 2 * math.Hypot(u0, v0)
	f2 := guess * guess
	if math.Abs(n2[2]*n3[2]) > 1e-9 {
		if est := -(n2[0]*n3[0] + n2[1]*n3[1]) / (n2[2] * n3[2]); est > 0 && !math.IsInf(est, 0) {
			if f := math.Sqrt(est); f > 0.25*guess && f < 4*guess {
				f2 = est
			}
		}
	}
	num := (n2[0]*n2[0]+n2[1]*n2[1])/f2 + n2[2]*n2[2]
	den := (n3[0]*n3[0]+n3[1]*n3[1])/f2 + n3[2]*n3[2]
	if num <= 0 || den <= 0 {
		return 0, false
	}
	wr := math.Sqrt(num / den) // width / height
	return 1 / wr, true
}

// Warp cuts the region q out of img and flattens it into an outW x
// outH image, sampling bilinearly.
func Warp(img image.Image, q Quad, outW, outH int) *image.RGBA {
	src := toRGBA(img)
	out := image.NewRGBA(image.Rect(0, 0, outW, outH))
	hm, ok := homographyFromRect(float64(outW), float64(outH), q)
	if !ok {
		return out
	}
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	parallelRows(outH, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			o := y * out.Stride
			for x := 0; x < outW; x++ {
				sx, sy := hm.apply(float64(x)+0.5, float64(y)+0.5)
				sx -= 0.5
				sy -= 0.5
				sampleBilinear(src, sw, sh, sx, sy, out.Pix[o+x*4:o+x*4+4])
			}
		}
	})
	return out
}

func sampleBilinear(src *image.RGBA, w, h int, x, y float64, dst []uint8) {
	if x < 0 {
		x = 0
	} else if x > float64(w-1) {
		x = float64(w - 1)
	}
	if y < 0 {
		y = 0
	} else if y > float64(h-1) {
		y = float64(h - 1)
	}
	x0, y0 := int(x), int(y)
	x1, y1 := min(x0+1, w-1), min(y0+1, h-1)
	fx, fy := x-float64(x0), y-float64(y0)
	p00 := src.Pix[y0*src.Stride+x0*4:]
	p10 := src.Pix[y0*src.Stride+x1*4:]
	p01 := src.Pix[y1*src.Stride+x0*4:]
	p11 := src.Pix[y1*src.Stride+x1*4:]
	for c := 0; c < 3; c++ {
		a := float64(p00[c])*(1-fx) + float64(p10[c])*fx
		b := float64(p01[c])*(1-fx) + float64(p11[c])*fx
		dst[c] = uint8(a*(1-fy) + b*fy + 0.5)
	}
	dst[3] = 255
}
