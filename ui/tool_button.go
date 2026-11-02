package ui

import "github.com/xmhrnx/giode"

// toolButton is an icon over a short label, for editor toolbars on the
// dark editor screens.
func toolButton(key any, iconName, label string, active bool, onClick func()) giode.Element {
	return tappable("tool", key, onClick, func(s giode.PressState) giode.Element {
		fg := cStudioMuted
		bg := ""
		if active {
			fg, bg = cStudioText, "#23263A"
		}
		if s.Pressed {
			bg = "#2C3048"
		}
		return giode.Stack(giode.Styles{
			Align: giode.AlignCenter, Gap: 6, MinWidth: 50,
			Padding: giode.SymmetricInset(6, 8), BorderRadius: radiusTile, Background: bg,
		},
			icon(iconName, 22, fg),
			giode.Text(label, giode.Styles{FontSize: 11, FontWeight: giode.FontWeightMedium, Color: fg}),
		)
	})
}
