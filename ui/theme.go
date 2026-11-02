package ui

import (
	"github.com/xmhrnx/giode"
)

// The palette: a deep indigo accent on soft neutral surfaces.
var (
	lightPalette = giode.Palette{
		Primary:    "#5B5BF0",
		Secondary:  "#EEF0F8",
		Background: "#F5F6FB",
		Surface:    "#FFFFFF",
		Text:       "#0E1222",
		Border:     "#E4E7F1",
		OnPrimary:  "#FFFFFF",
		Muted:      "#6B7286",
		Accent:     "#8B5CF6",
		Success:    "#12B981",
		Warning:    "#F59E0B",
		Danger:     "#EF4444",
		Info:       "#3B82F6",
	}
	darkPalette = giode.Palette{
		Primary:    "#7B7BFF",
		Secondary:  "#1C2031",
		Background: "#0A0C14",
		Surface:    "#141724",
		Text:       "#F2F3F9",
		Border:     "#262A3C",
		OnPrimary:  "#FFFFFF",
		Muted:      "#8E95AB",
		Accent:     "#A78BFA",
		Success:    "#34D399",
		Warning:    "#FBBF24",
		Danger:     "#F87171",
		Info:       "#60A5FA",
	}
)

// Colors used by name in the views.
const (
	cBg      = "theme.background"
	cSurface = "theme.surface"
	cText    = "theme.text"
	cMuted   = "theme.muted"
	cBorder  = "theme.border"
	cPrimary = "theme.primary"
	cOnPrim  = "theme.onprimary"
	cFill    = "theme.secondary" // subtle fills: chips, inputs, tiles
	cDanger  = "theme.danger"
	cSuccess = "theme.success"

	// The editor screens are always dark, like a camera's.
	cStudio      = "#07080D"
	cStudioPanel = "#12141E"
	cStudioLine  = "#262938"
	cStudioText  = "#F4F5FA"
	cStudioMuted = "#9AA0B4"
	cAccent      = "#7B7BFF"
)

// Tints that follow the theme.
var (
	cPrimarySoft = giode.Mix("theme.primary", "theme.surface", 0.12)
	cDangerSoft  = giode.Mix("theme.danger", "theme.surface", 0.12)
	cPressed     = giode.Mix("theme.text", "theme.surface", 0.06)
)

// brandGradient fills the main call to action.
var brandGradient = giode.Linear(135, "#6366F1", "#8B5CF6", "#A855F7")

// Sizes.
const (
	radiusCard  = 20
	radiusTile  = 14
	radiusPill  = 999
	screenInset = 20
)

var (
	shadowCard = giode.BoxShadow{Y: 6, Blur: 24, Color: "#1B1F4414"}
	shadowFab  = giode.BoxShadow{Y: 10, Blur: 28, Color: "#5B5BF066"}
)

// Text styles.
var (
	tDisplay = giode.Styles{FontSize: 30, FontWeight: giode.FontWeightExtraBold, Color: cText}
	tTitle   = giode.Styles{FontSize: 20, FontWeight: giode.FontWeightBold, Color: cText}
	tHeading = giode.Styles{FontSize: 16, FontWeight: giode.FontWeightSemiBold, Color: cText}
	tLabel   = giode.Styles{FontSize: 14, FontWeight: giode.FontWeightMedium, Color: cText}
	tCaption = giode.Styles{FontSize: 13, Color: cMuted}
	tTiny    = giode.Styles{FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: cMuted}
)

// with returns base with override's set fields on top.
func with(base giode.Styles, override giode.Styles) giode.Styles { return base.With(override) }

// applyTheme switches between light and dark.
func applyTheme(dark bool) {
	if dark {
		giode.SetMode(giode.ModeDark)
	} else {
		giode.SetMode(giode.ModeLight)
	}
}

// setupPalettes installs the app's palettes.
func setupPalettes() {
	giode.SetPalette(giode.ModeLight, lightPalette)
	giode.SetPalette(giode.ModeDark, darkPalette)
}
