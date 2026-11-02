package ui

import (
	"image"
	"image/color"
	"slices"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/doclens/internal/library"
)

// inkColors are the pen colors offered.
var inkColors = []color.RGBA{
	{R: 0x11, G: 0x13, B: 0x1C, A: 0xFF},
	{R: 0x1D, G: 0x4E, B: 0xD8, A: 0xFF},
	{R: 0xDC, G: 0x26, B: 0x26, A: 0xFF},
	{R: 0x05, G: 0x96, B: 0x69, A: 0xFF},
	{R: 0xFA, G: 0xCC, B: 0x15, A: 0xFF},
}

// markupScreen draws with a pen or highlighter on a page.
type markupScreen struct {
	a      *App
	doc    *library.Document
	page   *library.Page
	canvas *inkCanvas
	size   int // 0 thin, 1 medium, 2 thick
}

// openMarkup renders the page without its drawings and opens the
// editor on it.
func openMarkup(a *App, d *library.Document, p *library.Page) {
	clone := p.Clone()
	var base image.Image
	a.work("Opening the page", func(func(float64)) error {
		var err error
		base, err = a.store.RenderBase(d.ID, clone, screenSide)
		return err
	}, func() {
		s := &markupScreen{a: a, doc: d, page: p, canvas: newInkCanvas(), size: 1}
		s.canvas.img = base
		s.canvas.strokes = slices.Clone(p.Strokes)
		a.push(s)
	})
}

func (s *markupScreen) view() giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: cStudio},
		topBar(iconTile("mkClose", "x", true, s.close), "Markup", true,
			actionButton("mkDone", buttonStudio, "check", "Done", false, s.done)),
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, s.canvas),
		s.palette(),
		s.tools(),
	)
}

func (s *markupScreen) palette() giode.Element {
	c := s.canvas
	var kids []giode.Element
	for i, col := range inkColors {
		col := col
		kids = append(kids, tappable("inkColor", i, func() { c.color = col }, func(st giode.PressState) giode.Element {
			ring := "transparent"
			if c.color == col {
				ring = "#FFFFFF"
			}
			return giode.Box(giode.Styles{Width: 34, Height: 34, BorderRadius: 17, BorderWidth: 2, BorderColor: ring,
				Align: giode.AlignCenter, Justify: giode.JustifyCenter, Transform: pressScale(st)},
				giode.Box(giode.Styles{Width: 24, Height: 24, BorderRadius: 12, Background: hexOf(col), BorderWidth: 1, BorderColor: "#FFFFFF40"}))
		}))
	}
	sizes := []float64{0.0035, 0.006, 0.011}
	for i, w := range sizes {
		i, w := i, w
		kids = append(kids, tappable("inkSize", i, func() { s.size = i; c.width = w }, func(st giode.PressState) giode.Element {
			bg := cStudioPanel
			if s.size == i {
				bg = "#2C3048"
			}
			dot := 6 + 5*i
			return giode.Box(giode.Styles{Width: 34, Height: 34, BorderRadius: 17, Background: bg, Align: giode.AlignCenter, Justify: giode.JustifyCenter},
				giode.Box(giode.Styles{Width: properties.Width(dot), Height: properties.Height(dot), BorderRadius: properties.BorderRadius(dot), Background: cStudioText}))
		}))
	}
	return giode.Row(giode.Styles{Width: -1, Justify: giode.JustifyCenter, Align: giode.AlignCenter, Gap: 10, Padding: giode.SymmetricInset(12, 10)}, kids...)
}

func hexOf(c color.RGBA) string {
	const digits = "0123456789ABCDEF"
	b := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i, v := range []uint8{c.R, c.G, c.B} {
		b[1+2*i], b[2+2*i] = digits[v>>4], digits[v&15]
	}
	return string(b)
}

func (s *markupScreen) tools() giode.Element {
	c := s.canvas
	return giode.Row(giode.Styles{
		Width: -1, Justify: giode.JustifySpaceEvenly, Padding: giode.Inset{Top: 8, Bottom: 20, Left: 12, Right: 12},
		Background: cStudioPanel, BorderRadii: giode.TopCorners(24),
	},
		toolButton("mkPen", "pen-line", "Pen", !c.highlight, func() { c.highlight = false }),
		toolButton("mkHi", "highlighter", "Highlighter", c.highlight, func() {
			c.highlight = true
			if c.color == inkColors[0] {
				c.color = inkColors[4]
			}
		}),
		toolButton("mkUndo", "undo-2", "Undo", false, c.undo),
		toolButton("mkClear", "eraser", "Clear", false, func() { c.strokes = nil }),
	)
}

func (s *markupScreen) close() {
	properties.ForgetImage(s.canvas.img)
	s.a.pop()
}

func (s *markupScreen) done() {
	p := s.page
	p.Strokes = s.canvas.strokes
	p.Touch()
	s.doc.Modified = time.Now()
	s.a.save()
	s.close()
	renderInBackground(s.a, s.doc, []*library.Page{p})
}
