package ui

import "github.com/xmhrnx/giode"

// topBar is a screen header: a leading control, a title and trailing
// controls.
func topBar(lead giode.Element, title string, studio bool, trailing ...giode.Element) giode.Element {
	color := cText
	if studio {
		color = cStudioText
	}
	kids := []giode.Element{}
	if lead != nil {
		kids = append(kids, lead)
	}
	kids = append(kids, giode.Box(giode.Styles{FlexGrow: 1},
		giode.Text(title, giode.Styles{FontSize: 18, FontWeight: giode.FontWeightBold, Color: color, TextOverflow: giode.TextOverflowEllipsis}),
	))
	kids = append(kids, trailing...)
	return giode.Row(giode.Styles{
		Width: -1, Align: giode.AlignCenter, Gap: 12, Wrap: giode.WrapNoWrap,
		Padding: giode.Inset{Top: 12, Bottom: 12, Left: 16, Right: 16},
	}, kids...)
}
