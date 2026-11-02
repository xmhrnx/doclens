package ui

import "github.com/xmhrnx/giode"

// sheetRow is a tappable row with an icon in a sheet or settings list.
func sheetRow(scope, key any, iconName, label, detail string, danger bool, onClick func()) giode.Element {
	return tappable(scope, key, onClick, func(s giode.PressState) giode.Element {
		fg, tint := cText, cPrimary
		if danger {
			fg, tint = cDanger, cDanger
		}
		bg := ""
		if s.Pressed {
			bg = cPressed
		}
		kids := []giode.Element{
			giode.Box(giode.Styles{Width: 38, Height: 38, BorderRadius: 12, Background: giode.Mix(tint, "theme.surface", 0.12),
				Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icon(iconName, 19, tint)),
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(label, giode.Styles{FontSize: 15, FontWeight: giode.FontWeightMedium, Color: fg})),
		}
		if detail != "" {
			kids = append(kids, giode.Text(detail, tCaption))
		}
		return giode.Row(giode.Styles{
			Width: -1, Align: giode.AlignCenter, Gap: 14, Wrap: giode.WrapNoWrap,
			Padding: giode.SymmetricInset(8, 8), BorderRadius: radiusTile, Background: bg,
		}, kids...)
	})
}
