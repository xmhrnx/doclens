// Package pdfpages renders the pages of a PDF to images, so existing
// PDFs can be imported as scans:
//
//	doc, err := pdfpages.Open("contract.pdf")
//	defer doc.Close()
//	for i := 0; i < doc.PageCount(); i++ {
//		img, err := doc.Render(i, 250, 3000) // 250 dpi, at most 3000 px long
//	}
//
// Android renders with PdfRenderer (the app's Java class
// com.xmhrnx.doclens.PdfPages), iOS and macOS with Core Graphics. Other
// platforms return ErrNotSupported.
package pdfpages

import (
	"errors"
	"image"
	"math"
	"sync"
)

var (
	// ErrNotSupported is returned where the platform has no PDF
	// renderer.
	ErrNotSupported = errors.New("opening PDFs isn't supported on this device")
	// ErrPassword is returned for a PDF that needs a password to open.
	ErrPassword = errors.New("this PDF is password protected; remove the password and try again")
	// ErrClosed is returned by a closed Doc's methods.
	ErrClosed = errors.New("pdfpages: the document is closed")
)

// Doc is an open PDF. Its methods may be called from any goroutine;
// pages render one at a time.
type Doc struct {
	mu     sync.Mutex
	plat   platformDoc
	pages  int
	closed bool
}

// Supported reports whether this platform can render PDFs.
func Supported() bool { return supported }

// Open opens the PDF file at path.
func Open(path string) (*Doc, error) {
	if !supported {
		return nil, ErrNotSupported
	}
	p, n, err := open(path)
	if err != nil {
		return nil, err
	}
	return &Doc{plat: p, pages: n}, nil
}

// PageCount returns the number of pages.
func (d *Doc) PageCount() int { return d.pages }

// PageSize returns page i's size in points (1/72 inch) as it's shown.
func (d *Doc) PageSize(i int) (w, h float64, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.check(i); err != nil {
		return 0, 0, err
	}
	return d.plat.pageSize(i)
}

// Render renders page i (from 0) on white at dpi dots per inch, scaled
// down if needed so its longer side is at most maxSide pixels.
func (d *Doc) Render(i int, dpi float64, maxSide int) (*image.RGBA, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.check(i); err != nil {
		return nil, err
	}
	pw, ph, err := d.plat.pageSize(i)
	if err != nil {
		return nil, err
	}
	w, h, ok := renderSize(pw, ph, dpi, maxSide)
	if !ok {
		return nil, errors.New("pdfpages: the page has no size")
	}
	pix, err := d.plat.render(i, w, h)
	if err != nil {
		return nil, err
	}
	if len(pix) != w*h*4 {
		return nil, errors.New("pdfpages: the renderer returned the wrong size")
	}
	return &image.RGBA{Pix: pix, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}, nil
}

func (d *Doc) check(i int) error {
	if d.closed {
		return ErrClosed
	}
	if i < 0 || i >= d.pages {
		return errors.New("pdfpages: no such page")
	}
	return nil
}

// Close releases the document.
func (d *Doc) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	return d.plat.close()
}

// renderSize turns a page size in points into pixels at dpi, capped so
// the longer side is at most maxSide.
func renderSize(pw, ph, dpi float64, maxSide int) (w, h int, ok bool) {
	if pw <= 0 || ph <= 0 || dpi <= 0 || maxSide <= 0 {
		return 0, 0, false
	}
	s := dpi / 72
	if long := math.Max(pw, ph) * s; long > float64(maxSide) {
		s *= float64(maxSide) / long
	}
	return max(1, int(math.Round(pw*s))), max(1, int(math.Round(ph*s))), true
}
