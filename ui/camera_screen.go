package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/camera"
	"github.com/xmhrnx/giode/components/kit"

	"github.com/mehran/scanly/internal/scan"
)

const (
	// steadyFor is how long the page must hold still before an
	// automatic capture.
	steadyFor = 900 * time.Millisecond
	// steadyMove is how far (a fraction of the frame) a corner may
	// wander and still count as still.
	steadyMove = 0.018
	// lostAfter hides the outline once no page has been seen this long.
	lostAfter = 600 * time.Millisecond
	// autoPause keeps a batch from shooting the same page twice.
	autoPause = 1800 * time.Millisecond
)

// cameraScreen is the live scanner: the camera picture with the page
// outline found in it as you aim, a shot taken by itself once the page
// holds still, the torch, and the scan modes.
type cameraScreen struct {
	a *App
	f *scanFlow

	cam     *camera.Camera
	gen     int // invalidates a camera still opening when the screen hides
	opening bool
	denied  bool
	err     string

	// The outline, in frame fractions (0..1).
	target, drawn scan.Quad
	found         bool
	seen          time.Time // the last frame with a page
	steadySince   time.Time
	lastShot      time.Time

	auto       bool
	shooting   bool
	processing int // shots being decoded and analysed
	flash      time.Time

	discard *confirmDialog
}

func newCameraScreen(f *scanFlow) *cameraScreen {
	// The camera opens in shown, once the screen is pushed.
	return &cameraScreen{a: f.a, f: f, auto: f.a.store.Settings.AutoCapture, discard: newConfirmDialog()}
}

// shown opens the camera when the scanner comes on top.
func (s *cameraScreen) shown() {
	if s.cam != nil || s.opening {
		return
	}
	s.opening, s.denied, s.err = true, false, ""
	s.found = false
	s.gen++
	gen := s.gen
	a := s.a
	go func() {
		cam, err := camera.Open(context.Background(), a.g, camera.Options{
			AnalysisRate: 12,
			OnFrame:      func(fr camera.Frame) { s.analyse(gen, fr) },
			OnError:      func(err error) { a.g.Post(func() { s.err = err.Error() }) },
		})
		a.g.Post(func() {
			s.opening = false
			if gen != s.gen {
				// Hidden while opening.
				if cam != nil {
					cam.Close()
				}
				return
			}
			switch {
			case errors.Is(err, camera.ErrPermissionDenied):
				s.denied = true
			case err != nil:
				s.err = err.Error()
			default:
				s.cam = cam
			}
		})
	}()
}

// hidden releases the camera when the scanner is covered or closed.
func (s *cameraScreen) hidden() {
	s.gen++
	if s.cam != nil {
		s.cam.Close()
		s.cam = nil
	}
}

// analyse finds the page in an analysis frame. It runs on the camera's
// goroutine.
func (s *cameraScreen) analyse(gen int, fr camera.Frame) {
	d := scan.DetectPreview(fr.Image)
	b := fr.Image.Rect
	q := d.Quad.Scale(1/float64(b.Dx()), 1/float64(b.Dy()))
	at := fr.Time
	s.a.g.Post(func() {
		if gen == s.gen {
			s.update(d.Found, q, at)
		}
	})
}

// update takes a new outline and decides whether to shoot.
func (s *cameraScreen) update(found bool, q scan.Quad, at time.Time) {
	if !found {
		if at.Sub(s.seen) > lostAfter {
			s.found = false
		}
		s.steadySince = time.Time{}
		return
	}
	if !s.found {
		s.drawn = q
		s.steadySince = at
	} else if maxShift(s.target, q) > steadyMove {
		s.steadySince = at
	}
	s.found, s.target, s.seen = true, q, at
	if s.auto && s.ready() && at.Sub(s.steadySince) >= steadyFor {
		s.shoot()
	}
}

// ready reports whether a shot may be taken now.
func (s *cameraScreen) ready() bool {
	return s.cam != nil && !s.shooting && time.Since(s.lastShot) > autoPause && !s.full()
}

// full reports whether the mode has all its pages: one photo for a page
// or a book spread, two sides of a card; a batch is never full.
func (s *cameraScreen) full() bool {
	perShot := 1
	if s.f.mode == scanBook {
		perShot = 2 // a spread becomes two pages
	}
	n := len(s.f.pages) + s.processing*perShot
	switch s.f.mode {
	case scanBatch:
		return false
	case scanIDCard:
		return n >= 2
	case scanBook:
		return n >= 2 // the spread's two halves
	}
	return n >= 1
}

func maxShift(a, b scan.Quad) float64 {
	m := 0.0
	for i := range a {
		m = max(m, a[i].Dist(b[i]))
	}
	return m
}

// steadiness is how far along the wait for an automatic shot is, 0..1.
func (s *cameraScreen) steadiness() float64 {
	if !s.auto || !s.found || s.steadySince.IsZero() {
		return 0
	}
	return min(1, float64(time.Since(s.steadySince))/float64(steadyFor))
}

// shoot captures a full resolution photo.
func (s *cameraScreen) shoot() {
	if s.cam == nil || s.shooting {
		return
	}
	cam, a := s.cam, s.a
	s.shooting, s.lastShot, s.flash = true, time.Now(), time.Now()
	s.steadySince = time.Time{}
	giode.Haptic(giode.HapticMedium)
	go func() {
		data, err := cam.Capture(context.Background())
		a.g.Post(func() {
			s.shooting = false
			switch {
			case errors.Is(err, camera.ErrClosed):
				// The scanner closed while the shot was taken.
			case err != nil:
				a.fail(err)
			default:
				s.develop(data)
			}
		})
	}()
}

// develop finds the page in a shot at full resolution, while the
// camera keeps running for the next one.
func (s *cameraScreen) develop(data []byte) {
	s.processing++
	f, a := s.f, s.a
	go func() {
		pages, err := f.detectIn(data)
		a.g.Post(func() {
			s.processing--
			if err != nil {
				a.fail(err)
				return
			}
			f.add(pages)
		})
	}()
}

func (s *cameraScreen) view() giode.Element {
	if len(s.f.pages) > 0 {
		giode.HandleBack(s.askDiscard)
	}
	return giode.Box(giode.Styles{Width: -1, Height: -1, Background: "#000000"},
		s.viewfinder(),
		giode.Stack(giode.Styles{Position: giode.PositionAbsolute, Left: giode.Flush, Right: giode.Flush, Top: giode.Flush, Bottom: giode.Flush, ZIndex: 2},
			s.topControls(),
			giode.Box(giode.Styles{FlexGrow: 1}),
			s.hint(),
			s.modes(),
			s.bottomControls(),
		),
		s.discard.view(),
	)
}

func (s *cameraScreen) viewfinder() giode.Element {
	if s.cam == nil {
		return s.placeholder()
	}
	return s.cam.Preview(camera.PreviewOptions{Fill: true, Overlay: s.drawOutline})
}

// placeholder covers the screen while the camera opens, or explains why
// it didn't.
func (s *cameraScreen) placeholder() giode.Element {
	var kids []giode.Element
	switch {
	case s.denied:
		kids = []giode.Element{
			icon("camera", 40, cStudioMuted),
			giode.Text("Scanly needs the camera to scan", giode.Styles{FontSize: 17, FontWeight: giode.FontWeightSemiBold, Color: cStudioText}),
			giode.Text("Allow camera access in the system settings, or use the system camera for now.",
				giode.Styles{FontSize: 14, Color: cStudioMuted, TextAlign: giode.TextAlignCenter}),
			actionButton("camFallback", buttonStudio, "camera", "Use the system camera", false, s.useSystemCamera),
		}
	case s.err != "":
		kids = []giode.Element{
			icon("camera", 40, cStudioMuted),
			giode.Text("The camera isn't available", giode.Styles{FontSize: 17, FontWeight: giode.FontWeightSemiBold, Color: cStudioText}),
			giode.Text(s.err, giode.Styles{FontSize: 14, Color: cStudioMuted, TextAlign: giode.TextAlignCenter}),
			actionButton("camRetry", buttonStudio, "refresh-cw", "Try again", false, func() { s.hidden(); s.shown() }),
		}
	default:
		kids = []giode.Element{giode.Spinner(giode.Styles{Width: 32, Height: 32, Color: cAccent})}
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: 14,
		Padding: giode.SymmetricInset(32, 0), Background: cStudio}, kids...)
}

// useSystemCamera hands this flow over to the system camera app.
func (s *cameraScreen) useSystemCamera() {
	f := s.f
	giode.AfterLayout(func() {
		f.cam = nil
		f.a.pop()
		f.capture()
	})
}

var (
	colOutlineFill  = color.NRGBA{R: 0x7B, G: 0x7B, B: 0xFF, A: 0x38}
	colOutlineReady = color.NRGBA{R: 0x34, G: 0xD3, B: 0x99, A: 0xFF}
	colFlash        = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

// drawOutline draws the page outline over the picture, gliding toward
// each new detection, and the shutter flash.
func (s *cameraScreen) drawOutline(gtx layout.Context, toView func(f32.Point) f32.Point) {
	if since := time.Since(s.flash); since < 180*time.Millisecond {
		a := uint8(200 * (1 - float64(since)/float64(180*time.Millisecond)))
		c := colFlash
		c.A = a
		paint.FillShape(gtx.Ops, c, clip.Rect{Max: gtx.Constraints.Max}.Op())
		kit.RequestFrame(gtx)
	}
	if !s.found {
		return
	}
	moving := false
	for i := range s.drawn {
		d := s.target[i].Sub(s.drawn[i])
		if d.Len() > 0.001 {
			moving = true
		}
		s.drawn[i] = s.drawn[i].Add(d.Mul(0.35))
	}
	if moving {
		kit.RequestFrame(gtx)
	}
	pts := make([]f32.Point, 4)
	for i, p := range s.drawn {
		pts[i] = toView(f32.Pt(float32(p.X), float32(p.Y)))
	}
	line := colAccent
	if s.steadiness() >= 0.999 || s.shooting {
		line = colOutlineReady
	}
	var fill clip.Path
	fill.Begin(gtx.Ops)
	fill.MoveTo(pts[0])
	for _, p := range pts[1:] {
		fill.LineTo(p)
	}
	fill.Close()
	paint.FillShape(gtx.Ops, colOutlineFill, clip.Outline{Path: fill.End()}.Op())
	var edge clip.Path
	edge.Begin(gtx.Ops)
	edge.MoveTo(pts[0])
	for _, p := range pts[1:] {
		edge.LineTo(p)
	}
	edge.Close()
	paint.FillShape(gtx.Ops, line, clip.Stroke{Path: edge.End(), Width: float32(gtx.Dp(3))}.Op())
	for _, p := range pts {
		circle(gtx.Ops, p, float32(gtx.Dp(7)), line)
		circle(gtx.Ops, p, float32(gtx.Dp(4)), colWhite)
	}
	if s.auto && !s.steadySince.IsZero() {
		// Keep animating the shutter ring while the page holds still.
		kit.RequestFrame(gtx)
	}
}

func (s *cameraScreen) topControls() giode.Element {
	kids := []giode.Element{iconTile("camClose", "x", true, s.close), giode.Box(giode.Styles{FlexGrow: 1})}
	if s.cam != nil && s.cam.HasTorch() {
		torch := "flashlight-off"
		if s.cam.TorchOn() {
			torch = "flashlight"
		}
		cam := s.cam
		kids = append(kids, iconTile("camTorch", torch, true, func() {
			go func() {
				if err := cam.Torch(!cam.TorchOn()); err != nil {
					s.a.fail(err)
				}
			}()
		}))
	}
	kids = append(kids, s.autoToggle())
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap,
		Padding: giode.Inset{Top: 12, Left: 16, Right: 16, Bottom: 8}}, kids...)
}

// autoToggle switches automatic capture.
func (s *cameraScreen) autoToggle() giode.Element {
	return tappable("camAuto", "auto", func() {
		s.auto = !s.auto
		s.a.store.Settings.AutoCapture = s.auto
		s.a.save()
	}, func(st giode.PressState) giode.Element {
		bg, fg, label := "#12141ECC", cStudioMuted, "Auto off"
		if s.auto {
			bg, fg, label = cAccent, "#FFFFFF", "Auto"
		}
		return giode.Row(giode.Styles{Align: giode.AlignCenter, Gap: 6, Padding: giode.SymmetricInset(14, 11),
			BorderRadius: radiusPill, Background: bg, Transform: pressScale(st)},
			icon("scan", 16, fg),
			giode.Text(label, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: fg}))
	})
}

// hint says what the scanner is waiting for.
func (s *cameraScreen) hint() giode.Element {
	msg := ""
	switch {
	case s.cam == nil:
	case s.full() && s.f.mode == scanIDCard:
		msg = "Both sides captured"
	case s.full():
		msg = "Tap the picture to continue"
	case s.shooting:
		msg = "Capturing…"
	case !s.found:
		msg = "Point at a document"
	case s.auto && s.f.mode == scanIDCard && len(s.f.pages) == 1:
		msg = "Back side: hold steady"
	case s.auto:
		msg = "Hold steady…"
	default:
		msg = "Page found. Tap the shutter."
	}
	if s.f.mode == scanIDCard && len(s.f.pages) == 0 && s.cam != nil && !s.found {
		msg = "Front of the card"
	}
	if msg == "" {
		return giode.Box(giode.Styles{})
	}
	return giode.Box(giode.Styles{Width: -1, Justify: giode.JustifyCenter, Padding: giode.Inset{Bottom: 12}},
		giode.Box(giode.Styles{Padding: giode.SymmetricInset(16, 9), BorderRadius: radiusPill, Background: "#07080DB3"},
			giode.Text(msg, giode.Styles{FontSize: 14, FontWeight: giode.FontWeightMedium, Color: "#FFFFFF"})))
}

var modeNames = []struct {
	mode  scanMode
	label string
}{
	{scanSingle, "Document"},
	{scanBatch, "Batch"},
	{scanIDCard, "ID card"},
	{scanBook, "Book"},
}

// modes switches the scan mode until the first shot.
func (s *cameraScreen) modes() giode.Element {
	if len(s.f.pages) > 0 || s.f.replace != nil {
		return giode.Box(giode.Styles{})
	}
	var chips []giode.Element
	for _, m := range modeNames {
		m := m
		if m.mode == scanIDCard && s.f.doc != nil {
			continue // a card is a document of its own
		}
		chips = append(chips, tappable("camMode", m.mode, func() { s.f.mode = m.mode }, func(st giode.PressState) giode.Element {
			fg, bg := cStudioMuted, "transparent"
			if s.f.mode == m.mode {
				fg, bg = "#FFE58A", "#07080D99"
			}
			return giode.Box(giode.Styles{Padding: giode.SymmetricInset(12, 7), BorderRadius: radiusPill, Background: bg},
				giode.Text(m.label, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: fg, TextTransform: giode.TextTransformUppercase}))
		}))
	}
	return giode.Row(giode.Styles{Width: -1, Justify: giode.JustifyCenter, Gap: 4, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Bottom: 10}}, chips...)
}

func (s *cameraScreen) bottomControls() giode.Element {
	return giode.Row(giode.Styles{
		Width: -1, Align: giode.AlignCenter, Justify: giode.JustifySpaceBetween, Wrap: giode.WrapNoWrap,
		Padding: giode.Inset{Left: 28, Right: 28, Top: 14, Bottom: 30}, Background: "#07080DCC",
	},
		s.galleryButton(),
		s.shutter(),
		s.pagesButton(),
	)
}

func (s *cameraScreen) galleryButton() giode.Element {
	return tappable("camGallery", "gallery", s.importPhotos, func(st giode.PressState) giode.Element {
		return giode.Box(giode.Styles{Width: 54, Height: 54, BorderRadius: 16, Background: "#FFFFFF1F",
			Align: giode.AlignCenter, Justify: giode.JustifyCenter, Transform: pressScale(st)}, icon("images", 24, "#FFFFFF"))
	})
}

// importPhotos adds pictures from the library to this scan.
func (s *cameraScreen) importPhotos() {
	s.f.importPhotos()
}

// shutter takes a picture; its ring fills as the page holds still for
// an automatic shot.
func (s *cameraScreen) shutter() giode.Element {
	progress := s.steadiness()
	busy := s.shooting
	return tappable("camShutter", "shutter", func() {
		if s.cam != nil && !s.full() {
			s.shoot()
		}
	}, func(st giode.PressState) giode.Element {
		return giode.Canvas(giode.Styles{Width: 80, Height: 80}, func(p *giode.Painter) {
			c := p.Width() / 2
			p.Arc(c, c, c-4, 4, 0, 360, "#FFFFFF66")
			if progress > 0 {
				p.Arc(c, c, c-4, 4, 0, float32(360*progress), "#34D399")
			}
			inner := c - 11
			if st.Pressed || busy {
				inner -= 4
			}
			p.Circle(c, c, inner, "#FFFFFF")
		})
	})
}

// pagesButton shows the pages taken so far; tapping it finishes.
func (s *cameraScreen) pagesButton() giode.Element {
	n := len(s.f.pages)
	if n == 0 && s.processing == 0 {
		return giode.Box(giode.Styles{Width: 54, Height: 54})
	}
	var last image.Image
	if n > 0 {
		last = s.f.pages[n-1].src.view
	}
	return tappable("camPages", "pages", s.finish, func(st giode.PressState) giode.Element {
		var thumb giode.Element = giode.Spinner(giode.Styles{Width: 22, Height: 22, Color: "#FFFFFF"})
		if last != nil {
			thumb = giode.Image(last, giode.Styles{Width: 54, Height: 54, Fit: giode.FitCover})
		}
		label := fmt.Sprint(n)
		if s.processing > 0 {
			label = fmt.Sprintf("%d+", n)
		}
		return giode.Box(giode.Styles{Width: 54, Height: 54, Transform: pressScale(st)},
			giode.Box(giode.Styles{Width: 54, Height: 54, BorderRadius: 14, BorderWidth: 2, BorderColor: "#FFFFFF",
				Overflow: giode.OverflowHidden, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Background: "#FFFFFF1F"}, thumb),
			giode.Box(giode.Styles{Position: giode.PositionAbsolute, Top: -8, Right: -8, ZIndex: 2, MinWidth: 22, Height: 22,
				BorderRadius: 11, Background: cAccent, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Padding: giode.SymmetricInset(6, 0)},
				giode.Text(label, giode.Styles{FontSize: 12, FontWeight: giode.FontWeightBold, Color: "#FFFFFF"})),
		)
	})
}

// finish moves on to reviewing the pages taken.
func (s *cameraScreen) finish() {
	if s.processing > 0 {
		s.a.notify("Still processing the last shot…")
		return
	}
	if len(s.f.pages) > 0 {
		s.f.review()
	}
}

// close leaves the scanner, asking first if pages would be lost.
func (s *cameraScreen) close() {
	if len(s.f.pages) > 0 {
		s.askDiscard()
		return
	}
	s.f.close()
}

func (s *cameraScreen) askDiscard() {
	n := len(s.f.pages)
	s.discard.open(fmt.Sprintf("Discard %d page%s?", n, plural(n)),
		"The pages you captured haven't been saved.", "Discard", true, s.f.close)
}
