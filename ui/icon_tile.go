package ui

import "github.com/xmhrnx/giode"

// iconTile is a round icon button for top bars.
func iconTile(key any, iconName string, studio bool, onClick func()) giode.Element {
	return tappable("iconTile", key, onClick, func(s giode.PressState) giode.Element {
		bg, fg := cSurface, cText
		if studio {
			bg, fg = cStudioPanel, cStudioText
		}
		st := giode.Styles{
			Width: 44, Height: 44, BorderRadius: radiusPill, Background: bg,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter, Transform: pressScale(s),
		}
		if !studio {
			st.BorderWidth, st.BorderColor = 1, cBorder
		}
		return giode.Box(st, icon(iconName, 20, fg))
	})
}
