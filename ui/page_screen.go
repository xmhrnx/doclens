package ui

import (
	"fmt"
	"slices"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/zoom"
	"github.com/xmhrnx/giode/ocr"

	"github.com/xmhrnx/doclens/internal/library"
)

// pageScreen shows one finished page large, with the page tools.
type pageScreen struct {
	a      *App
	doc    *library.Document
	cur    int
	export *exportSheet
	text   *textSheet
}

func newPageScreen(a *App, d *library.Document, i int) *pageScreen {
	return &pageScreen{a: a, doc: d, cur: i, export: newExportSheet(a), text: newTextSheet(a)}
}

func (s *pageScreen) page() *library.Page { return s.doc.Pages[s.cur] }

func (s *pageScreen) view() giode.Element {
	d := s.doc
	if len(d.Pages) == 0 {
		giode.AfterLayout(s.a.pop)
		return giode.Box(giode.Styles{})
	}
	s.cur = min(s.cur, len(d.Pages)-1)
	n := len(d.Pages)
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: cStudio},
		topBar(iconTile("pageBack", "chevron-left", true, s.a.pop), fmt.Sprintf("Page %d of %d", s.cur+1, n), true,
			iconTile("pageDelete", "trash-2", true, s.delete),
			iconTile("pageShare", "share-2", true, func() { s.export.openPage(d, s.cur) })),
		s.viewer(),
		s.tools(),
		s.export.view(),
		s.text.view(),
	)
}

func (s *pageScreen) viewer() giode.Element {
	p := s.page()
	img := s.a.pages.get(s.doc.ID, p)
	if img == nil {
		// The thumbnail stands in while the full page loads.
		img = s.a.thumbs.get(s.doc.ID, p)
	}
	var content giode.Element = giode.Spinner(giode.Styles{Width: 32, Height: 32, Color: cAccent})
	if img != nil {
		// Pinch, double-tap or Ctrl+scroll to look closely; each page
		// keeps its own zoom.
		z := giode.Keyed("pageZoom", cacheKey(s.doc.ID, p), func() *zoom.Zoom {
			return giode.Zoomable(func() giode.Element { return s.pageImage(p) }).Limits(1, 6)
		})
		content = fillSpace(z)
	}
	n := len(s.doc.Pages)
	arrow := func(key, name string, dir int, side giode.Styles) giode.Element {
		if n < 2 {
			return giode.Box(giode.Styles{Display: giode.DisplayNone})
		}
		st := giode.Styles{Position: giode.PositionAbsolute, Top: giode.Flush, Bottom: giode.Flush, ZIndex: 2, Justify: giode.JustifyCenter, Align: giode.AlignCenter}
		return giode.Box(st.With(side), iconTile(key, name, true, func() { s.cur = (s.cur + dir + n) % n }))
	}
	// The arrows float over the page, so a zoomed page uses the whole
	// width.
	return giode.Box(giode.Styles{FlexGrow: 1, Width: -1, Padding: giode.SymmetricInset(0, 8)},
		giode.Box(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter}, content),
		arrow("pagePrev", "chevron-left", -1, giode.Styles{Left: 8}),
		arrow("pageNext", "chevron-right", 1, giode.Styles{Right: 8}),
	)
}

// pageImage is the page as large as it's loaded so far.
func (s *pageScreen) pageImage(p *library.Page) giode.Element {
	img := s.a.pages.get(s.doc.ID, p)
	if img == nil {
		img = s.a.thumbs.get(s.doc.ID, p)
	}
	if img == nil {
		return giode.Box(giode.Styles{})
	}
	return giode.Image(img, giode.Styles{Width: -1, Height: -1, Fit: giode.FitContain})
}

func (s *pageScreen) tools() giode.Element {
	a, d := s.a, s.doc
	p := s.page()
	return giode.Row(giode.Styles{
		Width: -1, Justify: giode.JustifySpaceEvenly, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Top: 8, Bottom: 20, Left: 6, Right: 6},
		Background: cStudioPanel, BorderRadii: giode.TopCorners(24),
	},
		toolButton("pCrop", "crop", "Crop", false, func() { startEdit(a, d, []*library.Page{p}, true) }),
		toolButton("pFilter", "wand-sparkles", "Filter", false, func() { startEdit(a, d, []*library.Page{p}, false) }),
		toolButton("pRotate", "rotate-cw", "Rotate", false, func() {
			p.Rotate(1)
			d.Modified = time.Now()
			a.save()
		}),
		toolButton("pMarkup", "pen-line", "Markup", false, func() { openMarkup(a, d, p) }),
		toolButton("pSign", "signature", "Sign", false, func() { openSign(a, d, p) }),
		toolButton("pRetake", "camera", "Retake", false, func() { startRetake(a, d, p) }),
		s.textTool(),
	)
}

func (s *pageScreen) delete() {
	d, p, at := s.doc, s.page(), s.cur
	giode.AfterLayout(func() {
		d.Pages = slices.Delete(d.Pages, at, at+1)
		s.a.save()
		s.a.toasts.ShowAction(giode.ToastInfo, fmt.Sprintf("Deleted page %d", at+1), "Undo", func() {
			d.Pages = slices.Insert(d.Pages, min(at, len(d.Pages)), p)
			s.a.save()
		})
	})
}

// textTool reads the page's text, where the device can.
func (s *pageScreen) textTool() giode.Element {
	if !ocr.Supported() {
		return giode.Box(giode.Styles{Display: giode.DisplayNone})
	}
	d, p := s.doc, s.page()
	return toolButton("pText", "text-select", "Text", false, func() { s.text.open(d, p) })
}
