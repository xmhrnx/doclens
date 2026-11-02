package ui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/xmhrnx/giode/styles/properties"
)

// imageFit is where an image sits when fitted, centered, into an area.
type imageFit struct {
	scale  float32
	origin f32.Point
}

func fitImage(area image.Point, img image.Rectangle, pad float32) imageFit {
	iw, ih := float32(img.Dx()), float32(img.Dy())
	s := min((float32(area.X)-2*pad)/iw, (float32(area.Y)-2*pad)/ih)
	return imageFit{scale: s, origin: f32.Pt((float32(area.X)-iw*s)/2, (float32(area.Y)-ih*s)/2)}
}

// toScreen maps image pixels to the screen.
func (f imageFit) toScreen(x, y float64) f32.Point {
	return f32.Pt(f.origin.X+float32(x)*f.scale, f.origin.Y+float32(y)*f.scale)
}

// toImage maps a screen position to image pixels.
func (f imageFit) toImage(p f32.Point) f32.Point {
	return f32.Pt((p.X-f.origin.X)/f.scale, (p.Y-f.origin.Y)/f.scale)
}

// drawImage paints img scaled by scale with its top-left at origin.
func drawImage(ops *op.Ops, img image.Image, origin f32.Point, scale float32) {
	defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(scale, scale)).Offset(origin)).Push(ops).Pop()
	defer clip.Rect{Max: img.Bounds().Size()}.Push(ops).Pop()
	properties.ImageOp(img).Add(ops)
	paint.PaintOp{}.Add(ops)
}
