package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xmhrnx/giode"

	"github.com/xmhrnx/doclens/internal/library"
	"github.com/xmhrnx/doclens/internal/pdfpages"
	"github.com/xmhrnx/doclens/internal/scan"
)

const (
	pdfImportDPI  = 250  // sharp text without huge photos
	pdfImportSide = 3264 // the same limit as camera photos
)

// startPDFImport turns the pages of a PDF the user picks into pages:
// a new document, or more pages of doc.
func startPDFImport(a *App, doc *library.Document) {
	if !pdfpages.Supported() {
		a.fail(pdfpages.ErrNotSupported)
		return
	}
	go func() {
		files, err := a.g.PickFiles(context.Background(), giode.PickOptions{Types: []string{"application/pdf"}})
		if err != nil {
			a.fail(err)
			return
		}
		f := files[0]
		// PDF renderers read files, not streams: keep a private copy
		// while importing.
		path, err := copyToImportDir(a.store.Dir, f)
		f.Close()
		if err != nil {
			a.fail(err)
			return
		}
		name := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
		a.g.Post(func() { importPDF(a, doc, path, name) })
	}()
}

func copyToImportDir(base string, r io.Reader) (string, error) {
	dir := filepath.Join(base, "import")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	out, err := os.CreateTemp(dir, "*.pdf")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		os.Remove(out.Name())
		return "", err
	}
	return out.Name(), out.Close()
}

// importPDF renders every page of the PDF at path into a photo and
// adds the pages, untouched (Original filter, no crop), to a document.
func importPDF(a *App, doc *library.Document, path, name string) {
	store := a.store
	docID := library.NewDocumentID()
	if doc != nil {
		docID = doc.ID
	}
	var pages []*library.Page
	a.work("Importing “"+name+"”", func(progress func(float64)) error {
		defer os.Remove(path)
		var err error
		pages, err = renderPDF(store, docID, path, progress)
		if err != nil && doc == nil {
			// Nothing of a new document is kept.
			if perr := store.PurgeDocument(docID); perr != nil {
				err = errors.Join(err, perr)
			}
		}
		return err
	}, func() {
		if doc == nil {
			if name == "" {
				name = library.DefaultName(time.Now())
			}
			doc = store.AddDocument(docID, name, homeFolder(a), pages)
			a.push(newDocumentScreen(a, doc))
		} else {
			doc.Pages = append(doc.Pages, pages...)
			doc.Modified = time.Now()
		}
		a.save()
		a.notify(fmt.Sprintf("Imported %d page%s", len(pages), plural(len(pages))))
		renderInBackground(a, doc, pages)
	})
}

// renderPDF renders the PDF's pages as photos of document docID. It
// only touches files, so it runs off the UI goroutine.
func renderPDF(store *library.Store, docID, path string, progress func(float64)) ([]*library.Page, error) {
	pdf, err := pdfpages.Open(path)
	if err != nil {
		return nil, err
	}
	defer pdf.Close()
	n := pdf.PageCount()
	if n == 0 {
		return nil, errors.New("the PDF has no pages")
	}
	pages := make([]*library.Page, 0, n)
	for i := 0; i < n; i++ {
		img, err := pdf.Render(i, pdfImportDPI, pdfImportSide)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i+1, err)
		}
		var buf bytes.Buffer
		if err := scan.EncodeJPEG(&buf, img, 90); err != nil {
			return nil, err
		}
		photo, err := store.AddPhoto(docID, buf.Bytes())
		if err != nil {
			return nil, err
		}
		w, h := img.Rect.Dx(), img.Rect.Dy()
		pages = append(pages, library.NewPage(photo, w, h, scan.Detection{Quad: scan.FullQuad(w, h)}, scan.FilterOriginal))
		progress(float64(i+1) / float64(n))
	}
	return pages, nil
}
