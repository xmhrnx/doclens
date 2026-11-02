// Package pdf writes scanned pages as a PDF: one JPEG image per page,
// optionally protected with a password (AES-256, PDF 2.0 standard
// security handler, revision 6).
package pdf

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

// PageSize selects the paper the pages are laid out on.
type PageSize string

const (
	// SizeAuto makes each page the shape of its image.
	SizeAuto   PageSize = "auto"
	SizeA4     PageSize = "a4"
	SizeLetter PageSize = "letter"
	SizeLegal  PageSize = "legal"
)

// dimensions in points (1/72 inch), portrait.
var sizes = map[PageSize][2]float64{
	SizeA4:     {595.28, 841.89},
	SizeLetter: {612, 792},
	SizeLegal:  {612, 1008},
}

// Page is one page: a JPEG and its pixel size.
type Page struct {
	JPEG []byte
	W, H int
	// Words is the page's recognized text, written as an invisible
	// layer over the image so the PDF can be searched and its text
	// selected and copied.
	Words []Word
	// Gray is true for a single channel JPEG.
	Gray bool
}

// Word is a recognized word and its box on the page image, in
// fractions of the image's width and height from the top left.
type Word struct {
	Text           string
	X0, Y0, X1, Y1 float64
}

// Options configure the document.
type Options struct {
	Size PageSize
	// Margin around the image on fixed page sizes, in points.
	Margin   float64
	Title    string
	Author   string
	Password string
}

// Write writes the pages as a PDF document.
func Write(out io.Writer, pages []Page, opts Options) error {
	if len(pages) == 0 {
		return fmt.Errorf("pdf: no pages")
	}
	w := &writer{out: bufio.NewWriter(out)}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	sum := md5.Sum(append(id, []byte(opts.Title)...))
	id = sum[:]
	if opts.Password != "" {
		enc, err := newEncryption(opts.Password)
		if err != nil {
			return err
		}
		w.enc = enc
	}

	w.write([]byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"))
	n := len(pages)
	// Object numbers: 1 catalog, 2 page tree, 3 info, 4 encrypt, 5 the
	// text layer's font, then three per page (page, content, image).
	const catalog, tree, info, encObj, fontObj = 1, 2, 3, 4, 5
	pageObj := func(i int) int { return 6 + 3*i }

	w.begin(catalog)
	w.raw("<< /Type /Catalog /Pages 2 0 R >>")
	w.end()

	w.begin(tree)
	w.raw("<< /Type /Pages /Count %d /Kids [", n)
	for i := range pages {
		w.raw(" %d 0 R", pageObj(i))
	}
	w.raw(" ] >>")
	w.end()

	now := time.Now()
	_, offset := now.Zone()
	tz := fmt.Sprintf("%+03d'%02d'", offset/3600, abs(offset%3600)/60)
	date := "D:" + now.Format("20060102150405") + tz
	w.begin(info)
	w.raw("<< /Producer %s /Creator %s /CreationDate %s /ModDate %s",
		w.str(info, "DocLens"), w.str(info, "DocLens"), w.str(info, date), w.str(info, date))
	if opts.Title != "" {
		w.raw(" /Title %s", w.str(info, opts.Title))
	}
	if opts.Author != "" {
		w.raw(" /Author %s", w.str(info, opts.Author))
	}
	w.raw(" >>")
	w.end()

	w.begin(encObj)
	if w.enc != nil {
		w.raw("%s", w.enc.dict())
	} else {
		w.raw("null")
	}
	w.end()

	w.begin(fontObj)
	w.raw("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
	w.end()

	for i, p := range pages {
		pw, ph, x, y, iw, ih := layout(p, opts)
		po := pageObj(i)
		w.begin(po)
		w.raw("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Resources << /XObject << /Im0 %d 0 R >> /Font << /F0 %d 0 R >> >> /Contents %d 0 R >>",
			num(pw), num(ph), po+2, fontObj, po+1)
		w.end()

		content := []byte(fmt.Sprintf("q %s 0 0 %s %s %s cm /Im0 Do Q", num(iw), num(ih), num(x), num(y)))
		content = append(content, textLayer(p.Words, x, y, iw, ih)...)
		w.stream(po+1, "", content)

		cs := "/DeviceRGB"
		if p.Gray {
			cs = "/DeviceGray"
		}
		w.stream(po+2, fmt.Sprintf("/Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace %s /BitsPerComponent 8 /Filter /DCTDecode", p.W, p.H, cs), p.JPEG)
	}

	xref := w.n
	total := pageObj(n)
	w.raw("xref\n0 %d\n0000000000 65535 f \n", total)
	for i := 1; i < total; i++ {
		w.raw("%010d 00000 n \n", w.offsets[i])
	}
	w.raw("trailer\n<< /Size %d /Root 1 0 R /Info 3 0 R /ID [<%x> <%x>]", total, id, id)
	if w.enc != nil {
		w.raw(" /Encrypt 4 0 R")
	}
	w.raw(" >>\nstartxref\n%d\n%%%%EOF\n", xref)
	if w.err != nil {
		return w.err
	}
	return w.out.Flush()
}

// layout returns the page size and the image's placement in points.
func layout(p Page, opts Options) (pw, ph, x, y, iw, ih float64) {
	aspect := float64(p.H) / float64(p.W)
	size, ok := sizes[opts.Size]
	if !ok {
		// The page takes the image's shape, with its longer side as
		// long as an A4 page's.
		long := sizes[SizeA4][1]
		if aspect >= 1 {
			return long / aspect, long, 0, 0, long / aspect, long
		}
		return long, long * aspect, 0, 0, long, long * aspect
	}
	pw, ph = size[0], size[1]
	if aspect < 1 {
		pw, ph = ph, pw // landscape image, landscape page
	}
	m := opts.Margin
	aw, ah := pw-2*m, ph-2*m
	iw, ih = aw, aw*aspect
	if ih > ah {
		ih, iw = ah, ah/aspect
	}
	return pw, ph, (pw - iw) / 2, (ph - ih) / 2, iw, ih
}

func num(v float64) string { return fmt.Sprintf("%.2f", v) }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type writer struct {
	out     *bufio.Writer
	n       int
	offsets map[int]int
	enc     *encryption
	err     error
}

func (w *writer) raw(format string, a ...any) {
	if w.err != nil {
		return
	}
	k, err := fmt.Fprintf(w.out, format, a...)
	w.n += k
	w.err = err
}

func (w *writer) write(b []byte) {
	if w.err != nil {
		return
	}
	k, err := w.out.Write(b)
	w.n += k
	w.err = err
}

func (w *writer) begin(obj int) {
	if w.offsets == nil {
		w.offsets = map[int]int{}
	}
	w.offsets[obj] = w.n
	w.raw("%d 0 obj\n", obj)
}

func (w *writer) end() { w.raw("\nendobj\n") }

func (w *writer) stream(obj int, dict string, data []byte) {
	if w.enc != nil {
		data = w.enc.encrypt(data)
	}
	w.begin(obj)
	w.raw("<< %s /Length %d >>\nstream\n", dict, len(data))
	w.write(data)
	w.raw("\nendstream")
	w.end()
}

// str encodes a text string: UTF-16 with a byte order mark when it
// isn't plain ASCII, encrypted when the document is, as hex.
func (w *writer) str(obj int, s string) string {
	b := textBytes(s)
	if w.enc != nil {
		b = w.enc.encrypt(b)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "<%x>", b)
	return buf.String()
}

func textBytes(s string) []byte {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return []byte(s)
	}
	out := []byte{0xFE, 0xFF}
	for _, r := range s {
		if r > 0xFFFF {
			r -= 0x10000
			hi, lo := 0xD800+(r>>10), 0xDC00+(r&0x3FF)
			out = append(out, byte(hi>>8), byte(hi), byte(lo>>8), byte(lo))
			continue
		}
		out = append(out, byte(r>>8), byte(r))
	}
	return out
}
