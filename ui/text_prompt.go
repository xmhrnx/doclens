package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/dialog"
	"github.com/xmhrnx/giode/components/input"
)

// textPrompt is a dialog asking for one line of text.
type textPrompt struct {
	shown  bool
	title  string
	action string
	field  *input.Input
	dlg    *dialog.Dialog
	onSave func(string)
}

func newTextPrompt() *textPrompt {
	t := &textPrompt{}
	t.field = giode.Input.Text("", giode.Styles{
		Background: cFill, BorderColor: cFill, BorderRadius: radiusTile, Color: cText, FontSize: 16,
		Padding: giode.SymmetricInset(14, 14),
	}).Submit(func(string) { t.submit() })
	t.dlg = giode.Dialog(&t.shown, t.content, giode.Styles{Background: cSurface, BorderRadius: 24, Width: 340, Padding: giode.UniformInset(22)})
	return t
}

func (t *textPrompt) open(title, placeholder, value, action string, onSave func(string)) {
	t.title, t.action, t.onSave = title, action, onSave
	t.field.Placeholder(placeholder)
	t.field.Password(false)
	t.field.SetText(value)
	t.field.SelectAll()
	t.field.Focus()
	t.shown = true
}

func (t *textPrompt) submit() {
	t.shown = false
	if t.onSave != nil {
		t.onSave(t.field.Text())
	}
}

func (t *textPrompt) content() giode.Element {
	return giode.Stack(giode.Styles{Gap: 18, Width: -1},
		giode.Text(t.title, tTitle),
		t.field,
		giode.Row(giode.Styles{Gap: 10, Justify: giode.JustifyEnd, Width: -1},
			actionButton("promptCancel", buttonGhost, "", "Cancel", false, func() { t.shown = false }),
			actionButton("promptSave", buttonPrimary, "", t.action, false, t.submit),
		),
	)
}

func (t *textPrompt) view() giode.Element { return t.dlg }
