package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/slider"

	"github.com/mehran/scanly/internal/scan"
)

// filterNames label the filters.
var filterNames = map[scan.Filter]string{
	scan.FilterMagic:    "Magic Color",
	scan.FilterOriginal: "Original",
	scan.FilterNoShadow: "No Shadow",
	scan.FilterGray:     "Grayscale",
	scan.FilterBW:       "B & W",
}

// enhanceScreen picks each page's filter, rotation and brightness and
// contrast, with a live preview, then saves.
type enhanceScreen struct {
	f          *scanFlow
	cur        int
	r          *pagePreviewer
	adjusting  bool
	brightness *slider.Slider
	contrast   *slider.Slider
}

func newEnhanceScreen(f *scanFlow) *enhanceScreen {
	s := &enhanceScreen{f: f, r: newPagePreviewer(f.a)}
	slide := giode.Styles{Color: cAccent, Background: cStudioLine, BorderColor: "#FFFFFF"}
	s.brightness = giode.Input.Slider(slide).OnChange(func(v float32) { s.page().adjust.Brightness = float64(v)*2 - 1 })
	s.contrast = giode.Input.Slider(slide).OnChange(func(v float32) { s.page().adjust.Contrast = float64(v)*2 - 1 })
	s.show(0)
	return s
}

func (s *enhanceScreen) page() *pendingPage { return s.f.pages[s.cur] }

func (s *enhanceScreen) show(i int) {
	s.cur = i
	p := s.page()
	s.brightness.SetValue(float32(p.adjust.Brightness+1) / 2)
	s.contrast.SetValue(float32(p.adjust.Contrast+1) / 2)
}

func (s *enhanceScreen) view() giode.Element {
	n := len(s.f.pages)
	title := "Enhance"
	if n > 1 {
		title = fmt.Sprintf("Enhance · %d of %d", s.cur+1, n)
	}
	label := "Save"
	if n > 1 && !s.f.editing {
		label = fmt.Sprintf("Save %d", n)
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: cStudio},
		topBar(iconTile("enhBack", "chevron-left", true, s.back), title, true,
			actionButton("enhSave", buttonStudio, "check", label, false, s.save)),
		s.previewArea(),
		s.pager(),
		s.adjustPanel(),
		s.filterStrip(),
		s.tools(),
	)
}

func (s *enhanceScreen) back() {
	s.r.release()
	s.f.a.pop()
}

func (s *enhanceScreen) previewArea() giode.Element {
	img, current := s.r.preview(s.page())
	kids := []giode.Element{}
	if img != nil {
		kids = append(kids, giode.Image(img, giode.Styles{Width: -1, Height: -1, Fit: giode.FitContain}))
	}
	if !current {
		kids = append(kids, giode.Box(giode.Styles{
			Position: giode.PositionAbsolute, Left: giode.Flush, Right: giode.Flush, Top: giode.Flush, Bottom: giode.Flush,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter,
		}, giode.Box(giode.Styles{Padding: giode.UniformInset(14), Background: "#12141ECC", BorderRadius: radiusPill},
			giode.Spinner(giode.Styles{Width: 28, Height: 28, Color: cAccent}))))
	}
	return giode.Box(giode.Styles{FlexGrow: 1, Width: -1, Padding: giode.SymmetricInset(24, 12)},
		giode.Box(giode.Styles{Width: -1, Height: -1}, kids...))
}

// pager moves between pages of a multi-page scan.
func (s *enhanceScreen) pager() giode.Element {
	n := len(s.f.pages)
	if n < 2 {
		return giode.Box(giode.Styles{})
	}
	return giode.Row(giode.Styles{Width: -1, Justify: giode.JustifyCenter, Align: giode.AlignCenter, Gap: 16, Padding: giode.Inset{Bottom: 6}},
		iconTile("enhPrev", "chevron-left", true, func() { s.show((s.cur + n - 1) % n) }),
		giode.Text(fmt.Sprintf("Page %d / %d", s.cur+1, n), giode.Styles{FontSize: 14, FontWeight: giode.FontWeightSemiBold, Color: cStudioText}),
		iconTile("enhNext", "chevron-right", true, func() { s.show((s.cur + 1) % n) }),
	)
}

func (s *enhanceScreen) adjustPanel() giode.Element {
	if !s.adjusting {
		return giode.Box(giode.Styles{})
	}
	row := func(iconName, label string, sl *slider.Slider) giode.Element {
		return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
			icon(iconName, 18, cStudioMuted),
			giode.Box(giode.Styles{Width: 84}, giode.Text(label, giode.Styles{FontSize: 13, Color: cStudioText})),
			giode.Box(giode.Styles{FlexGrow: 1}, sl),
		)
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 12, Padding: giode.SymmetricInset(24, 10)},
		row("sun", "Brightness", s.brightness),
		row("contrast", "Contrast", s.contrast),
	)
}

// filterStrip shows the page in every filter.
func (s *enhanceScreen) filterStrip() giode.Element {
	p := s.page()
	looks := s.r.filterLooks(p)
	var tiles []giode.Element
	for _, f := range scan.Filters {
		f := f
		tiles = append(tiles, tappable("filterTile", f, func() { p.filter = f }, func(st giode.PressState) giode.Element {
			selected := p.filter == f
			border, fg := "transparent", cStudioMuted
			if selected {
				border, fg = cAccent, cStudioText
			}
			var thumb giode.Element = giode.Box(giode.Styles{Width: 58, Height: 74, Background: cStudioPanel})
			if img := looks[f]; img != nil {
				thumb = giode.Image(img, giode.Styles{Width: 58, Height: 74, Fit: giode.FitCover})
			}
			return giode.Stack(giode.Styles{Align: giode.AlignCenter, Gap: 6, Transform: pressScale(st)},
				giode.Box(giode.Styles{Width: 62, Height: 78, BorderRadius: 12, BorderWidth: 2, BorderColor: border,
					Overflow: giode.OverflowHidden, Align: giode.AlignCenter, Justify: giode.JustifyCenter}, thumb),
				giode.Text(filterNames[f], giode.Styles{FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: fg}),
			)
		}))
	}
	return giode.Row(giode.Styles{Width: -1, Justify: giode.JustifySpaceEvenly, Wrap: giode.WrapNoWrap, Padding: giode.SymmetricInset(8, 10)}, tiles...)
}

func (s *enhanceScreen) tools() giode.Element {
	p := s.page()
	kids := []giode.Element{
		toolButton("rotl", "rotate-ccw", "Left", false, func() { p.rotation = (p.rotation + 3) % 4 }),
		toolButton("rotr", "rotate-cw", "Right", false, func() { p.rotation = (p.rotation + 1) % 4 }),
		toolButton("adjust", "sliders-horizontal", "Adjust", s.adjusting, func() { s.adjusting = !s.adjusting }),
	}
	if len(s.f.pages) > 1 {
		kids = append(kids, toolButton("all", "layers", "Apply to all", false, s.applyToAll))
	}
	if s.f.editing {
		kids = append(kids, toolButton("reset", "refresh-cw", "Reset", false, func() {
			p.adjust = scan.Adjust{}
			s.show(s.cur)
		}))
	}
	return giode.Row(giode.Styles{
		Width: -1, Justify: giode.JustifySpaceEvenly, Padding: giode.Inset{Top: 8, Bottom: 20, Left: 12, Right: 12},
		Background: cStudioPanel, BorderRadii: giode.TopCorners(24),
	}, kids...)
}

func (s *enhanceScreen) applyToAll() {
	p := s.page()
	for _, o := range s.f.pages {
		o.filter, o.adjust = p.filter, p.adjust
	}
	s.f.a.notify(fmt.Sprintf("%s applied to all %d pages", filterNames[p.filter], len(s.f.pages)))
}

func (s *enhanceScreen) save() {
	s.r.release()
	s.f.save()
}
