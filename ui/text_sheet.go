package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/sheet"
	"github.com/xmhrnx/giode/ocr"

	"github.com/xmhrnx/doclens/internal/library"
)

// textSheet shows a page's recognized text, to copy or share.
type textSheet struct {
	a     *App
	shown bool
	sh    *sheet.Sheet
	text  string
	key   string
}

func newTextSheet(a *App) *textSheet {
	t := &textSheet{a: a}
	t.sh = giode.Sheet(&t.shown, t.content, giode.Styles{Background: cSurface}).
		Title("Text on this page").Footer(t.footer).Adaptive(true).Detents(giode.DetentMedium, giode.DetentLarge)
	return t
}

// open reads the page's text if it isn't known yet, then shows it.
func (t *textSheet) open(d *library.Document, p *library.Page) {
	a := t.a
	if !ocr.Supported() {
		a.fail(fmt.Errorf("reading text isn't supported on this device"))
		return
	}
	if pt := p.CurrentText(); pt != nil {
		t.show(cacheKey(d.ID, p), pt.Text)
		return
	}
	clone := p.Clone()
	var pt *library.PageText
	a.work("Reading the text", func(func(float64)) error {
		img, err := a.store.RenderPage(d.ID, clone)
		if err != nil {
			return err
		}
		pt, err = readText(img, clone.Rev)
		return err
	}, func() {
		a.setText(d.ID, p.ID, pt)
		t.show(cacheKey(d.ID, p), pt.Text)
	})
}

func (t *textSheet) show(key, text string) {
	t.key, t.text = key, strings.TrimSpace(text)
	t.shown = true
}

func (t *textSheet) view() giode.Element { return t.sh }

func (t *textSheet) content() giode.Element {
	if t.text == "" {
		return giode.Stack(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 10, Padding: giode.UniformInset(24)},
			icon("text-select", 32, cMuted),
			giode.Text("No text found on this page.", tCaption))
	}
	return giode.Box(giode.Styles{Width: -1, Padding: giode.Inset{Bottom: 8}},
		giode.Text(t.text, giode.Styles{FontSize: 15, LineHeight: 23, Color: cText}).Selectable("pageText:"+t.key))
}

func (t *textSheet) footer() giode.Element {
	if t.text == "" {
		return giode.Box(giode.Styles{})
	}
	return giode.Row(giode.Styles{Width: -1, Gap: 10, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Top: 8, Bottom: 8}},
		actionButton("textCopy", buttonSoft, "copy", "Copy all", true, func() {
			t.a.g.SetClipboard(t.text)
			t.a.notify("Text copied")
		}),
		actionButton("textShare", buttonPrimary, "share-2", "Share", true, func() {
			text := t.text
			go func() {
				if err := t.a.g.Share(context.Background(), giode.ShareText{Text: text}); err != nil {
					t.a.fail(err)
				}
			}()
		}),
	)
}
