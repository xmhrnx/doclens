package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"
)

// brandMark is the app icon at a square size.
func brandMark(a *App, size int) giode.Element {
	return giode.Image(a.logo, giode.Styles{Width: properties.Width(size), Height: properties.Height(size), Fit: giode.FitContain})
}
