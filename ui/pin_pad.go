package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
)

const pinLength = 4

// pinPad is a PIN entry: dots for the digits typed and a keypad.
type pinPad struct {
	digits   string
	disabled bool
	// onComplete runs when pinLength digits are in.
	onComplete func(pin string)
}

func (p *pinPad) press(d string) {
	if p.disabled || len(p.digits) >= pinLength {
		return
	}
	p.digits += d
	giode.Haptic(giode.HapticLight)
	if len(p.digits) == pinLength && p.onComplete != nil {
		pin := p.digits
		giode.AfterLayout(func() { p.onComplete(pin) })
	}
}

func (p *pinPad) erase() {
	if n := len(p.digits); n > 0 {
		p.digits = p.digits[:n-1]
	}
}

func (p *pinPad) view() giode.Element {
	var dots []giode.Element
	for i := 0; i < pinLength; i++ {
		bg := cBorder
		if i < len(p.digits) {
			bg = cPrimary
		}
		dots = append(dots, giode.Box(giode.Styles{Width: 16, Height: 16, BorderRadius: 8, Background: bg}))
	}
	key := func(label string, fn func()) giode.Element {
		return tappable("pinKey", label, fn, func(s giode.PressState) giode.Element {
			bg := cSurface
			if s.Pressed {
				bg = cPrimarySoft
			}
			var inner giode.Element = giode.Text(label, giode.Styles{FontSize: 26, FontWeight: giode.FontWeightSemiBold, Color: cText})
			if label == "⌫" {
				inner = icon("delete", 24, cText)
			}
			return giode.Box(giode.Styles{Width: 76, Height: 76, BorderRadius: 38, Background: bg, BorderWidth: 1, BorderColor: cBorder,
				Align: giode.AlignCenter, Justify: giode.JustifyCenter, Transform: pressScale(s)}, inner)
		})
	}
	var rows []giode.Element
	for r := 0; r < 3; r++ {
		var ks []giode.Element
		for c := 1; c <= 3; c++ {
			d := fmt.Sprint(r*3 + c)
			ks = append(ks, key(d, func() { p.press(d) }))
		}
		rows = append(rows, giode.Row(giode.Styles{Gap: 22, Wrap: giode.WrapNoWrap}, ks...))
	}
	rows = append(rows, giode.Row(giode.Styles{Gap: 22, Wrap: giode.WrapNoWrap},
		giode.Box(giode.Styles{Width: 76, Height: 76}),
		key("0", func() { p.press("0") }),
		key("⌫", p.erase),
	))
	return giode.Stack(giode.Styles{Align: giode.AlignCenter, Gap: 34},
		giode.Row(giode.Styles{Gap: 18, Wrap: giode.WrapNoWrap}, dots...),
		giode.Stack(giode.Styles{Align: giode.AlignCenter, Gap: 18}, rows...),
	)
}
