package ui

import (
	"fmt"
	"image"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/biometric"
	"github.com/xmhrnx/giode/camera"
	"github.com/xmhrnx/giode/components/dropdown"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/giode/components/scroll"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/giode/ocr"

	"github.com/mehran/scanly/internal/library"
	"github.com/mehran/scanly/internal/scan"
)

var themes = []library.Theme{library.ThemeSystem, library.ThemeLight, library.ThemeDark}

// settingsScreen edits the user's preferences.
type settingsScreen struct {
	a       *App
	scroll  *scroll.Scroll
	filter  *dropdown.Select
	auto    *toggle.Switch
	skip    *toggle.Switch
	size    *segmented.Control
	quality *segmented.Control
	margins *toggle.Switch
	author  *input.Input
	theme   *segmented.Control
	lock    *toggle.Switch
	bio     *toggle.Switch
	capture *toggle.Switch
	text    *toggle.Switch
	pad     *signatureSheet
}

func newSettingsScreen(a *App) *settingsScreen {
	st := &a.store.Settings
	s := &settingsScreen{a: a, pad: newSignatureSheet(a)}
	sw := giode.Styles{Color: cText, FontSize: 15, Width: -1}
	names := make([]string, len(scan.Filters))
	for i, f := range scan.Filters {
		names[i] = filterNames[f]
	}
	s.filter = giode.Input.Select(giode.Styles{Background: cFill, Color: cText, BorderColor: cBorder, BorderRadius: radiusTile, Width: 170}, names...).
		Sheet(giode.SheetCompact).
		OnChange(func(i int) { st.DefaultFilter = scan.Filters[i]; a.save() })
	s.filter.SetSelected(indexOf(scan.Filters, st.DefaultFilter))
	s.auto = giode.Input.Switch("Find page edges automatically", sw).OnChange(func(on bool) { st.AutoCrop = on; a.save() })
	s.auto.SetChecked(st.AutoCrop)
	s.skip = giode.Input.Switch("Save without review when edges are found", sw).OnChange(func(on bool) { st.SkipReview = on; a.save() })
	s.skip.SetChecked(st.SkipReview)
	s.size = giode.Input.Segmented("Auto", "A4", "Letter", "Legal").Styles(giode.Styles{Width: -1}).
		OnChange(func(i int) { st.PageSize = pageSizes[i]; a.save() })
	s.size.SetSelected(indexOf(pageSizes, st.PageSize))
	s.quality = giode.Input.Segmented("Small", "Balanced", "Best").Styles(giode.Styles{Width: -1}).
		OnChange(func(i int) { st.Quality = qualities[i]; a.save() })
	s.quality.SetSelected(indexOf(qualities, st.Quality))
	s.margins = giode.Input.Switch("Page margins", sw).OnChange(func(on bool) { st.Margins = on; a.save() })
	s.margins.SetChecked(st.Margins)
	s.author = giode.Input.Text("Your name (PDF author)", giode.Styles{Background: cFill, BorderColor: cFill, BorderRadius: radiusTile, Color: cText, Padding: giode.SymmetricInset(14, 12)})
	s.author.SetText(st.Author)
	s.author.OnBlur(func() { st.Author = strings.TrimSpace(s.author.Text()); a.save() })
	s.theme = giode.Input.Segmented("System", "Light", "Dark").Styles(giode.Styles{Width: -1}).
		OnChange(func(i int) { st.Theme = themes[i]; a.applyThemeSetting(); a.save() })
	s.theme.SetSelected(indexOf(themes, st.Theme))
	s.lock = giode.Input.Switch("Lock with a PIN", sw).OnChange(s.toggleLock)
	s.lock.SetChecked(st.AppLock)
	s.bio = giode.Input.Switch(biometricLabel(), sw).OnChange(func(on bool) { st.Biometric = on; a.save() })
	s.bio.SetChecked(st.Biometric)
	s.capture = giode.Input.Switch("Capture automatically when the page is steady", sw).
		OnChange(func(on bool) { st.AutoCapture = on; a.save() })
	s.capture.SetChecked(st.AutoCapture)
	s.text = giode.Input.Switch("Recognize text for search", sw).OnChange(func(on bool) {
		st.AutoText = on
		a.save()
		if on {
			a.text.queueAll()
		}
	})
	s.text.SetChecked(st.AutoText)
	s.scroll = giode.Scroll(s.content, giode.Styles{ScrollBar: giode.ScrollBar{Hidden: true}})
	return s
}

func (s *settingsScreen) view() giode.Element {
	// The lock may have been set up (or abandoned) on the PIN screen.
	s.lock.SetChecked(s.a.store.Settings.AppLock)
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		topBar(iconTile("setBack", "chevron-left", false, s.leave), "Settings", false),
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, s.scroll),
		s.pad.view(),
	)
}

func (s *settingsScreen) leave() {
	st := &s.a.store.Settings
	if a := strings.TrimSpace(s.author.Text()); a != st.Author {
		st.Author = a
		s.a.save()
	}
	s.a.pop()
}

func (s *settingsScreen) content() giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Gap: 18, Padding: giode.Inset{Left: screenInset, Right: screenInset, Bottom: 32}},
		settingsGroup("Scanning",
			giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
				giode.Box(giode.Styles{FlexGrow: 1}, giode.Text("Default filter", giode.Styles{FontSize: 15, Color: cText})), s.filter),
			s.auto, s.liveCapture(), s.skip, s.textSwitch()),
		settingsGroup("Export", optionLabel("Page size"), s.size, optionLabel("Quality"), s.quality, s.margins, s.author),
		settingsGroup("Appearance", s.theme),
		settingsGroup("Privacy", s.lock, s.bioSwitch(),
			giode.Text("Scanly locks when it leaves the screen. Your documents never leave this device unless you share them.", with(tCaption, giode.Styles{LineHeight: 19}))),
		s.signatures(),
		s.about(),
	)
}

func settingsGroup(title string, kids ...giode.Element) giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Gap: 10},
		giode.Text(strings.ToUpper(title), with(tTiny, giode.Styles{Margin: giode.Inset{Left: 4}})),
		giode.Stack(giode.Styles{Width: -1, Gap: 14, Align: giode.AlignStretch, Padding: giode.UniformInset(16), Background: cSurface,
			BorderRadius: radiusCard, BorderWidth: 1, BorderColor: cBorder}, kids...),
	)
}

func (s *settingsScreen) signatures() giode.Element {
	a := s.a
	rows := []giode.Element{}
	for _, sg := range a.store.Signatures {
		sg := sg
		rows = append(rows, giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
			icon("signature", 20, cPrimary),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text("Saved "+relativeDate(sg.Created), tLabel)),
			iconTile("sigDel"+sg.ID, "trash-2", false, func() {
				giode.AfterLayout(func() {
					if err := a.store.DeleteSignature(sg.ID); err != nil {
						a.fail(err)
					}
					a.save()
				})
			})))
	}
	if len(rows) == 0 {
		rows = append(rows, giode.Text("No saved signatures.", tCaption))
	}
	rows = append(rows, actionButton("sigAdd", buttonSoft, "plus", "Add signature", false, func() {
		s.pad.open(func(*library.Signature, image.Image) { a.notify("Signature saved") })
	}))
	return settingsGroup("Signatures", rows...)
}

func (s *settingsScreen) about() giode.Element {
	st := s.a.store
	pages := 0
	for _, d := range st.Docs {
		pages += len(d.Pages)
	}
	return settingsGroup("About",
		giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 12, BackgroundGradient: brandGradient,
				Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icon("scan-line", 20, "#FFFFFF")),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
				giode.Text("Scanly "+Version, tHeading),
				giode.Text(fmt.Sprintf("%d documents · %d pages", len(st.Docs), pages), tCaption))),
		sheetRow("about", "licenses", "info", "Open-source licenses", "", false, func() { s.a.push(newLicensesScreen(s.a)) }),
	)
}

// toggleLock turns the PIN lock on (asking for a PIN) or off.
func (s *settingsScreen) toggleLock(on bool) {
	a := s.a
	if on {
		a.push(newPinSetupScreen(a))
		return
	}
	a.store.Settings.AppLock = false
	a.save()
	go func() {
		if err := clearPIN(); err != nil {
			a.fail(err)
		}
	}()
}

// liveCapture is shown where the app runs its own camera.
func (s *settingsScreen) liveCapture() giode.Element {
	if !camera.Supported() {
		return giode.Box(giode.Styles{Display: giode.DisplayNone})
	}
	s.capture.SetChecked(s.a.store.Settings.AutoCapture)
	return s.capture
}

// textSwitch is shown where the device can read text.
func (s *settingsScreen) textSwitch() giode.Element {
	if !ocr.Supported() {
		return giode.Box(giode.Styles{Display: giode.DisplayNone})
	}
	return s.text
}

// bioSwitch is shown with the PIN lock on a device with a fingerprint
// reader or face recognition.
func (s *settingsScreen) bioSwitch() giode.Element {
	if !s.a.store.Settings.AppLock || biometric.Available() == biometric.None {
		return giode.Box(giode.Styles{Display: giode.DisplayNone})
	}
	return s.bio
}

func biometricLabel() string {
	if biometric.Available() == biometric.Face {
		return "Unlock with Face ID"
	}
	return "Unlock with fingerprint"
}
