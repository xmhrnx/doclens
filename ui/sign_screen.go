package ui

import (
	"errors"
	"image"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/doclens/internal/library"
)

var errNoSignature = errors.New("draw your signature first")

// signScreen places saved signatures on a page.
type signScreen struct {
	a       *App
	doc     *library.Document
	page    *library.Page
	placer  *stampPlacer
	pad     *signatureSheet
	library map[string]image.Image // saved signatures by ID
}

// openSign loads the page and the saved signatures and opens the
// signing screen.
func openSign(a *App, d *library.Document, p *library.Page) {
	clone := p.Clone()
	sigs := append([]*library.Signature(nil), a.store.Signatures...)
	var base image.Image
	loaded := map[string]image.Image{}
	a.work("Opening the page", func(func(float64)) error {
		var err error
		if base, err = a.store.RenderBase(d.ID, clone, screenSide); err != nil {
			return err
		}
		for _, sg := range sigs {
			if img, err := a.store.LoadSignature(sg.ID); err == nil {
				loaded[sg.ID] = img
			}
		}
		return nil
	}, func() {
		s := &signScreen{a: a, doc: d, page: p, placer: newStampPlacer(), pad: newSignatureSheet(a), library: loaded}
		s.placer.page = base
		for _, ref := range p.Stamps {
			if img := loaded[ref.Signature]; img != nil {
				s.placer.stamps = append(s.placer.stamps, &placedStamp{ref: ref, img: img})
			}
		}
		a.push(s)
		if len(loaded) == 0 {
			s.pad.open(s.add)
		}
	})
}

func (s *signScreen) view() giode.Element {
	hint := "Pick a signature below, then drag it into place. Drag the corner to resize."
	if len(s.placer.stamps) > 0 {
		hint = "Drag to move, drag the corner to resize."
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Background: cStudio},
		topBar(iconTile("sgClose", "x", true, s.close), "Sign", true,
			actionButton("sgDone", buttonStudio, "check", "Done", false, s.done)),
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, s.placer),
		giode.Box(giode.Styles{Width: -1, Justify: giode.JustifyCenter, Padding: giode.SymmetricInset(20, 6)},
			giode.Text(hint, giode.Styles{FontSize: 13, Color: cStudioMuted, TextAlign: giode.TextAlignCenter})),
		s.shelf(),
		s.pad.view(),
	)
}

// shelf lists the saved signatures, a button to draw a new one and one
// to remove the selected signature.
func (s *signScreen) shelf() giode.Element {
	kids := []giode.Element{
		tappable("sigNew", "new", func() { s.pad.open(s.add) }, func(st giode.PressState) giode.Element {
			return giode.Stack(giode.Styles{Width: 96, Height: 64, BorderRadius: radiusTile, BorderWidth: 1, BorderColor: cStudioLine,
				Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: 4, Transform: pressScale(st)},
				icon("plus", 20, cStudioText),
				giode.Text("New", giode.Styles{FontSize: 11, Color: cStudioMuted}))
		}),
	}
	for _, sg := range s.a.store.Signatures {
		sg := sg
		img := s.library[sg.ID]
		if img == nil {
			continue
		}
		kids = append(kids, tappable("sigPick", sg.ID, func() { s.place(sg, img) }, func(st giode.PressState) giode.Element {
			return giode.Box(giode.Styles{Width: 96, Height: 64, BorderRadius: radiusTile, Background: "#FFFFFF",
				Padding: giode.UniformInset(6), Transform: pressScale(st)},
				giode.Image(img, giode.Styles{Width: -1, Height: -1, Fit: giode.FitContain}))
		}))
	}
	row := giode.Row(giode.Styles{Gap: 10, Wrap: giode.WrapNoWrap}, kids...)
	tools := []giode.Element{giode.Box(giode.Styles{FlexGrow: 1}, row)}
	if s.placer.selected >= 0 && s.placer.selected < len(s.placer.stamps) {
		tools = append(tools, toolButton("sigRemove", "trash-2", "Remove", false, s.removeSelected))
	}
	return giode.Row(giode.Styles{
		Width: -1, Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Top: 14, Bottom: 22, Left: 16, Right: 16},
		Background: cStudioPanel, BorderRadii: giode.TopCorners(24),
	}, tools...)
}

// add places a newly drawn signature.
func (s *signScreen) add(sg *library.Signature, img image.Image) {
	s.library[sg.ID] = img
	s.place(sg, img)
}

func (s *signScreen) place(sg *library.Signature, img image.Image) {
	ref := library.StampRef{Signature: sg.ID, X: 0.55, Y: 0.78, W: 0.32, Aspect: sg.Aspect}
	b := s.placer.page.Bounds()
	if h := ref.W * float64(b.Dx()) / ref.Aspect / float64(b.Dy()); ref.Y+h > 0.98 {
		ref.Y = max(0, 0.98-h)
	}
	s.placer.stamps = append(s.placer.stamps, &placedStamp{ref: ref, img: img})
	s.placer.selected = len(s.placer.stamps) - 1
	giode.Haptic(giode.HapticLight)
}

func (s *signScreen) removeSelected() {
	i := s.placer.selected
	giode.AfterLayout(func() {
		if i >= 0 && i < len(s.placer.stamps) {
			s.placer.stamps = append(s.placer.stamps[:i], s.placer.stamps[i+1:]...)
		}
		s.placer.selected = -1
	})
}

func (s *signScreen) close() {
	properties.ForgetImage(s.placer.page)
	s.a.pop()
}

func (s *signScreen) done() {
	p := s.page
	p.Stamps = nil
	for _, st := range s.placer.stamps {
		p.Stamps = append(p.Stamps, st.ref)
	}
	p.Touch()
	s.doc.Modified = time.Now()
	s.a.save()
	s.close()
	renderInBackground(s.a, s.doc, []*library.Page{p})
	s.a.notify("Signed")
}
