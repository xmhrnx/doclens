package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/doclens/internal/pdfpages"
)

// OnShare imports what another app shares with DocLens: pictures go
// through page detection and the review like imported photos, and each
// PDF becomes a document of its own.
func (a *App) OnShare(s giode.SharedContent) {
	var pictures, pdfs []giode.PickedFile
	skipped := 0
	for _, f := range s.Files {
		switch {
		case isPDF(f):
			pdfs = append(pdfs, f)
		case strings.HasPrefix(f.MIME, "image/"):
			pictures = append(pictures, f)
		default:
			f.Close()
			skipped++
		}
	}
	if skipped > 0 || (len(s.Files) == 0 && s.Text != "") {
		a.toasts.Show(giode.ToastWarning, "DocLens can import pictures and PDFs")
	}
	if len(pictures) > 0 {
		newFlow(a, scanBatch, nil).importFiles(pictures)
	}
	if len(pdfs) == 0 {
		return
	}
	if !pdfpages.Supported() {
		for _, f := range pdfs {
			f.Close()
		}
		a.fail(pdfpages.ErrNotSupported)
		return
	}
	go func() {
		var queue []sharedPDF
		for _, f := range pdfs {
			path, err := copyToImportDir(a.store.Dir, f)
			f.Close()
			if err != nil {
				a.fail(fmt.Errorf("%s: %w", f.Name, err))
				continue
			}
			queue = append(queue, sharedPDF{path: path, name: strings.TrimSuffix(f.Name, filepath.Ext(f.Name))})
		}
		a.g.Post(func() { importPDFs(a, queue) })
	}()
}

type sharedPDF struct{ path, name string }

// importPDFs imports PDFs one after another.
func importPDFs(a *App, queue []sharedPDF) {
	if len(queue) == 0 {
		return
	}
	importPDF(a, nil, queue[0].path, queue[0].name, func() { importPDFs(a, queue[1:]) })
}

func isPDF(f giode.PickedFile) bool {
	return f.MIME == "application/pdf" || strings.EqualFold(filepath.Ext(f.Name), ".pdf")
}
