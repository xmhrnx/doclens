package ui

import (
	"image"
	"image/color"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/sheet"

	"github.com/mehran/scanly/internal/library"
	"github.com/mehran/scanly/internal/scan"
)

// signatureSheet is a pad for drawing a new signature, which is saved
// for reuse.
type signatureSheet struct {
	a      *App
	shown  bool
	sh     *sheet.Sheet
	pad    *inkCanvas
	onSave func(*library.Signature, image.Image)
}

func newSignatureSheet(a *App) *signatureSheet {
	s := &signatureSheet{a: a, pad: newInkCanvas()}
	s.pad.blank = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	s.pad.width = 0.012
	s.pad.color = inkColors[0]
	s.sh = giode.Sheet(&s.shown, s.content, giode.Styles{Background: cSurface}).
		Title("Draw your signature").Footer(s.footer).Adaptive(true).Dismissible(true)
	return s
}

func (s *signatureSheet) open(onSave func(*library.Signature, image.Image)) {
	s.pad.strokes = nil
	s.onSave = onSave
	s.shown = true
}

func (s *signatureSheet) view() giode.Element { return s.sh }

func (s *signatureSheet) content() giode.Element {
	var swatches []giode.Element
	for i, col := range inkColors[:3] {
		col := col
		swatches = append(swatches, tappable("sigColor", i, func() { s.pad.color = col }, func(st giode.PressState) giode.Element {
			ring := "transparent"
			if s.pad.color == col {
				ring = cPrimary
			}
			return giode.Box(giode.Styles{Width: 32, Height: 32, BorderRadius: 16, BorderWidth: 2, BorderColor: ring,
				Align: giode.AlignCenter, Justify: giode.JustifyCenter},
				giode.Box(giode.Styles{Width: 22, Height: 22, BorderRadius: 11, Background: hexOf(col), BorderWidth: 1, BorderColor: cBorder}))
		}))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 12},
		giode.Box(giode.Styles{Width: -1, Height: 220, BorderRadius: radiusTile, Overflow: giode.OverflowHidden,
			BorderWidth: 1, BorderColor: cBorder}, s.pad),
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 8, Wrap: giode.WrapNoWrap},
			giode.Row(giode.Styles{Gap: 6, FlexGrow: 1}, swatches...),
			actionButton("sigUndo", buttonGhost, "undo-2", "Undo", false, s.pad.undo),
			actionButton("sigClear", buttonGhost, "eraser", "Clear", false, func() { s.pad.strokes = nil }),
		),
		giode.Text("Sign with your finger. Saved signatures can be placed on any page.", tCaption),
	)
}

func (s *signatureSheet) footer() giode.Element {
	return giode.Row(giode.Styles{Width: -1, Padding: giode.Inset{Top: 8, Bottom: 8}},
		actionButton("sigSave", buttonPrimary, "check", "Save signature", true, s.save))
}

func (s *signatureSheet) save() {
	strokes := s.pad.strokes
	if len(strokes) == 0 {
		s.a.fail(errNoSignature)
		return
	}
	area := s.pad.area
	s.shown = false
	a := s.a
	var img image.Image
	var sig *library.Signature
	a.work("Saving signature", func(func(float64)) error {
		img = signatureImage(strokes, area)
		return nil
	}, func() {
		var err error
		if sig, err = a.store.AddSignature(img); err != nil {
			a.fail(err)
			return
		}
		a.save()
		if s.onSave != nil {
			s.onSave(sig, img)
		}
	})
}

// signatureImage renders pad strokes at twice the pad's size on a
// transparent background, cropped to the ink.
func signatureImage(strokes []scan.Stroke, area image.Point) image.Image {
	w, h := area.X*2, area.Y*2
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	inked := scan.Annotate(canvas, strokes, nil)
	rgba, ok := inked.(*image.RGBA)
	if !ok {
		return inked
	}
	minX, minY, maxX, maxY := w, h, 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if rgba.Pix[y*rgba.Stride+x*4+3] > 0 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX <= minX || maxY <= minY {
		return rgba
	}
	const pad = 8
	r := image.Rect(max(0, minX-pad), max(0, minY-pad), min(w, maxX+pad), min(h, maxY+pad))
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		copy(out.Pix[y*out.Stride:], rgba.Pix[(r.Min.Y+y)*rgba.Stride+r.Min.X*4:(r.Min.Y+y)*rgba.Stride+r.Max.X*4])
	}
	return out
}
