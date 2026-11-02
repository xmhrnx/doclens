package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/mehran/scanly/internal/library"
)

// placedStamp is a signature on the page being signed.
type placedStamp struct {
	ref library.StampRef
	img image.Image
}

// stampPlacer shows a page with signatures on it. Dragging a signature
// moves it; dragging its corner handle resizes it. Tapping one selects
// it.
type stampPlacer struct {
	page     image.Image
	stamps   []*placedStamp
	selected int

	fit      imageFit
	dragging int // -1 none
	resizing bool
	last     f32.Point
}

func newStampPlacer() *stampPlacer { return &stampPlacer{selected: -1, dragging: -1} }

// rect returns a stamp's box on screen.
func (s *stampPlacer) rect(st *placedStamp) (f32.Point, f32.Point) {
	b := s.page.Bounds()
	pw, ph := float64(b.Dx()), float64(b.Dy())
	w := st.ref.W * pw
	h := w / max(st.ref.Aspect, 0.01)
	return s.fit.toScreen(st.ref.X*pw, st.ref.Y*ph), s.fit.toScreen(st.ref.X*pw+w, st.ref.Y*ph+h)
}

func (s *stampPlacer) Layout(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	if s.page == nil || size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	s.fit = fitImage(size, s.page.Bounds(), float32(gtx.Dp(unit.Dp(16))))
	s.handleInput(gtx)

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, s)
	drawImage(gtx.Ops, s.page, s.fit.origin, s.fit.scale)
	for i, st := range s.stamps {
		tl, br := s.rect(st)
		sb := st.img.Bounds()
		drawImage(gtx.Ops, st.img, tl, (br.X-tl.X)/float32(sb.Dx()))
		if i == s.selected {
			r := image.Rect(int(tl.X)-4, int(tl.Y)-4, int(br.X)+4, int(br.Y)+4)
			paint.FillShape(gtx.Ops, colAccent, clip.Stroke{Path: clip.RRect{Rect: r, SE: 6, SW: 6, NE: 6, NW: 6}.Path(gtx.Ops), Width: float32(gtx.Dp(2))}.Op())
			h := f32.Pt(br.X+4, br.Y+4)
			circle(gtx.Ops, h, float32(gtx.Dp(13)), colAccent)
			circle(gtx.Ops, h, float32(gtx.Dp(9)), color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
		}
	}
	return layout.Dimensions{Size: size}
}

func (s *stampPlacer) handleInput(gtx layout.Context) {
	b := s.page.Bounds()
	pw, ph := float64(b.Dx()), float64(b.Dy())
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: s, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			s.dragging, s.resizing = -1, false
			if s.selected >= 0 && s.selected < len(s.stamps) {
				_, br := s.rect(s.stamps[s.selected])
				if dist32(e.Position, f32.Pt(br.X+4, br.Y+4)) < float32(gtx.Dp(28)) {
					s.dragging, s.resizing = s.selected, true
				}
			}
			if s.dragging < 0 {
				for i := len(s.stamps) - 1; i >= 0; i-- {
					tl, br := s.rect(s.stamps[i])
					if e.Position.X >= tl.X && e.Position.X <= br.X && e.Position.Y >= tl.Y && e.Position.Y <= br.Y {
						s.dragging, s.selected = i, i
						break
					}
				}
			}
			if s.dragging >= 0 {
				gtx.Execute(pointer.GrabCmd{Tag: s, ID: e.PointerID})
			}
			s.last = e.Position
		case pointer.Drag:
			if s.dragging < 0 {
				continue
			}
			st := s.stamps[s.dragging]
			dx := float64(e.Position.X-s.last.X) / float64(s.fit.scale)
			dy := float64(e.Position.Y-s.last.Y) / float64(s.fit.scale)
			if s.resizing {
				st.ref.W = clampF(st.ref.W+dx/pw, 0.08, 1-st.ref.X)
			} else {
				h := st.ref.W * pw / max(st.ref.Aspect, 0.01) / ph
				st.ref.X = clampF(st.ref.X+dx/pw, 0, 1-st.ref.W)
				st.ref.Y = clampF(st.ref.Y+dy/ph, 0, max(0, 1-h))
			}
			s.last = e.Position
		case pointer.Release, pointer.Cancel:
			s.dragging, s.resizing = -1, false
		}
	}
}
