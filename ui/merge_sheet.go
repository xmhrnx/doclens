package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/sheet"

	"github.com/mehran/scanly/internal/library"
)

// mergeSheet appends another document's pages to this one.
type mergeSheet struct {
	a     *App
	shown bool
	sh    *sheet.Sheet
	doc   *library.Document
}

func newMergeSheet(a *App) *mergeSheet {
	m := &mergeSheet{a: a}
	m.sh = giode.Sheet(&m.shown, m.content, giode.Styles{Background: cSurface}).
		Title("Merge a document into this one").Adaptive(true).Detents(giode.DetentMedium, giode.DetentLarge)
	return m
}

func (m *mergeSheet) open(d *library.Document) {
	m.doc = d
	m.shown = true
}

func (m *mergeSheet) view() giode.Element { return m.sh }

func (m *mergeSheet) content() giode.Element {
	var rows []giode.Element
	for _, o := range m.a.store.Docs {
		if m.doc == nil || o.ID == m.doc.ID || len(o.Pages) == 0 {
			continue
		}
		o := o
		rows = append(rows, tappable("mergeRow", o.ID, func() { m.merge(o) }, func(s giode.PressState) giode.Element {
			bg := ""
			if s.Pressed {
				bg = cPressed
			}
			return giode.Row(giode.Styles{Width: -1, Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap,
				Padding: giode.UniformInset(8), BorderRadius: radiusTile, Background: bg},
				docThumb(m.a, o, 40, 52),
				giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
					giode.Text(o.Name, with(tLabel, giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
					giode.Text(fmt.Sprintf("%d page%s", len(o.Pages), plural(len(o.Pages))), tCaption)),
				icon("plus", 18, cPrimary))
		}))
	}
	if len(rows) == 0 {
		rows = append(rows, giode.Text("No other documents to merge.", tCaption))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 4, Padding: giode.Inset{Bottom: 12}}, rows...)
}

func (m *mergeSheet) merge(o *library.Document) {
	m.shown = false
	a, d := m.a, m.doc
	clones := clonePages(o.Pages)
	var copies []*library.Page
	a.work("Merging “"+o.Name+"”", func(func(float64)) error {
		var err error
		copies, err = a.store.CopyPagesInto(o.ID, d.ID, clones)
		return err
	}, func() {
		d.Pages = append(d.Pages, copies...)
		a.save()
		a.notify(fmt.Sprintf("Added %d page%s from “%s”", len(copies), plural(len(copies)), o.Name))
	})
}
