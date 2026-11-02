package ui

import (
	"github.com/xmhrnx/giode"
)

// licensesScreen lists the open-source software Scanly is built with.
type licensesScreen struct {
	a    *App
	list giode.Element
}

func newLicensesScreen(a *App) *licensesScreen {
	return &licensesScreen{a: a, list: giode.LicensesView(a.licenses)}
}

func (s *licensesScreen) view() giode.Element {
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		topBar(iconTile("licBack", "chevron-left", false, s.a.pop), "Open-source licenses", false),
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1, Padding: giode.SymmetricInset(16, 0)}, s.list),
	)
}
