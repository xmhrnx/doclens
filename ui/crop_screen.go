package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/zoom"

	"github.com/xmhrnx/doclens/internal/scan"
)

// cropScreen lets the user check and adjust each page's outline.
type cropScreen struct {
	f      *scanFlow
	cur    int
	editor *cropEditor
	zoom   *zoom.Zoom
}

func newCropScreen(f *scanFlow) *cropScreen {
	s := &cropScreen{f: f, editor: newCropEditor()}
	// Pinch to place corners precisely; the magnifier helps too.
	s.zoom = giode.Zoomable(func() giode.Element { return s.editor }).Limits(1, 4).DoubleTapScale(0)
	s.editor.onChange = func(q scan.Quad) { s.page().quad = s.fromView(q) }
	s.show(0)
	return s
}

func (s *cropScreen) page() *pendingPage { return s.f.pages[s.cur] }

func (s *cropScreen) show(i int) {
	s.cur = i
	s.zoom.Reset()
	p := s.page()
	sc := p.src.scale
	s.editor.set(p.src.view, p.quad.Scale(sc, sc), p.auto.Scale(sc, sc), p.detected)
}

func (s *cropScreen) fromView(q scan.Quad) scan.Quad {
	sc := 1 / s.page().src.scale
	return q.Scale(sc, sc)
}

func (s *cropScreen) view() giode.Element {
	f := s.f
	title := "Adjust edges"
	if n := len(f.pages); n > 1 {
		title = fmt.Sprintf("Adjust edges · %d of %d", s.cur+1, n)
	}
	next := "Next"
	if f.editing {
		next = "Apply"
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: cStudio},
		topBar(iconTile("cropClose", "x", true, f.close), title, true,
			actionButton("cropNext", buttonStudio, "", next, false, s.next)),
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, s.zoom),
		s.hint(),
		s.pageStrip(),
		s.tools(),
	)
}

func (s *cropScreen) hint() giode.Element {
	p := s.page()
	msg := "Drag the corners or sides. Edges snap to the detected page."
	switch {
	case !p.detected && p.quad == scan.FullQuad(p.src.w, p.src.h):
		msg = "Drag the corners or sides to crop."
	case !p.detected:
		msg = "No page edges found. Drag the corners onto the page."
	}
	if !s.editor.quad.Convex() {
		msg = "The outline crosses itself. Move a corner back."
	}
	return giode.Box(giode.Styles{Width: -1, Justify: giode.JustifyCenter, Padding: giode.SymmetricInset(20, 6)},
		giode.Text(msg, giode.Styles{FontSize: 13, Color: cStudioMuted, TextAlign: giode.TextAlignCenter}))
}

// pageStrip shows the other captured pages, to jump between them.
func (s *cropScreen) pageStrip() giode.Element {
	if len(s.f.pages) < 2 {
		return giode.Box(giode.Styles{})
	}
	var kids []giode.Element
	for i, p := range s.f.pages {
		i, p := i, p
		kids = append(kids, tappable("cropStrip", p, func() { s.show(i) }, func(st giode.PressState) giode.Element {
			border := cStudioLine
			if i == s.cur {
				border = cAccent
			}
			return giode.Box(giode.Styles{Width: 46, Height: 60, BorderRadius: 8, BorderWidth: 2, BorderColor: border, Overflow: giode.OverflowHidden},
				giode.Image(p.src.view, giode.Styles{Width: 46, Height: 60, Fit: giode.FitCover}))
		}))
	}
	return giode.Row(giode.Styles{Width: -1, Justify: giode.JustifyCenter, Gap: 8, Padding: giode.SymmetricInset(16, 6)}, kids...)
}

func (s *cropScreen) tools() giode.Element {
	p := s.page()
	kids := []giode.Element{
		toolButton("auto", "wand-sparkles", "Auto", false, func() {
			p.quad = p.auto
			s.show(s.cur)
		}),
		toolButton("full", "maximize", "Full page", false, func() {
			p.quad = scan.FullQuad(p.src.w, p.src.h)
			s.show(s.cur)
		}),
	}
	if !s.f.editing && s.f.mode != scanBook {
		kids = append(kids, toolButton("retake", "camera", "Retake", false, s.retake))
	}
	if len(s.f.pages) > 1 {
		kids = append(kids, toolButton("drop", "trash-2", "Remove", false, s.remove))
	}
	return giode.Row(giode.Styles{
		Width: -1, Justify: giode.JustifySpaceEvenly, Padding: giode.Inset{Top: 8, Bottom: 20, Left: 12, Right: 12},
		Background: cStudioPanel, BorderRadii: giode.TopCorners(24),
	}, kids...)
}

// retake drops this capture and opens the camera again.
func (s *cropScreen) retake() {
	f := s.f
	giode.AfterLayout(func() {
		f.pages = append(f.pages[:s.cur], f.pages[s.cur+1:]...)
		f.a.pop()
		if f.cam == nil {
			f.capture()
		}
		// Otherwise the live camera is underneath, ready again.
	})
}

func (s *cropScreen) remove() {
	giode.AfterLayout(func() {
		f := s.f
		f.pages = append(f.pages[:s.cur], f.pages[s.cur+1:]...)
		s.show(min(s.cur, len(f.pages)-1))
	})
}

func (s *cropScreen) next() {
	for i, p := range s.f.pages {
		if !p.quad.Convex() {
			s.show(i)
			s.f.a.fail(fmt.Errorf("page %d: the outline crosses itself", i+1))
			return
		}
	}
	if s.f.editing {
		s.f.save()
		return
	}
	s.f.a.push(newEnhanceScreen(s.f))
}
