package ui

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/xmhrnx/giode"

	"github.com/mehran/scanly/internal/scan"
)

// cropEditor shows a photo with the page outline over it. The four
// corners and the middle of each side can be dragged; while dragging,
// a magnifier shows the area under the finger at 3x, and a corner near
// the detected one snaps to it.
type cropEditor struct {
	img     image.Image
	quad    scan.Quad // in img pixels
	auto    scan.Quad // the detected outline, for snapping
	hasAuto bool

	active   int // -1 none, 0-3 corners, 4-7 the side after corner i-4
	grab     f32.Point
	last     f32.Point // finger position in img pixels
	finger   f32.Point // finger position on screen
	snapped  bool
	onChange func(scan.Quad)

	// The image's placement on screen in the last frame.
	scale  float32
	origin f32.Point
}

func newCropEditor() *cropEditor { return &cropEditor{active: -1} }

// set shows a new photo and outline.
func (c *cropEditor) set(img image.Image, q, auto scan.Quad, hasAuto bool) {
	c.img, c.quad, c.auto, c.hasAuto, c.active = img, q, auto, hasAuto, -1
}

var (
	colAccent  = color.NRGBA{R: 0x7B, G: 0x7B, B: 0xFF, A: 0xFF}
	colInvalid = color.NRGBA{R: 0xF8, G: 0x71, B: 0x71, A: 0xFF}
	colWhite   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colScrim   = color.NRGBA{R: 0x07, G: 0x08, B: 0x0D, A: 0xA8}
	colGrid    = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x50}
)

func (c *cropEditor) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	if c.img == nil || size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	fit := fitImage(size, c.img.Bounds(), float32(gtx.Dp(unit.Dp(22))))
	c.scale, c.origin = fit.scale, fit.origin

	c.handleInput(gtx, size)

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, c)

	drawImage(gtx.Ops, c.img, c.origin, c.scale)
	c.drawScrim(gtx.Ops, size)
	c.drawOutline(gtx, c.toScreen, 1)
	c.drawHandles(gtx)
	if c.active >= 0 {
		c.drawMagnifier(gtx, size)
	}
	return layout.Dimensions{Size: size}
}

func (c *cropEditor) toScreen(p scan.Point) f32.Point {
	return f32.Pt(c.origin.X+float32(p.X)*c.scale, c.origin.Y+float32(p.Y)*c.scale)
}

func (c *cropEditor) toImage(p f32.Point) f32.Point {
	return f32.Pt((p.X-c.origin.X)/c.scale, (p.Y-c.origin.Y)/c.scale)
}

// handles returns the screen positions of the corners, then the side
// midpoints.
func (c *cropEditor) handles() [8]f32.Point {
	var h [8]f32.Point
	for i := 0; i < 4; i++ {
		h[i] = c.toScreen(c.quad[i])
		h[4+i] = c.toScreen(c.quad[i].Add(c.quad[(i+1)%4]).Mul(0.5))
	}
	return h
}

func (c *cropEditor) handleInput(gtx layout.Context, size image.Point) {
	b := c.img.Bounds()
	maxX, maxY := float64(b.Dx()-1), float64(b.Dy()-1)
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
			c.active = -1
			best := float32(gtx.Dp(unit.Dp(44)))
			for i, h := range c.handles() {
				// Corners win over sides at equal distance.
				d := dist32(h, e.Position)
				if i >= 4 {
					d += float32(gtx.Dp(unit.Dp(6)))
				}
				if d < best {
					best, c.active = d, i
				}
			}
			if c.active < 0 {
				continue
			}
			gtx.Execute(pointer.GrabCmd{Tag: c, ID: e.PointerID})
			pos := c.toImage(e.Position)
			c.last, c.finger, c.snapped = pos, e.Position, false
			if c.active < 4 {
				q := c.quad[c.active]
				c.grab = f32.Pt(float32(q.X)-pos.X, float32(q.Y)-pos.Y)
			}
		case pointer.Drag:
			if c.active < 0 {
				continue
			}
			pos := c.toImage(e.Position)
			c.finger = e.Position
			if c.active < 4 {
				p := scan.Point{X: float64(pos.X + c.grab.X), Y: float64(pos.Y + c.grab.Y)}
				p.X, p.Y = clampF(p.X, 0, maxX), clampF(p.Y, 0, maxY)
				c.quad[c.active] = c.snap(gtx, c.active, p)
			} else {
				i := c.active - 4
				a, bq := c.quad[i], c.quad[(i+1)%4]
				d := bq.Sub(a)
				n := scan.Point{X: -d.Y, Y: d.X}
				if l := n.Len(); l > 0 {
					n = n.Mul(1 / l)
				}
				delta := scan.Point{X: float64(pos.X - c.last.X), Y: float64(pos.Y - c.last.Y)}
				move := n.Mul(delta.Dot(n))
				na, nb := a.Add(move), bq.Add(move)
				if inside(na, maxX, maxY) && inside(nb, maxX, maxY) {
					c.quad[i], c.quad[(i+1)%4] = na, nb
				}
			}
			c.last = pos
		case pointer.Release, pointer.Cancel:
			if c.active >= 0 && c.onChange != nil {
				c.onChange(c.quad)
			}
			c.active = -1
		}
	}
}

// snap pulls corner i onto the detected corner when it comes close.
func (c *cropEditor) snap(gtx layout.Context, i int, p scan.Point) scan.Point {
	if !c.hasAuto {
		return p
	}
	r := float64(gtx.Dp(unit.Dp(12))) / float64(c.scale)
	if p.Dist(c.auto[i]) < r {
		if !c.snapped {
			giode.Haptic(giode.HapticLight)
		}
		c.snapped = true
		return c.auto[i]
	}
	c.snapped = false
	return p
}

func inside(p scan.Point, maxX, maxY float64) bool {
	return p.X >= 0 && p.Y >= 0 && p.X <= maxX && p.Y <= maxY
}

func clampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func dist32(a, b f32.Point) float32 {
	return float32(math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)))
}

// drawScrim darkens everything outside the outline.
func (c *cropEditor) drawScrim(ops *op.Ops, size image.Point) {
	var p clip.Path
	p.Begin(ops)
	w, h := float32(size.X), float32(size.Y)
	p.MoveTo(f32.Pt(0, 0))
	p.LineTo(f32.Pt(w, 0))
	p.LineTo(f32.Pt(w, h))
	p.LineTo(f32.Pt(0, h))
	p.Close()
	// The outline in the opposite direction cuts a hole.
	q := c.quad
	p.MoveTo(c.toScreen(q[0]))
	p.LineTo(c.toScreen(q[3]))
	p.LineTo(c.toScreen(q[2]))
	p.LineTo(c.toScreen(q[1]))
	p.Close()
	paint.FillShape(ops, colScrim, clip.Outline{Path: p.End()}.Op())
}

// drawOutline strokes the outline and a rule-of-thirds grid, mapped to
// the screen by m.
func (c *cropEditor) drawOutline(gtx layout.Context, m func(scan.Point) f32.Point, width float32) {
	q := c.quad
	col := colAccent
	if !q.Convex() {
		col = colInvalid
	}
	lerp := func(a, b scan.Point, t float64) scan.Point { return a.Add(b.Sub(a).Mul(t)) }
	if c.active >= 0 {
		for _, t := range []float64{1.0 / 3, 2.0 / 3} {
			strokeLine(gtx.Ops, m(lerp(q[0], q[1], t)), m(lerp(q[3], q[2], t)), float32(gtx.Dp(1))*width, colGrid)
			strokeLine(gtx.Ops, m(lerp(q[0], q[3], t)), m(lerp(q[1], q[2], t)), float32(gtx.Dp(1))*width, colGrid)
		}
	}
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(m(q[0]))
	for i := 1; i < 4; i++ {
		p.LineTo(m(q[i]))
	}
	p.Close()
	paint.FillShape(gtx.Ops, col, clip.Stroke{Path: p.End(), Width: float32(gtx.Dp(2)) * width}.Op())
}

func strokeLine(ops *op.Ops, a, b f32.Point, width float32, col color.NRGBA) {
	var p clip.Path
	p.Begin(ops)
	p.MoveTo(a)
	p.LineTo(b)
	paint.FillShape(ops, col, clip.Stroke{Path: p.End(), Width: width}.Op())
}

func (c *cropEditor) drawHandles(gtx layout.Context) {
	col := colAccent
	if !c.quad.Convex() {
		col = colInvalid
	}
	for i, h := range c.handles() {
		r := float32(gtx.Dp(11))
		if i >= 4 {
			r = float32(gtx.Dp(7))
		}
		if i == c.active {
			r *= 1.25
		}
		circle(gtx.Ops, h, r+float32(gtx.Dp(3)), col)
		circle(gtx.Ops, h, r, colWhite)
	}
}

func circle(ops *op.Ops, c f32.Point, r float32, col color.NRGBA) {
	rect := image.Rect(int(c.X-r), int(c.Y-r), int(c.X+r), int(c.Y+r))
	paint.FillShape(ops, col, clip.Ellipse(rect).Op(ops))
}

// drawMagnifier shows the area under the finger enlarged, in the top
// corner away from the finger, with a crosshair on the handle.
func (c *cropEditor) drawMagnifier(gtx layout.Context, size image.Point) {
	r := float32(gtx.Dp(64))
	margin := float32(gtx.Dp(14))
	center := f32.Pt(margin+r, margin+r)
	if c.finger.X < float32(size.X)/2 && c.finger.Y < float32(size.Y)/2+r {
		center.X = float32(size.X) - margin - r
	}
	var target scan.Point
	if c.active < 4 {
		target = c.quad[c.active]
	} else {
		i := c.active - 4
		target = c.quad[i].Add(c.quad[(i+1)%4]).Mul(0.5)
	}
	zoom := c.scale * 3
	origin := f32.Pt(center.X-float32(target.X)*zoom, center.Y-float32(target.Y)*zoom)

	circle(gtx.Ops, center, r+float32(gtx.Dp(3)), colWhite)
	rect := image.Rect(int(center.X-r), int(center.Y-r), int(center.X+r), int(center.Y+r))
	stack := clip.Ellipse(rect).Push(gtx.Ops)
	paint.Fill(gtx.Ops, color.NRGBA{A: 0xFF})
	drawImage(gtx.Ops, c.img, origin, zoom)
	c.drawOutline(gtx, func(p scan.Point) f32.Point {
		return f32.Pt(origin.X+float32(p.X)*zoom, origin.Y+float32(p.Y)*zoom)
	}, 0.8)
	l := float32(gtx.Dp(10))
	strokeLine(gtx.Ops, f32.Pt(center.X-l, center.Y), f32.Pt(center.X+l, center.Y), float32(gtx.Dp(1)), colWhite)
	strokeLine(gtx.Ops, f32.Pt(center.X, center.Y-l), f32.Pt(center.X, center.Y+l), float32(gtx.Dp(1)), colWhite)
	stack.Pop()
}
