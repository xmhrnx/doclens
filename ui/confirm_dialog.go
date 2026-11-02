package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/dialog"
)

// confirmDialog asks the user to confirm an action.
type confirmDialog struct {
	shown   bool
	title   string
	message string
	action  string
	danger  bool
	onYes   func()
	dlg     *dialog.Dialog
}

func newConfirmDialog() *confirmDialog {
	c := &confirmDialog{}
	c.dlg = giode.Dialog(&c.shown, c.content, giode.Styles{Background: cSurface, BorderRadius: 24, Width: 340, Padding: giode.UniformInset(22)})
	return c
}

func (c *confirmDialog) open(title, message, action string, danger bool, onYes func()) {
	c.title, c.message, c.action, c.danger, c.onYes = title, message, action, danger, onYes
	c.shown = true
}

func (c *confirmDialog) content() giode.Element {
	kind := buttonPrimary
	if c.danger {
		kind = buttonDanger
	}
	return giode.Stack(giode.Styles{Gap: 14, Width: -1},
		giode.Text(c.title, tTitle),
		giode.Text(c.message, with(tCaption, giode.Styles{FontSize: 15, LineHeight: 21})),
		giode.Row(giode.Styles{Gap: 10, Justify: giode.JustifyEnd, Width: -1, Margin: giode.Inset{Top: 6}},
			actionButton("confirmNo", buttonGhost, "", "Cancel", false, func() { c.shown = false }),
			actionButton("confirmYes", kind, "", c.action, false, func() {
				c.shown = false
				if c.onYes != nil {
					giode.AfterLayout(c.onYes)
				}
			}),
		),
	)
}

func (c *confirmDialog) view() giode.Element { return c.dlg }
