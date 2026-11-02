package ui

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/giode/components/scroll"

	"github.com/xmhrnx/doclens/internal/library"
)

// Folder filters on the home screen besides real folders.
const (
	filterAll       = ""
	filterFavorites = "*favorites"
)

// homeScreen lists the documents, with search, folders and the scan
// buttons.
type homeScreen struct {
	a       *App
	search  *input.Input
	scroll  *scroll.Scroll
	chips   *scroll.Scroll
	folder  string
	actions *docActions
	prompt  *textPrompt
	modes   *scanModeSheet
}

func newHomeScreen(a *App) *homeScreen {
	h := &homeScreen{a: a}
	h.search = giode.Input.Text("Search documents", giode.Styles{
		Background: "transparent", BorderColor: "transparent", Color: cText, FontSize: 15,
		Padding: giode.SymmetricInset(4, 12),
	})
	h.scroll = giode.Scroll(h.content, giode.Styles{ScrollBar: giode.ScrollBar{Hidden: true}})
	h.chips = giode.Scroll(h.chipRow, giode.Styles{ScrollBar: giode.ScrollBar{Hidden: true}}).Axis(giode.Horizontal)
	h.actions = newDocActions(a)
	h.prompt = newTextPrompt()
	h.modes = newScanModeSheet(a)
	return h
}

func (h *homeScreen) view() giode.Element {
	return giode.Box(giode.Styles{Width: -1, Height: -1, FlexDirection: giode.FlexDirectionColumn},
		h.scroll,
		h.scanBar(),
		h.actions.view(),
		h.prompt.view(),
		h.modes.view(),
	)
}

func (h *homeScreen) content() giode.Element {
	docs := h.visibleDocs()
	kids := []giode.Element{
		h.header(),
		h.searchField(),
		h.toolsRow(),
		h.chips,
		h.sectionTitle(docs),
	}
	if len(docs) == 0 {
		kids = append(kids, h.emptyState())
	}
	for _, d := range docs {
		kids = append(kids, h.docRow(d))
	}
	// Room for the scan bar.
	kids = append(kids, giode.Box(giode.Styles{Height: 120}))
	return giode.Stack(giode.Styles{Width: -1, Gap: 12, Padding: giode.Inset{Left: screenInset, Right: screenInset, Top: 8}}, kids...)
}

func (h *homeScreen) visibleDocs() []*library.Document {
	q := h.search.Text()
	var out []*library.Document
	for _, d := range h.a.store.Docs {
		switch {
		case h.folder == filterFavorites && !d.Favorite:
			continue
		case h.folder != filterAll && h.folder != filterFavorites && d.FolderID != h.folder:
			continue
		}
		if d.Matches(q) {
			out = append(out, d)
		}
	}
	return out
}

func (h *homeScreen) header() giode.Element {
	n := len(h.a.store.Docs)
	sub := "Your pocket scanner"
	if n > 0 {
		sub = fmt.Sprintf("%d document%s · %d pages", n, plural(n), h.totalPages())
	}
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Top: 12, Bottom: 4}},
		giode.Box(giode.Styles{Width: 46, Height: 46, BorderRadius: 12, BoxShadow: shadowFab, Margin: giode.Inset{Right: 12}},
			brandMark(h.a, 46)),
		giode.Stack(giode.Styles{FlexGrow: 1, Gap: 2},
			giode.Text("DocLens", with(tDisplay, giode.Styles{FontSize: 26})),
			giode.Text(sub, tCaption),
		),
		iconTile("settings", "settings", false, func() { h.a.push(newSettingsScreen(h.a)) }),
	)
}

func (h *homeScreen) totalPages() int {
	n := 0
	for _, d := range h.a.store.Docs {
		n += len(d.Pages)
	}
	return n
}

func (h *homeScreen) searchField() giode.Element {
	return giode.Row(giode.Styles{
		Width: -1, Align: giode.AlignCenter, Gap: 6, Wrap: giode.WrapNoWrap,
		Background: cSurface, BorderRadius: radiusPill, BorderWidth: 1, BorderColor: cBorder,
		Padding: giode.Inset{Left: 16, Right: 8},
	},
		icon("search", 18, cMuted),
		giode.Box(giode.Styles{FlexGrow: 1}, h.search),
	)
}

// toolsRow offers the scan modes besides the plain scan button.
func (h *homeScreen) toolsRow() giode.Element {
	tool := func(key, iconName, label, tint string, onClick func()) giode.Element {
		return growTappable("homeTool", key, onClick, func(s giode.PressState) giode.Element {
			return giode.Stack(giode.Styles{
				Width: -1, Align: giode.AlignCenter, Gap: 8, Padding: giode.SymmetricInset(4, 14),
				Background: cSurface, BorderRadius: radiusTile, BorderWidth: 1, BorderColor: cBorder,
				Transform: pressScale(s),
			},
				giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 12, Background: giode.Mix(tint, "theme.surface", 0.14),
					Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icon(iconName, 20, tint)),
				giode.Text(label, giode.Styles{FontSize: 12, FontWeight: giode.FontWeightSemiBold, Color: cText}),
			)
		})
	}
	return giode.Row(giode.Styles{Width: -1, Gap: 10, Wrap: giode.WrapNoWrap},
		tool("batch", "files", "Batch", "#6366F1", func() { startScan(h.a, scanBatch, nil) }),
		tool("id", "id-card", "ID Card", "#0EA5E9", func() { startScan(h.a, scanIDCard, nil) }),
		tool("book", "book-open", "Book", "#F59E0B", func() { startScan(h.a, scanBook, nil) }),
		tool("import", "images", "Import", "#10B981", func() { startImport(h.a, nil) }),
	)
}

// chipRow is the row of folder filters.
func (h *homeScreen) chipRow() giode.Element {
	chip := func(key, label, color string, selected bool, onClick func(), onLong func()) giode.Element {
		content := func(s giode.PressState) giode.Element {
			bg, fg, border := cSurface, cText, cBorder
			if selected {
				bg, fg, border = cText, cBg, cText
			}
			kids := []giode.Element{}
			if color != "" {
				kids = append(kids, giode.Box(giode.Styles{Width: 8, Height: 8, BorderRadius: 4, Background: color}))
			}
			kids = append(kids, giode.Text(label, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: fg}))
			return giode.Row(giode.Styles{
				Align: giode.AlignCenter, Gap: 6, Padding: giode.SymmetricInset(14, 9), Wrap: giode.WrapNoWrap,
				Background: bg, BorderRadius: radiusPill, BorderWidth: 1, BorderColor: border, Transform: pressScale(s),
			}, kids...)
		}
		if onLong != nil {
			return longTappable("folderChip", key, onClick, onLong, content)
		}
		return tappable("folderChip", key, onClick, content)
	}
	sel := func(id string) func() { return func() { h.folder = id } }
	chips := []giode.Element{
		chip("all", "All", "", h.folder == filterAll, sel(filterAll), nil),
		chip("fav", "Favorites", "#F59E0B", h.folder == filterFavorites, sel(filterFavorites), nil),
	}
	for _, f := range h.a.store.Folders {
		f := f
		chips = append(chips, chip(f.ID, f.Name, f.Color, h.folder == f.ID, sel(f.ID), func() { h.folderMenu(f) }))
	}
	chips = append(chips, chip("new", "+ Folder", "", false, h.newFolder, nil))
	return giode.Row(giode.Styles{Gap: 8, Wrap: giode.WrapNoWrap}, chips...)
}

func (h *homeScreen) newFolder() {
	h.prompt.open("New folder", "Folder name", "", "Create", func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		f := h.a.store.NewFolder(name, folderColors[len(h.a.store.Folders)%len(folderColors)])
		h.folder = f.ID
		h.a.save()
	})
}

func (h *homeScreen) folderMenu(f *library.Folder) {
	h.actions.openFolder(f, func() {
		if h.folder == f.ID {
			h.folder = filterAll
		}
	}, h.prompt)
}

// folderColors tag folders in turn.
var folderColors = []string{"#6366F1", "#10B981", "#F43F5E", "#0EA5E9", "#F59E0B", "#8B5CF6", "#14B8A6"}

func (h *homeScreen) sectionTitle(docs []*library.Document) giode.Element {
	title := "Recent"
	switch {
	case h.search.Text() != "":
		title = fmt.Sprintf("%d result%s", len(docs), plural(len(docs)))
	case h.folder == filterFavorites:
		title = "Favorites"
	case h.folder != filterAll:
		if f := h.a.store.Folder(h.folder); f != nil {
			title = f.Name
		}
	}
	return giode.Text(title, with(tHeading, giode.Styles{Margin: giode.Inset{Top: 6}}))
}

func (h *homeScreen) emptyState() giode.Element {
	msg, sub := "No documents yet", "Tap Scan to capture your first page. DocLens finds the edges, flattens the page and cleans it up."
	if h.search.Text() != "" || h.folder != filterAll {
		msg, sub = "Nothing here", "No documents match. Try another search or folder."
	}
	return giode.Stack(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 12, Padding: giode.SymmetricInset(24, 36),
		Background: cSurface, BorderRadius: radiusCard, BorderWidth: 1, BorderColor: cBorder},
		giode.Box(giode.Styles{Width: 72, Height: 72, BorderRadius: 24, Background: cPrimarySoft,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter}, icon("file-text", 34, cPrimary)),
		giode.Text(msg, tTitle),
		giode.Text(sub, with(tCaption, giode.Styles{TextAlign: giode.TextAlignCenter, LineHeight: 20})),
	)
}

func (h *homeScreen) docRow(d *library.Document) giode.Element {
	// A search that matched text inside a page opens that page.
	hit := -1
	if q := h.search.Text(); q != "" && !strings.Contains(strings.ToLower(d.Name), strings.ToLower(strings.TrimSpace(q))) {
		hit = d.MatchingPage(q)
	}
	open := func() {
		h.a.push(newDocumentScreen(h.a, d))
		if hit >= 0 {
			h.a.push(newPageScreen(h.a, d, hit))
		}
	}
	more := func() { h.actions.openDoc(d, h.prompt, nil) }
	return longTappable("docRow", d.ID, open, more, func(s giode.PressState) giode.Element {
		meta := fmt.Sprintf("%s · %d page%s", relativeDate(d.Modified), len(d.Pages), plural(len(d.Pages)))
		if hit >= 0 {
			meta = fmt.Sprintf("Text found on page %d", hit+1)
		}
		badges := []giode.Element{giode.Text(meta, tCaption)}
		if f := h.a.store.Folder(d.FolderID); f != nil {
			badges = append(badges, giode.Row(giode.Styles{Align: giode.AlignCenter, Gap: 4, Padding: giode.SymmetricInset(8, 3),
				BorderRadius: radiusPill, Background: giode.Mix(f.Color, "theme.surface", 0.14)},
				giode.Text(f.Name, giode.Styles{FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: f.Color})))
		}
		bg := cSurface
		if s.Pressed {
			bg = cPressed
		}
		star := giode.Element(giode.Box(giode.Styles{}))
		if d.Favorite {
			star = icon("star", 18, "#F59E0B")
		}
		return giode.Row(giode.Styles{
			Width: -1, Align: giode.AlignCenter, Gap: 14, Wrap: giode.WrapNoWrap, Padding: giode.UniformInset(10),
			Background: bg, BorderRadius: radiusCard, BorderWidth: 1, BorderColor: cBorder, BoxShadow: shadowCard,
		},
			docThumb(h.a, d, 64, 84),
			giode.Stack(giode.Styles{FlexGrow: 1, Gap: 6},
				giode.Text(d.Name, with(tHeading, giode.Styles{TextOverflow: giode.TextOverflowEllipsis})),
				giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter}, badges...),
			),
			star,
			icon("chevron-right", 18, cMuted),
		)
	})
}

// scanBar holds the main Scan button, floating over the list.
func (h *homeScreen) scanBar() giode.Element {
	fab := tappable("home", "scan", func() { h.modes.show() }, func(s giode.PressState) giode.Element {
		return giode.Row(giode.Styles{
			Align: giode.AlignCenter, Gap: 10, Padding: giode.SymmetricInset(30, 18),
			BorderRadius: radiusPill, BackgroundGradient: brandGradient, BoxShadow: shadowFab, Transform: pressScale(s),
		},
			icon("camera", 22, "#FFFFFF"),
			giode.Text("Scan", giode.Styles{FontSize: 17, FontWeight: giode.FontWeightBold, Color: "#FFFFFF"}),
		)
	})
	return giode.Box(giode.Styles{
		Position: giode.PositionAbsolute, Left: giode.Flush, Right: giode.Flush, Bottom: 24,
		Justify: giode.JustifyCenter, ZIndex: 5,
	}, fab)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
