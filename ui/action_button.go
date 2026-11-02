package ui

import "github.com/xmhrnx/giode"

// buttonKind picks an actionButton's look.
type buttonKind int

const (
	buttonPrimary buttonKind = iota // the brand gradient
	buttonSoft                      // a tinted fill
	buttonGhost                     // text only
	buttonDanger
	buttonStudio // light on the dark editor screens
)

// actionButton is a rounded pill button with an optional icon.
func actionButton(key any, kind buttonKind, iconName, label string, grow bool, onClick func()) giode.Element {
	tap := tappable
	if grow {
		tap = growTappable
	}
	return tap("action", key, onClick, func(s giode.PressState) giode.Element {
		st := giode.Styles{
			Padding:      giode.SymmetricInset(20, 14),
			BorderRadius: radiusPill,
			Align:        giode.AlignCenter,
			Justify:      giode.JustifyCenter,
			Gap:          8,
			Transform:    pressScale(s),
		}
		fg := cOnPrim
		switch kind {
		case buttonPrimary:
			st.BackgroundGradient = brandGradient
			st.BoxShadow = giode.BoxShadow{Y: 6, Blur: 18, Color: "#5B5BF04D"}
		case buttonSoft:
			st.Background, fg = cPrimarySoft, cPrimary
		case buttonGhost:
			fg = cPrimary
			if s.Pressed {
				st.Background = cPrimarySoft
			}
		case buttonDanger:
			st.Background, fg = cDangerSoft, cDanger
		case buttonStudio:
			st.Background, fg = "#FFFFFF", "#0E1222"
		}
		if s.Disabled {
			st.Opacity = 0.45
		}
		if grow {
			st.Width = -1
		}
		kids := []giode.Element{}
		if iconName != "" {
			kids = append(kids, icon(iconName, 18, fg))
		}
		if label != "" {
			kids = append(kids, giode.Text(label, giode.Styles{FontSize: 15, FontWeight: giode.FontWeightSemiBold, Color: fg}))
		}
		return giode.Row(st, kids...)
	})
}
