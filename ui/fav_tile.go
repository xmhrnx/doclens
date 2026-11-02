package ui

import "github.com/xmhrnx/giode"

// favTile is the favorite toggle: a star, filled amber when on.
func favTile(key any, on bool, onClick func()) giode.Element {
	return tappable("favTile", key, onClick, func(s giode.PressState) giode.Element {
		bg, fg, border := cSurface, cText, cBorder
		if on {
			bg, fg, border = giode.Mix("#F59E0B", "theme.surface", 0.16), "#F59E0B", "#F59E0B"
		}
		return giode.Box(giode.Styles{
			Width: 44, Height: 44, BorderRadius: radiusPill, Background: bg, BorderWidth: 1, BorderColor: border,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter, Transform: pressScale(s),
		}, icon("star", 20, fg))
	})
}
