package ui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/biometric"
)

// lockScreen asks for the PIN before showing the library.
type lockScreen struct {
	a        *App
	pad      *pinPad
	message  string
	failures int
	// asked is set once the biometric prompt was offered for this lock.
	asked     bool
	prompting bool
}

func newLockScreen(a *App) *lockScreen {
	l := &lockScreen{a: a, pad: &pinPad{}}
	l.pad.onComplete = l.check
	return l
}

func (l *lockScreen) reset() {
	l.pad.digits, l.message = "", ""
	l.asked = false
}

// canUseBiometric reports whether a fingerprint or face may unlock.
func (l *lockScreen) canUseBiometric() bool {
	return l.a.store.Settings.Biometric && biometric.Available() != biometric.None
}

// unlockWithBiometric shows the system's biometric prompt.
func (l *lockScreen) unlockWithBiometric() {
	if l.prompting {
		return
	}
	l.asked, l.prompting = true, true
	a := l.a
	go func() {
		err := biometric.Authenticate(context.Background(), a.g, biometric.Options{
			Reason: "Unlock your documents",
			Title:  "Unlock Scanly",
			Cancel: "Use PIN",
		})
		a.g.Post(func() {
			l.prompting = false
			switch {
			case err == nil:
				l.failures, l.message = 0, ""
				a.locked = false
			case errors.Is(err, biometric.ErrCanceled):
			case errors.Is(err, biometric.ErrLockout):
				l.message = "Too many attempts. Enter your PIN."
			default:
				l.message = "Not recognised. Enter your PIN."
			}
		})
	}()
}

func (l *lockScreen) check(pin string) {
	a := l.a
	l.pad.disabled = true
	go func() {
		ok, err := checkPIN(pin)
		a.g.Post(func() {
			l.pad.disabled = false
			l.pad.digits = ""
			switch {
			case err != nil:
				l.message = "Couldn't read the PIN: " + err.Error()
			case ok:
				l.failures, l.message = 0, ""
				a.locked = false
			default:
				l.failures++
				giode.Haptic(giode.HapticWarning)
				l.message = "Wrong PIN. Try again."
				if l.failures >= 5 {
					l.lockOut()
				}
			}
		})
	}()
}

// lockOut pauses entry after repeated wrong PINs.
func (l *lockScreen) lockOut() {
	const wait = 30 * time.Second
	l.pad.disabled = true
	l.message = fmt.Sprintf("Too many attempts. Wait %d seconds.", int(wait.Seconds()))
	time.AfterFunc(wait, func() {
		l.a.g.Post(func() {
			l.pad.disabled, l.failures, l.message = false, 0, ""
		})
	})
}

func (l *lockScreen) view() giode.Element {
	if !l.canUseBiometric() {
		return pinScreenLayout("Scanly is locked", "Enter your PIN", l.message, l.pad)
	}
	if !l.asked {
		// Offer the fingerprint or face as soon as the lock shows.
		giode.AfterLayout(l.unlockWithBiometric)
	}
	label := "Use fingerprint"
	name := "fingerprint"
	if biometric.Available() == biometric.Face {
		label, name = "Use Face ID", "scan-face"
	}
	return giode.Box(giode.Styles{Width: -1, Height: -1},
		pinScreenLayout("Scanly is locked", "Enter your PIN", l.message, l.pad),
		giode.Box(giode.Styles{Position: giode.PositionAbsolute, Left: giode.Flush, Right: giode.Flush, Bottom: 28, Justify: giode.JustifyCenter, ZIndex: 2},
			actionButton("lockBio", buttonGhost, name, label, false, l.unlockWithBiometric)),
	)
}

// pinScreenLayout centers a PIN pad under a title.
func pinScreenLayout(title, subtitle, message string, pad *pinPad) giode.Element {
	msg := giode.Element(giode.Box(giode.Styles{Height: 20}))
	if message != "" {
		msg = giode.Text(message, giode.Styles{FontSize: 14, Color: cDanger, TextAlign: giode.TextAlignCenter})
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter, Gap: 18, Background: cBg},
		giode.Box(giode.Styles{Width: 64, Height: 64, BorderRadius: 20, BackgroundGradient: brandGradient, BoxShadow: shadowFab,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icon("lock-keyhole", 30, "#FFFFFF")),
		giode.Text(title, tTitle),
		giode.Text(subtitle, tCaption),
		msg,
		pad.view(),
	)
}
