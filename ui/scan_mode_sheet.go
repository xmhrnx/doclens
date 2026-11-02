package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/sheet"

	"github.com/mehran/scanly/internal/library"
	"github.com/mehran/scanly/internal/pdfpages"
)

// scanModeSheet asks how to scan before opening the camera.
type scanModeSheet struct {
	a     *App
	shown bool
	sh    *sheet.Sheet
	doc   *library.Document // add pages to this document; nil for a new one
}

func newScanModeSheet(a *App) *scanModeSheet {
	m := &scanModeSheet{a: a}
	m.sh = giode.Sheet(&m.shown, m.content, giode.Styles{Background: cSurface}).Title("New scan").Adaptive(true)
	return m
}

func (m *scanModeSheet) show() { m.showFor(nil) }

func (m *scanModeSheet) showFor(d *library.Document) {
	m.doc = d
	title := "New scan"
	if d != nil {
		title = "Add pages"
	}
	m.sh.Title(title)
	m.shown = true
}

func (m *scanModeSheet) view() giode.Element { return m.sh }

func (m *scanModeSheet) content() giode.Element {
	mode := func(key, iconName, tint, title, sub string, fn func()) giode.Element {
		return tappable("scanMode", key, func() { m.shown = false; fn() }, func(s giode.PressState) giode.Element {
			bg := cFill
			if s.Pressed {
				bg = cPressed
			}
			return giode.Row(giode.Styles{
				Width: -1, Align: giode.AlignCenter, Gap: 14, Wrap: giode.WrapNoWrap, Padding: giode.UniformInset(14),
				Background: bg, BorderRadius: 18, Transform: pressScale(s),
			},
				giode.Box(giode.Styles{Width: 46, Height: 46, BorderRadius: 14, Background: giode.Mix(tint, "theme.surface", 0.16),
					Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icon(iconName, 22, tint)),
				giode.Stack(giode.Styles{FlexGrow: 1, Gap: 3},
					giode.Text(title, tHeading),
					giode.Text(sub, with(tCaption, giode.Styles{LineHeight: 18})),
				),
				icon("chevron-right", 18, cMuted),
			)
		})
	}
	d := m.doc
	kids := []giode.Element{
		mode("single", "scan", "#6366F1", "Document", "One page. Edges are found and the page is flattened.", func() { startScan(m.a, scanSingle, d) }),
		mode("batch", "files", "#8B5CF6", "Batch", "Shoot page after page; close the camera when done.", func() { startScan(m.a, scanBatch, d) }),
	}
	if d == nil {
		kids = append(kids,
			mode("id", "id-card", "#0EA5E9", "ID card", "Front and back on one sheet, at real size.", func() { startScan(m.a, scanIDCard, nil) }))
	}
	kids = append(kids,
		mode("book", "book-open", "#F59E0B", "Book", "A two-page spread, split into two pages.", func() { startScan(m.a, scanBook, d) }),
		mode("import", "images", "#10B981", "Import pictures", "Turn photos you already have into scans.", func() { startImport(m.a, d) }),
	)
	if pdfpages.Supported() {
		kids = append(kids,
			mode("pdf", "file-input", "#F43F5E", "Import PDF", "Bring in a PDF's pages to edit, sign and re-export.", func() { startPDFImport(m.a, d) }))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 10, Padding: giode.Inset{Bottom: 16}}, kids...)
}
