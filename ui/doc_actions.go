package ui

import (
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/sheet"
	"github.com/xmhrnx/giode/printing"

	"github.com/xmhrnx/doclens/internal/library"
)

// docActions is the bottom sheet of things to do with a document (or a
// folder): rename, share, move, favorite, delete.
type docActions struct {
	a      *App
	shown  bool
	sh     *sheet.Sheet
	doc    *library.Document
	folder *library.Folder
	moving bool
	prompt *textPrompt
	export *exportSheet
	// onDeleted runs after the document or folder is removed.
	onDeleted func()
}

func newDocActions(a *App) *docActions {
	d := &docActions{a: a, export: newExportSheet(a)}
	d.sh = giode.Sheet(&d.shown, d.content, giode.Styles{Background: cSurface}).Adaptive(true)
	return d
}

func (d *docActions) openDoc(doc *library.Document, prompt *textPrompt, onDeleted func()) {
	d.doc, d.folder, d.moving, d.prompt, d.onDeleted = doc, nil, false, prompt, onDeleted
	d.sh.Title(doc.Name)
	d.shown = true
}

func (d *docActions) openFolder(f *library.Folder, onDeleted func(), prompt *textPrompt) {
	d.doc, d.folder, d.moving, d.prompt, d.onDeleted = nil, f, false, prompt, onDeleted
	d.sh.Title(f.Name)
	d.shown = true
}

func (d *docActions) view() giode.Element {
	return giode.Box(giode.Styles{}, d.sh, d.export.view())
}

func (d *docActions) content() giode.Element {
	switch {
	case d.folder != nil:
		return d.folderContent()
	case d.doc == nil:
		return giode.Box(giode.Styles{})
	case d.moving:
		return d.moveContent()
	}
	doc := d.doc
	fav := "Add to favorites"
	if doc.Favorite {
		fav = "Remove from favorites"
	}
	row := func(key, iconName, label string, danger bool, fn func()) giode.Element {
		return sheetRow("docAction", key, iconName, label, "", danger, fn)
	}
	return giode.Stack(giode.Styles{Gap: 2, Width: -1, Padding: giode.Inset{Bottom: 12}},
		row("share", "share-2", "Share PDF", false, func() { d.shown = false; d.export.quickShare(doc) }),
		row("export", "file-down", "Export…", false, func() { d.shown = false; d.export.open(doc) }),
		d.printRow(doc),
		row("rename", "pencil", "Rename", false, func() {
			d.shown = false
			d.prompt.open("Rename document", "Name", doc.Name, "Save", func(name string) {
				if name = strings.TrimSpace(name); name != "" {
					doc.Name = name
					d.a.save()
				}
			})
		}),
		row("move", "folder-input", "Move to folder", false, func() { d.moving = true; d.sh.Title("Move to folder") }),
		row("fav", "star", fav, false, func() { doc.Favorite = !doc.Favorite; d.shown = false; d.a.save() }),
		row("delete", "trash-2", "Delete", true, func() { d.shown = false; d.deleteDoc(doc) }),
	)
}

// deleteDoc removes a document with a few seconds to undo.
func (d *docActions) deleteDoc(doc *library.Document) {
	giode.AfterLayout(func() {
		removed, at := d.a.store.RemoveDocument(doc.ID)
		if removed == nil {
			return
		}
		d.a.save()
		if d.onDeleted != nil {
			d.onDeleted()
		}
		undone := false
		d.a.toasts.ShowAction(giode.ToastInfo, "Deleted “"+doc.Name+"”", "Undo", func() {
			undone = true
			d.a.store.RestoreDocument(removed, at)
			d.a.save()
		})
		go func() {
			// Files go once the undo window has passed.
			<-afterToast()
			d.a.g.Post(func() {
				if !undone && d.a.store.Doc(doc.ID) == nil {
					if err := d.a.store.PurgeDocument(doc.ID); err != nil {
						d.a.fail(err)
					}
				}
			})
		}()
	})
}

func (d *docActions) moveContent() giode.Element {
	doc := d.doc
	move := func(id string) func() {
		return func() {
			doc.FolderID = id
			d.shown = false
			d.a.save()
		}
	}
	rows := []giode.Element{sheetRow("move", "none", "file-text", "No folder", check(doc.FolderID == ""), false, move(""))}
	for _, f := range d.a.store.Folders {
		rows = append(rows, sheetRow("move", f.ID, "folder", f.Name, check(doc.FolderID == f.ID), false, move(f.ID)))
	}
	rows = append(rows, sheetRow("move", "new", "folder-plus", "New folder…", "", false, func() {
		d.shown = false
		d.prompt.open("New folder", "Folder name", "", "Create", func(name string) {
			if name = strings.TrimSpace(name); name != "" {
				f := d.a.store.NewFolder(name, folderColors[len(d.a.store.Folders)%len(folderColors)])
				doc.FolderID = f.ID
				d.a.save()
			}
		})
	}))
	return giode.Stack(giode.Styles{Gap: 2, Width: -1, Padding: giode.Inset{Bottom: 12}}, rows...)
}

func check(on bool) string {
	if on {
		return "✓"
	}
	return ""
}

func (d *docActions) folderContent() giode.Element {
	f := d.folder
	return giode.Stack(giode.Styles{Gap: 2, Width: -1, Padding: giode.Inset{Bottom: 12}},
		sheetRow("folderAction", "rename", "pencil", "Rename folder", "", false, func() {
			d.shown = false
			d.prompt.open("Rename folder", "Name", f.Name, "Save", func(name string) {
				if name = strings.TrimSpace(name); name != "" {
					f.Name = name
					d.a.save()
				}
			})
		}),
		sheetRow("folderAction", "delete", "trash-2", "Delete folder", "", true, func() {
			d.shown = false
			giode.AfterLayout(func() {
				d.a.store.DeleteFolder(f.ID)
				d.a.save()
				if d.onDeleted != nil {
					d.onDeleted()
				}
				d.a.notify("Folder deleted; its documents are kept")
			})
		}),
	)
}

// printRow prints the document, where the platform can print.
func (d *docActions) printRow(doc *library.Document) giode.Element {
	if !printing.Supported() {
		return giode.Box(giode.Styles{Display: giode.DisplayNone})
	}
	return sheetRow("docAction", "print", "printer", "Print", "", false, func() { d.shown = false; d.export.quickPrint(doc) })
}
