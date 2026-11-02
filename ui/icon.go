package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"
)

// icon renders a named icon from assets/icons at a square size.
func icon(name string, size int, color string) giode.Element {
	return giode.Icon(name, giode.Styles{Width: properties.Width(size), Height: properties.Height(size), Color: properties.Color(color)})
}
