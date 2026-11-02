package ui

import (
	"gioui.org/layout"
	"github.com/xmhrnx/giode"
)

// fillSpace lays el out at the full size it's offered: a zoomable view
// fills its area rather than shrinking to its content.
func fillSpace(el giode.Element) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return el.Layout(gtx)
	})
}
