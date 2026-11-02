package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/xmhrnx/doclens/internal/scan"
)

// inkCanvas draws freehand strokes over an image (or over a blank
// area when img is nil). Strokes are kept in fractions of the drawing
// area, so they map onto the page at any resolution.
type inkCanvas struct {
	img     image.Image
	strokes []scan.Stroke
	drawing *scan.Stroke
	// The pen for new strokes.
	color     color.RGBA
	width     float64 // fraction of the area's width
	highlight bool
	// blank is the area's background when there's no image.
	blank color.NRGBA
	// onStroke runs when a stroke is finished.
	onStroke func()

	fit  imageFit
	area image.Point // the drawing area in image pixels
}

func newInkCanvas() *inkCanvas {
	return &inkCanvas{color: color.RGBA{R: 0x1D, G: 0x4E, B: 0xD8, A: 0xFF}, width: 0.006}
}

func (c *inkCanvas) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	bounds := image.Rectangle{Max: size}
	pad := float32(gtx.Dp(unit.Dp(12)))
	if c.img != nil {
		bounds = c.img.Bounds()
	} else {
		pad = 0
	}
	c.fit = fitImage(size, bounds, pad)
	c.area = bounds.Size()
	c.handleInput(gtx)

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, c)
	if c.img != nil {
		drawImage(gtx.Ops, c.img, c.fit.origin, c.fit.scale)
	} else {
		paint.FillShape(gtx.Ops, c.blank, clip.Rect{Max: size}.Op())
	}
	for _, s := range c.strokes {
		c.drawStroke(gtx.Ops, s)
	}
	if c.drawing != nil {
		c.drawStroke(gtx.Ops, *c.drawing)
	}
	return layout.Dimensions{Size: size}
}

func (c *inkCanvas) toFraction(p f32.Point) (scan.Point, bool) {
	ip := c.fit.toImage(p)
	x, y := float64(ip.X)/float64(c.area.X), float64(ip.Y)/float64(c.area.Y)
	return scan.Point{X: x, Y: y}, x >= 0 && y >= 0 && x <= 1 && y <= 1
}

func (c *inkCanvas) handleInput(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: c, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			p, in := c.toFraction(e.Position)
			if !in {
				continue
			}
			gtx.Execute(pointer.GrabCmd{Tag: c, ID: e.PointerID})
			w := c.width
			if c.highlight {
				w *= 4
			}
			c.drawing = &scan.Stroke{Points: []scan.Point{p}, Width: w, Color: c.color, Highlight: c.highlight}
		case pointer.Drag:
			if c.drawing == nil {
				continue
			}
			p, _ := c.toFraction(e.Position)
			p.X, p.Y = clampF(p.X, 0, 1), clampF(p.Y, 0, 1)
			last := c.drawing.Points[len(c.drawing.Points)-1]
			// Skip points closer than about a pixel on screen.
			dx := (p.X - last.X) * float64(c.area.X) * float64(c.fit.scale)
			dy := (p.Y - last.Y) * float64(c.area.Y) * float64(c.fit.scale)
			if dx*dx+dy*dy >= 2 {
				c.drawing.Points = append(c.drawing.Points, p)
			}
		case pointer.Release, pointer.Cancel:
			if c.drawing != nil {
				c.strokes = append(c.strokes, *c.drawing)
				c.drawing = nil
				if c.onStroke != nil {
					c.onStroke()
				}
			}
		}
	}
}

func (c *inkCanvas) drawStroke(ops *op.Ops, s scan.Stroke) {
	if len(s.Points) == 0 {
		return
	}
	at := func(p scan.Point) f32.Point {
		return c.fit.toScreen(p.X*float64(c.area.X), p.Y*float64(c.area.Y))
	}
	width := float32(s.Width*float64(c.area.X)) * c.fit.scale
	col := color.NRGBA{R: s.Color.R, G: s.Color.G, B: s.Color.B, A: 0xFF}
	if s.Highlight {
		col.A = 0x66
	}
	if len(s.Points) == 1 {
		circle(ops, at(s.Points[0]), width/2, col)
		return
	}
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(at(s.Points[0]))
	for _, pt := range s.Points[1:] {
		p.LineTo(at(pt))
	}
	paint.FillShape(ops, col, clip.Stroke{Path: p.End(), Width: width}.Op())
	if !s.Highlight {
		// Round the ends.
		circle(ops, at(s.Points[0]), width/2, col)
		circle(ops, at(s.Points[len(s.Points)-1]), width/2, col)
	}
}

// undo removes the last stroke.
func (c *inkCanvas) undo() {
	if n := len(c.strokes); n > 0 {
		c.strokes = c.strokes[:n-1]
	}
}
