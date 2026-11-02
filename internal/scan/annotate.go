package scan

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// Stroke is a freehand line drawn on a page. Points are in page
// fractions (0..1 of width and height), Width is a fraction of the
// page width.
type Stroke struct {
	Points    []Point    `json:"points"`
	Width     float64    `json:"width"`
	Color     color.RGBA `json:"color"`
	Highlight bool       `json:"highlight,omitempty"`
}

// Stamp places a picture (a signature) on a page. X, Y and W are page
// fractions: the top-left corner and the width; the height follows the
// picture's aspect ratio.
type Stamp struct {
	Image image.Image
	X, Y  float64
	W     float64
}

// Annotate draws strokes and stamps onto a page, returning a new
// image.
func Annotate(page image.Image, strokes []Stroke, stamps []Stamp) image.Image {
	if len(strokes) == 0 && len(stamps) == 0 {
		return page
	}
	if g, ok := page.(*image.Gray); ok && grayOnly(strokes, stamps) {
		out := image.NewGray(g.Rect)
		copy(out.Pix, g.Pix)
		rgba := toRGBA(out)
		annotateRGBA(rgba, strokes, stamps)
		gray := image.NewGray(g.Rect)
		draw.Draw(gray, gray.Rect, rgba, image.Point{}, draw.Src)
		return gray
	}
	out := cloneRGBA(toRGBA(page))
	annotateRGBA(out, strokes, stamps)
	return out
}

func grayOnly(strokes []Stroke, stamps []Stamp) bool {
	if len(stamps) > 0 {
		return false
	}
	for _, s := range strokes {
		if s.Color.R != s.Color.G || s.Color.G != s.Color.B {
			return false
		}
	}
	return true
}

func annotateRGBA(dst *image.RGBA, strokes []Stroke, stamps []Stamp) {
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	for _, st := range stamps {
		sb := st.Image.Bounds()
		if sb.Empty() {
			continue
		}
		dw := st.W * float64(w)
		dh := dw * float64(sb.Dy()) / float64(sb.Dx())
		r := image.Rect(int(st.X*float64(w)), int(st.Y*float64(h)), int(st.X*float64(w)+dw), int(st.Y*float64(h)+dh))
		xdraw.CatmullRom.Scale(dst, r, st.Image, sb, xdraw.Over, nil)
	}
	for _, s := range strokes {
		drawStroke(dst, s, w, h)
	}
}

func drawStroke(dst *image.RGBA, s Stroke, w, h int) {
	if len(s.Points) == 0 {
		return
	}
	width := math.Max(1, s.Width*float64(w))
	r := vector.NewRasterizer(w, h)
	pts := make([]Point, len(s.Points))
	for i, p := range s.Points {
		pts[i] = Point{p.X * float64(w), p.Y * float64(h)}
	}
	half := width / 2
	for i := 0; i < len(pts); i++ {
		if !s.Highlight {
			addCircle(r, pts[i], half)
		}
		if i == 0 {
			continue
		}
		a, b := pts[i-1], pts[i]
		d := b.Sub(a)
		l := d.Len()
		if l < 1e-6 {
			continue
		}
		n := Point{-d.Y / l * half, d.X / l * half}
		p0, p1, p2, p3 := a.Add(n), b.Add(n), b.Sub(n), a.Sub(n)
		// Every piece winds the same way as the round joins (positive
		// area), so where they overlap coverage adds up instead of
		// cancelling out.
		if polyArea([]Point{p0, p1, p2, p3}) < 0 {
			p0, p1, p2, p3 = p3, p2, p1, p0
		}
		r.MoveTo(float32(p0.X), float32(p0.Y))
		r.LineTo(float32(p1.X), float32(p1.Y))
		r.LineTo(float32(p2.X), float32(p2.Y))
		r.LineTo(float32(p3.X), float32(p3.Y))
		r.ClosePath()
	}
	if len(pts) == 1 && s.Highlight {
		addCircle(r, pts[0], half)
	}
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	r.Draw(mask, mask.Rect, image.Opaque, image.Point{})
	c := s.Color
	if s.Highlight {
		// A highlighter darkens like a translucent marker: multiply.
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				a := float32(mask.Pix[y*mask.Stride+x]) / 255 * 0.85
				if a == 0 {
					continue
				}
				p := dst.Pix[y*dst.Stride+x*4:]
				for k, cv := range [3]uint8{c.R, c.G, c.B} {
					m := float32(p[k]) * float32(cv) / 255
					p[k] = uint8(float32(p[k])*(1-a) + m*a + 0.5)
				}
			}
		}
		return
	}
	draw.DrawMask(dst, dst.Rect, image.NewUniform(color.RGBA{c.R, c.G, c.B, 255}), image.Point{}, mask, image.Point{}, draw.Over)
}

func addCircle(r *vector.Rasterizer, c Point, rad float64) {
	const n = 20
	for i := 0; i <= n; i++ {
		a := 2 * math.Pi * float64(i) / n
		x, y := float32(c.X+rad*math.Cos(a)), float32(c.Y+rad*math.Sin(a))
		if i == 0 {
			r.MoveTo(x, y)
		} else {
			r.LineTo(x, y)
		}
	}
	r.ClosePath()
}

// Watermark draws text diagonally across the page in a translucent
// color.
func Watermark(page image.Image, text string, fontData []byte, c color.RGBA, opacity float64) (image.Image, error) {
	if text == "" {
		return page, nil
	}
	f, err := opentype.Parse(fontData)
	if err != nil {
		return nil, err
	}
	dst := cloneRGBA(toRGBA(page))
	w, h := dst.Rect.Dx(), dst.Rect.Dy()
	diag := math.Hypot(float64(w), float64(h))
	// Size the text to span about 70% of the diagonal.
	size := 100.0
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil, err
	}
	adv := font.MeasureString(face, text)
	size *= 0.7 * diag / math.Max(1, float64(adv.Round()))
	size = math.Min(size, float64(h)/4)
	face.Close()
	face, err = opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	adv = font.MeasureString(face, text)
	m := face.Metrics()
	tw, th := adv.Ceil()+8, (m.Ascent+m.Descent).Ceil()+8
	layer := image.NewRGBA(image.Rect(0, 0, tw, th))
	alpha := uint8(math.Max(0, math.Min(1, opacity)) * 255)
	d := font.Drawer{
		Dst:  layer,
		Src:  image.NewUniform(color.NRGBA{c.R, c.G, c.B, alpha}),
		Face: face,
		Dot:  fixed.P(4, m.Ascent.Ceil()+4),
	}
	d.DrawString(text)
	angle := -math.Atan2(float64(h), float64(w))
	cos, sin := math.Cos(angle), math.Sin(angle)
	// Rotate the text layer about its center onto the page center.
	cx, cy := float64(tw)/2, float64(th)/2
	px, py := float64(w)/2, float64(h)/2
	aff := f64.Aff3{
		cos, -sin, px - cos*cx + sin*cy,
		sin, cos, py - sin*cx - cos*cy,
	}
	xdraw.BiLinear.Transform(dst, aff, layer, layer.Rect, xdraw.Over, nil)
	return dst, nil
}
