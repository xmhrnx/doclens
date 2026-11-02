package library

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"

	xdraw "golang.org/x/image/draw"

	"github.com/mehran/scanly/internal/pdf"
	"github.com/mehran/scanly/internal/scan"
)

// ExportOptions configure an export.
type ExportOptions struct {
	Size      pdf.PageSize
	Margins   bool
	Quality   Quality
	Password  string
	Watermark string
	Author    string
	// Font is the TrueType font watermarks are drawn in.
	Font []byte
	// Progress, when set, is called after each page with the number
	// done and the total.
	Progress func(done, total int)
	// Text, when set, returns page i's words for a searchable PDF.
	Text func(i int) ([]TextWord, error)
}

// PageSource returns the finished image of page i.
type PageSource func(i int) (image.Image, error)

// ExportPDF writes the document's pages as a PDF.
func ExportPDF(w io.Writer, d *Document, src PageSource, opts ExportOptions) error {
	images, err := collect(d, src, opts)
	if err != nil {
		return err
	}
	size := opts.Size
	margin := 0.0
	if opts.Margins {
		margin = 28 // about 1 cm
	}
	if d.Layout == LayoutIDCard {
		images = idCardSheets(images)
		size, margin = pdf.SizeA4, 0
	}
	q, maxSide := opts.Quality.JPEG()
	pages := make([]pdf.Page, 0, len(images))
	for i, img := range images {
		page := pdf.Page{}
		// ID card sheets are composed from several pages: no text layer.
		if opts.Text != nil && d.Layout != LayoutIDCard {
			words, err := opts.Text(i)
			if err != nil {
				return fmt.Errorf("reading the text of page %d: %w", i+1, err)
			}
			for _, w := range words {
				page.Words = append(page.Words, pdf.Word{Text: w.Text, X0: w.Box[0], Y0: w.Box[1], X1: w.Box[2], Y1: w.Box[3]})
			}
		}
		img = scan.Downscale(img, maxSide)
		var buf bytes.Buffer
		if err := scan.EncodeJPEG(&buf, img, q); err != nil {
			return err
		}
		b := img.Bounds()
		_, page.Gray = img.(*image.Gray)
		page.JPEG, page.W, page.H = buf.Bytes(), b.Dx(), b.Dy()
		pages = append(pages, page)
	}
	return pdf.Write(w, pages, pdf.Options{
		Size: size, Margin: margin, Title: d.Name, Author: opts.Author, Password: opts.Password,
	})
}

// ExportJPEG writes one page as a JPEG.
func ExportJPEG(w io.Writer, img image.Image, opts ExportOptions) error {
	img, err := watermark(img, opts)
	if err != nil {
		return err
	}
	q, maxSide := opts.Quality.JPEG()
	return scan.EncodeJPEG(w, scan.Downscale(img, maxSide), q)
}

// ExportLongImage stacks every page into one tall JPEG, the way pages
// are shared in chat apps.
func ExportLongImage(w io.Writer, d *Document, src PageSource, opts ExportOptions) error {
	images, err := collect(d, src, opts)
	if err != nil {
		return err
	}
	_, maxSide := opts.Quality.JPEG()
	width := min(maxSide*2/3, 1800)
	const gap = 24
	height := gap
	scaled := make([]image.Rectangle, len(images))
	for i, img := range images {
		b := img.Bounds()
		h := b.Dy() * width / max(1, b.Dx())
		scaled[i] = image.Rect(0, height, width, height+h)
		height += h + gap
	}
	if height > 60000 {
		return fmt.Errorf("the document is too long for one image; export it as a PDF instead")
	}
	out := image.NewRGBA(image.Rect(0, 0, width+2*gap, height))
	draw.Draw(out, out.Rect, image.NewUniform(color.RGBA{232, 234, 240, 255}), image.Point{}, draw.Src)
	for i, img := range images {
		xdraw.ApproxBiLinear.Scale(out, scaled[i].Add(image.Pt(gap, 0)), img, img.Bounds(), xdraw.Src, nil)
	}
	q, _ := opts.Quality.JPEG()
	return scan.EncodeJPEG(w, out, q)
}

func collect(d *Document, src PageSource, opts ExportOptions) ([]image.Image, error) {
	if len(d.Pages) == 0 {
		return nil, fmt.Errorf("the document has no pages")
	}
	out := make([]image.Image, len(d.Pages))
	for i := range d.Pages {
		img, err := src(i)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i+1, err)
		}
		img, err = watermark(img, opts)
		if err != nil {
			return nil, err
		}
		out[i] = img
		if opts.Progress != nil {
			opts.Progress(i+1, len(d.Pages))
		}
	}
	return out, nil
}

func watermark(img image.Image, opts ExportOptions) (image.Image, error) {
	if opts.Watermark == "" {
		return img, nil
	}
	return scan.Watermark(img, opts.Watermark, opts.Font, color.RGBA{120, 120, 130, 255}, 0.22)
}

// idCardSheets lays card sides out two per A4 sheet at their real
// size (85.6 x 54 mm on 210 x 297 mm), at 300 dpi.
func idCardSheets(sides []image.Image) []image.Image {
	dpmm := 300 / 25.4
	sheetW, sheetH := int(210*dpmm), int(297*dpmm)
	cardW := int(85.6 * dpmm)
	var out []image.Image
	for i := 0; i < len(sides); i += 2 {
		sheet := image.NewRGBA(image.Rect(0, 0, sheetW, sheetH))
		draw.Draw(sheet, sheet.Rect, image.White, image.Point{}, draw.Src)
		for k := 0; k < 2 && i+k < len(sides); k++ {
			img := sides[i+k]
			b := img.Bounds()
			w := cardW
			if b.Dy() > b.Dx() {
				w = int(54 * dpmm) // a portrait card
			}
			h := b.Dy() * w / max(1, b.Dx())
			// Front above the middle, back below it, as on a copier.
			cx, cy := sheetW/2, sheetH*(3+2*k)/8
			r := image.Rect(cx-w/2, cy-h/2, cx+w/2, cy+h/2)
			xdraw.CatmullRom.Scale(sheet, r, img, b, xdraw.Src, nil)
		}
		out = append(out, sheet)
	}
	return out
}
