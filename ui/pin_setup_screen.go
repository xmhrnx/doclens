package ui

import "github.com/xmhrnx/giode"

// pinSetupScreen chooses a new PIN: typed twice.
type pinSetupScreen struct {
	a       *App
	pad     *pinPad
	first   string
	message string
}

func newPinSetupScreen(a *App) *pinSetupScreen {
	s := &pinSetupScreen{a: a, pad: &pinPad{}}
	s.pad.onComplete = s.entered
	return s
}

func (s *pinSetupScreen) entered(pin string) {
	s.pad.digits = ""
	if s.first == "" {
		s.first, s.message = pin, ""
		return
	}
	if pin != s.first {
		s.first, s.message = "", "The PINs didn't match. Start again."
		giode.Haptic(giode.HapticWarning)
		return
	}
	a := s.a
	s.pad.disabled = true
	go func() {
		err := savePIN(pin)
		a.g.Post(func() {
			s.pad.disabled = false
			if err != nil {
				a.fail(err)
				return
			}
			a.store.Settings.AppLock = true
			a.save()
			a.notify("App lock is on")
			a.pop()
		})
	}()
}

func (s *pinSetupScreen) view() giode.Element {
	sub := "Choose a 4-digit PIN"
	if s.first != "" {
		sub = "Enter it again to confirm"
	}
	return giode.Box(giode.Styles{Width: -1, Height: -1},
		pinScreenLayout("Set up app lock", sub, s.message, s.pad),
		giode.Box(giode.Styles{Position: giode.PositionAbsolute, Top: 12, Left: 16, ZIndex: 2},
			iconTile("pinBack", "x", false, s.a.pop)),
	)
}
