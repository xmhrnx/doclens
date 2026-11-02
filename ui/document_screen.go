package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/scroll"
	"github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/doclens/internal/library"
)

// docMode is what tapping a page does on the document screen.
type docMode int

const (
	modeView    docMode = iota
	modeSelect          // tap toggles selection
	modeReorder         // arrows move pages
)

// documentScreen shows a document's pages in a grid, with tools to
// add, reorder, select, share and export.
type documentScreen struct {
	a        *App
	doc      *library.Document
	scroll   *scroll.Scroll
	mode     docMode
	selected map[string]bool
	actions  *docActions
	prompt   *textPrompt
	modes    *scanModeSheet
	export   *exportSheet
	merge    *mergeSheet
}

func newDocumentScreen(a *App, d *library.Document) *documentScreen {
	s := &documentScreen{a: a, doc: d, selected: map[string]bool{}}
	s.scroll = giode.Scroll(s.content, giode.Styles{ScrollBar: giode.ScrollBar{Hidden: true}})
	s.actions = newDocActions(a)
	s.prompt = newTextPrompt()
	s.modes = newScanModeSheet(a)
	s.export = newExportSheet(a)
	s.merge = newMergeSheet(a)
	return s
}

func (s *documentScreen) view() giode.Element {
	if s.mode != modeView {
		giode.HandleBack(func() { s.setMode(modeView) })
	}
	return giode.Stack(giode.Styles{Width: -1, Height: -1},
		s.header(),
		giode.Box(giode.Styles{FlexGrow: 1, Width: -1}, s.scroll),
		s.bottomBar(),
		s.actions.view(), s.prompt.view(), s.modes.view(), s.export.view(), s.merge.view(),
	)
}

func (s *documentScreen) setMode(m docMode) {
	s.mode = m
	s.selected = map[string]bool{}
}

func (s *documentScreen) header() giode.Element {
	d := s.doc
	if s.mode == modeSelect {
		return topBar(iconTile("docDone", "x", false, func() { s.setMode(modeView) }),
			fmt.Sprintf("%d selected", len(s.selected)), false,
			actionButton("docAll", buttonGhost, "", "All", false, func() {
				for _, p := range d.Pages {
					s.selected[p.ID] = true
				}
			}))
	}
	if s.mode == modeReorder {
		return topBar(nil, "Reorder pages", false,
			actionButton("docReorderDone", buttonPrimary, "check", "Done", false, func() { s.setMode(modeView); s.a.save() }))
	}
	return topBar(iconTile("docBack", "chevron-left", false, s.a.pop), "", false,
		favTile("docFav", d.Favorite, func() {
			d.Favorite = !d.Favorite
			s.a.save()
		}),
		iconTile("docMore", "ellipsis-vertical", false, func() {
			s.actions.openDoc(d, s.prompt, func() { s.a.popTo(s.a.home) })
		}),
	)
}

func (s *documentScreen) content() giode.Element {
	d := s.doc
	meta := fmt.Sprintf("%d page%s · %s", len(d.Pages), plural(len(d.Pages)), relativeDate(d.Modified))
	if d.Layout == library.LayoutIDCard {
		meta += " · ID card layout"
	}
	if f := s.a.store.Folder(d.FolderID); f != nil {
		meta += " · " + f.Name
	}
	title := tappable("docTitle", d.ID, s.rename, func(st giode.PressState) giode.Element {
		return giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(d.Name, with(tDisplay, giode.Styles{FontSize: 24, MaxLines: 2}))),
			icon("pencil", 16, cMuted))
	})
	kids := []giode.Element{
		giode.Stack(giode.Styles{Gap: 4, Width: -1}, title, giode.Text(meta, tCaption)),
		s.grid(),
		giode.Box(giode.Styles{Height: 24}),
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 18, Padding: giode.Inset{Left: screenInset, Right: screenInset, Top: 4}}, kids...)
}

func (s *documentScreen) rename() {
	d := s.doc
	s.prompt.open("Rename document", "Name", d.Name, "Save", func(name string) {
		if name = strings.TrimSpace(name); name != "" {
			d.Name = name
			s.a.save()
		}
	})
}

// grid lays the pages out two (or more, on wide screens) per row.
func (s *documentScreen) grid() giode.Element {
	d := s.doc
	if len(d.Pages) == 0 {
		return giode.Stack(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 10, Padding: giode.UniformInset(32)},
			icon("file-stack", 40, cMuted),
			giode.Text("No pages yet. Add some with the camera.", tCaption))
	}
	cols := 2
	if !giode.Compact() {
		cols = 4
	}
	var rows []giode.Element
	for i := 0; i < len(d.Pages); i += cols {
		var cells []giode.Element
		for j := i; j < i+cols; j++ {
			if j < len(d.Pages) {
				cells = append(cells, s.pageCard(j))
			} else {
				cells = append(cells, giode.Box(giode.Styles{FlexGrow: 1}))
			}
		}
		rows = append(rows, giode.Row(giode.Styles{Width: -1, Gap: 14, Wrap: giode.WrapNoWrap}, cells...))
	}
	return giode.Stack(giode.Styles{Width: -1, Gap: 14}, rows...)
}

func (s *documentScreen) pageCard(i int) giode.Element {
	d := s.doc
	p := d.Pages[i]
	onClick := func() {
		switch s.mode {
		case modeSelect:
			s.toggle(p.ID)
		case modeView:
			s.a.push(newPageScreen(s.a, d, i))
		}
	}
	onLong := func() {
		if s.mode == modeView {
			s.mode = modeSelect
			s.selected = map[string]bool{p.ID: true}
			giode.Haptic(giode.HapticMedium)
		}
	}
	return longTappableStyled("pageCard", p.ID, giode.Styles{FlexGrow: 1}, onClick, onLong, func(st giode.PressState) giode.Element {
		img := s.a.thumbs.get(d.ID, p)
		var thumb giode.Element = giode.Box(giode.Styles{Width: -1, Height: -1, Align: giode.AlignCenter, Justify: giode.JustifyCenter},
			giode.Spinner(giode.Styles{Width: 22, Height: 22, Color: cMuted}))
		if img != nil {
			thumb = giode.Image(img, giode.Styles{Width: -1, Height: -1, Fit: giode.FitContain})
		}
		sel := s.selected[p.ID]
		border, bw := cBorder, 1
		if sel {
			border, bw = cPrimary, 3
		}
		overlay := []giode.Element{
			giode.Box(giode.Styles{Position: giode.PositionAbsolute, Left: 8, Bottom: 8, ZIndex: 2,
				Background: "#0E1222CC", BorderRadius: radiusPill, Padding: giode.SymmetricInset(9, 3)},
				giode.Text(fmt.Sprint(i+1), giode.Styles{FontSize: 12, FontWeight: giode.FontWeightBold, Color: "#FFFFFF"})),
		}
		if s.mode == modeSelect {
			mark := giode.Styles{Width: 24, Height: 24, BorderRadius: 12, BorderWidth: 2, BorderColor: "#FFFFFF",
				Background: "#0E122255", Align: giode.AlignCenter, Justify: giode.JustifyCenter}
			var inner giode.Element = giode.Box(giode.Styles{})
			if sel {
				mark.Background, mark.BorderColor = cPrimary, cPrimary
				inner = icon("check", 14, "#FFFFFF")
			}
			overlay = append(overlay, giode.Box(with(mark, giode.Styles{Position: giode.PositionAbsolute, Top: 8, Right: 8, ZIndex: 2}), inner))
		}
		if s.mode == modeReorder {
			overlay = append(overlay, giode.Row(giode.Styles{Position: giode.PositionAbsolute, Top: 8, Right: 8, ZIndex: 2, Gap: 6},
				s.moveButton(i, -1), s.moveButton(i, 1)))
		}
		return giode.Box(giode.Styles{
			FlexGrow: 1, AspectRatio: 0.75, Background: cSurface, BorderRadius: radiusTile, BorderWidth: properties.BorderWidth(bw), BorderColor: border,
			Overflow: giode.OverflowHidden, Padding: giode.UniformInset(6), BoxShadow: shadowCard, Transform: pressScale(st),
		}, append([]giode.Element{thumb}, overlay...)...)
	})
}

func (s *documentScreen) moveButton(i, dir int) giode.Element {
	name := "arrow-left"
	if dir > 0 {
		name = "arrow-right"
	}
	return tappable("movePage", [2]int{i, dir}, func() {
		j := i + dir
		pages := s.doc.Pages
		if j < 0 || j >= len(pages) {
			return
		}
		giode.AfterLayout(func() { pages[i], pages[j] = pages[j], pages[i] })
	}, func(st giode.PressState) giode.Element {
		return giode.Box(giode.Styles{Width: 30, Height: 30, BorderRadius: 15, Background: cPrimary,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter, Transform: pressScale(st)}, icon(name, 16, "#FFFFFF"))
	})
}

func (s *documentScreen) toggle(id string) {
	if s.selected[id] {
		delete(s.selected, id)
	} else {
		s.selected[id] = true
	}
}

// bar is the floating tool bar at the bottom.
func (s *documentScreen) bottomBar() giode.Element {
	var kids []giode.Element
	tool := func(key, iconName, label string, fn func()) giode.Element {
		return growTappable("docTool", key, fn, func(st giode.PressState) giode.Element {
			return giode.Stack(giode.Styles{Align: giode.AlignCenter, Gap: 5, Width: -1, Padding: giode.SymmetricInset(4, 6), Transform: pressScale(st)},
				icon(iconName, 22, cText),
				giode.Text(label, giode.Styles{FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: cMuted}))
		})
	}
	switch s.mode {
	case modeSelect:
		kids = []giode.Element{
			tool("selFilter", "wand-sparkles", "Filter", func() { s.editSelected(false) }),
			tool("selCrop", "crop", "Crop", func() { s.editSelected(true) }),
			tool("selExtract", "file-down", "New doc", s.extractSelected),
			tool("selDelete", "trash-2", "Delete", s.deleteSelected),
		}
	case modeReorder:
		return giode.Box(giode.Styles{})
	default:
		kids = []giode.Element{
			tool("add", "plus", "Add", func() { s.modes.showFor(s.doc) }),
			tool("reorder", "arrow-up-down", "Reorder", func() { s.setMode(modeReorder) }),
			tool("select", "list-checks", "Select", func() { s.setMode(modeSelect) }),
			tool("merge", "layers", "Merge", func() { s.merge.open(s.doc) }),
			tool("exportDoc", "file-down", "Export", func() { s.export.open(s.doc) }),
		}
	}
	share := tappable("docShare", "share", func() { s.export.quickShare(s.doc) }, func(st giode.PressState) giode.Element {
		return giode.Box(giode.Styles{Width: 54, Height: 54, BorderRadius: 27, BackgroundGradient: brandGradient, BoxShadow: shadowFab,
			Align: giode.AlignCenter, Justify: giode.JustifyCenter, Transform: pressScale(st)}, icon("share-2", 22, "#FFFFFF"))
	})
	bar := giode.Row(giode.Styles{
		FlexGrow: 1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Gap: 2, Padding: giode.SymmetricInset(6, 8),
		Background: cSurface, BorderRadius: 26, BorderWidth: 1, BorderColor: cBorder, BoxShadow: shadowCard,
	}, kids...)
	row := []giode.Element{bar}
	if s.mode == modeView {
		row = append(row, share)
	}
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap,
		Padding: giode.Inset{Left: 14, Right: 14, Bottom: 14, Top: 6}}, row...)
}

func (s *documentScreen) selectedPages() []*library.Page {
	var out []*library.Page
	for _, p := range s.doc.Pages {
		if s.selected[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

func (s *documentScreen) editSelected(crop bool) {
	pages := s.selectedPages()
	if len(pages) == 0 {
		return
	}
	s.setMode(modeView)
	startEdit(s.a, s.doc, pages, crop)
}

// extractSelected copies the selected pages into a new document.
func (s *documentScreen) extractSelected() {
	pages := s.selectedPages()
	if len(pages) == 0 {
		return
	}
	src := s.doc
	a := s.a
	s.setMode(modeView)
	clones := clonePages(pages)
	var id string
	var copies []*library.Page
	a.work("Creating document", func(func(float64)) error {
		var err error
		id, copies, err = a.store.CopyPages(src.ID, clones)
		return err
	}, func() {
		nd := a.store.AddDocument(id, src.Name+" (extract)", src.FolderID, copies)
		a.save()
		a.notify(fmt.Sprintf("Created “%s” with %d page%s", nd.Name, len(copies), plural(len(copies))))
		a.push(newDocumentScreen(a, nd))
	})
}

func clonePages(pages []*library.Page) []*library.Page {
	out := make([]*library.Page, len(pages))
	for i, p := range pages {
		out[i] = p.Clone()
	}
	return out
}

func (s *documentScreen) deleteSelected() {
	d := s.doc
	ids := s.selected
	if len(ids) == 0 {
		return
	}
	giode.AfterLayout(func() {
		before := slices.Clone(d.Pages)
		d.Pages = slices.DeleteFunc(d.Pages, func(p *library.Page) bool { return ids[p.ID] })
		n := len(before) - len(d.Pages)
		s.setMode(modeView)
		s.a.save()
		s.a.toasts.ShowAction(giode.ToastInfo, fmt.Sprintf("Deleted %d page%s", n, plural(n)), "Undo", func() {
			d.Pages = before
			s.a.save()
		})
	})
}
