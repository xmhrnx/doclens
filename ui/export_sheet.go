package ui

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/input"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/giode/components/sheet"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/giode/ocr"
	"github.com/xmhrnx/giode/printing"

	"github.com/mehran/scanly/internal/library"
	"github.com/mehran/scanly/internal/pdf"
)

// exportSheet turns a document into a PDF, JPEGs or one long image,
// then shares, saves or prints it.
type exportSheet struct {
	a     *App
	shown bool
	sh    *sheet.Sheet
	doc   *library.Document
	// page is the only page to export as JPEG, or -1 for all.
	page int

	format    *segmented.Control
	size      *segmented.Control
	quality   *segmented.Control
	margins   *toggle.Switch
	protect   *toggle.Switch
	password  *input.Input
	stamp     *toggle.Switch
	watermark *input.Input
	text      *toggle.Switch
}

// destination is where an export goes.
type destination int

const (
	toShare destination = iota
	toFiles
	toPhotos
	toPrinter
)

const (
	formatPDF = iota
	formatJPEG
	formatLong
)

var (
	pageSizes = []pdf.PageSize{pdf.SizeAuto, pdf.SizeA4, pdf.SizeLetter, pdf.SizeLegal}
	qualities = []library.Quality{library.QualityLow, library.QualityMedium, library.QualityHigh}
)

func newExportSheet(a *App) *exportSheet {
	e := &exportSheet{a: a, page: -1}
	field := giode.Styles{Background: cFill, BorderColor: cFill, BorderRadius: radiusTile, Color: cText, Padding: giode.SymmetricInset(14, 12)}
	e.format = giode.Input.Segmented("PDF", "JPG", "Long image").Styles(giode.Styles{Width: -1})
	e.size = giode.Input.Segmented("Auto", "A4", "Letter", "Legal").Styles(giode.Styles{Width: -1})
	e.quality = giode.Input.Segmented("Small", "Balanced", "Best").Styles(giode.Styles{Width: -1})
	sw := giode.Styles{Color: cText, FontSize: 15, Width: -1}
	e.margins = giode.Input.Switch("Page margins", sw)
	e.protect = giode.Input.Switch("Password protect", sw)
	e.password = giode.Input.Text("Password", field).Password(true)
	e.stamp = giode.Input.Switch("Watermark", sw)
	e.watermark = giode.Input.Text("Watermark text, e.g. CONFIDENTIAL", field)
	e.text = giode.Input.Switch("Searchable text", sw)
	e.text.SetChecked(true)
	e.sh = giode.Sheet(&e.shown, e.content, giode.Styles{Background: cSurface}).
		Title("Export").Footer(e.footer).Adaptive(true).Detents(giode.DetentLarge)
	return e
}

// open shows the options for a whole document.
func (e *exportSheet) open(d *library.Document) {
	e.page = -1
	e.show(d)
}

// openPage shows the options for page i, as a JPEG.
func (e *exportSheet) openPage(d *library.Document, i int) {
	e.page = i
	e.format.SetSelected(formatJPEG)
	e.show(d)
}

func (e *exportSheet) show(d *library.Document) {
	e.doc = d
	e.applyDefaults()
	e.sh.Title("Export “" + d.Name + "”")
	e.shown = true
}

func (e *exportSheet) applyDefaults() {
	s := e.a.store.Settings
	e.size.SetSelected(indexOf(pageSizes, s.PageSize))
	e.quality.SetSelected(indexOf(qualities, s.Quality))
	e.margins.SetChecked(s.Margins)
}

// quickShare shares the document as a PDF with the default settings.
func (e *exportSheet) quickShare(d *library.Document) {
	e.doc, e.page = d, -1
	e.applyDefaults()
	e.format.SetSelected(formatPDF)
	e.protect.SetChecked(false)
	e.stamp.SetChecked(false)
	e.run(toShare)
}

// quickPrint prints the document as a PDF with the default settings.
func (e *exportSheet) quickPrint(d *library.Document) {
	e.doc, e.page = d, -1
	e.applyDefaults()
	e.format.SetSelected(formatPDF)
	e.protect.SetChecked(false)
	e.stamp.SetChecked(false)
	e.run(toPrinter)
}

func indexOf[T comparable](list []T, v T) int {
	for i, x := range list {
		if x == v {
			return i
		}
	}
	return 0
}

func (e *exportSheet) view() giode.Element { return e.sh }

func (e *exportSheet) content() giode.Element {
	if e.doc == nil {
		return giode.Box(giode.Styles{})
	}
	isPDF := e.format.Selected() == formatPDF
	kids := []giode.Element{optionLabel("Format"), e.format}
	if e.format.Selected() == formatJPEG {
		what := fmt.Sprintf("All %d pages, one picture each", len(e.doc.Pages))
		if e.page >= 0 || len(e.doc.Pages) == 1 {
			what = "This page"
		}
		kids = append(kids, giode.Text(what, tCaption))
	}
	if isPDF {
		kids = append(kids, optionLabel("Page size"), e.size)
	}
	kids = append(kids, optionLabel("Quality"), e.quality, giode.Text(qualityHint(e.quality.Selected()), tCaption))
	kids = append(kids, giode.Divider(giode.Styles{Color: cBorder}))
	if isPDF {
		if ocr.Supported() && e.doc.Layout != library.LayoutIDCard {
			kids = append(kids, e.text)
		}
		kids = append(kids, e.margins, e.protect)
		if e.protect.Checked() {
			kids = append(kids, e.password,
				giode.Row(giode.Styles{Width: -1, Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
					icon("shield-check", 16, cSuccess),
					giode.Box(giode.Styles{FlexGrow: 1}, giode.Text("AES-256 encryption. Without the password the file can't be opened.", tCaption))))
		}
	}
	kids = append(kids, e.stamp)
	if e.stamp.Checked() {
		kids = append(kids, e.watermark)
	}
	return giode.Stack(giode.Styles{Gap: 10, Width: -1, Align: giode.AlignStretch, Padding: giode.Inset{Bottom: 8}}, kids...)
}

func optionLabel(s string) giode.Element {
	return giode.Text(strings.ToUpper(s), with(tTiny, giode.Styles{Margin: giode.Inset{Top: 6}}))
}

func qualityHint(i int) string {
	switch i {
	case 0:
		return "Smallest files, for email and messaging."
	case 2:
		return "Full detail, for printing and archiving."
	}
	return "Sharp text at a moderate size."
}

func (e *exportSheet) footer() giode.Element {
	var kids []giode.Element
	if e.format.Selected() == formatPDF {
		if printing.Supported() {
			kids = append(kids, iconTile("exportPrint", "printer", false, func() { e.run(toPrinter) }))
		}
		kids = append(kids, actionButton("exportSave", buttonSoft, "download", "Save", true, func() { e.run(toFiles) }))
	} else {
		kids = append(kids, actionButton("exportPhotos", buttonSoft, "image-down", "Save to Photos", true, func() { e.run(toPhotos) }))
	}
	kids = append(kids, actionButton("exportShare", buttonPrimary, "share-2", "Share", true, func() { e.run(toShare) }))
	return giode.Row(giode.Styles{Width: -1, Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Padding: giode.Inset{Top: 8, Bottom: 8}}, kids...)
}

// exportFile is one file an export produced.
type exportFile struct {
	name string
	data []byte
}

// run exports with the chosen options and sends the result to dest.
func (e *exportSheet) run(dest destination) {
	a := e.a
	d := e.doc
	if d == nil || len(d.Pages) == 0 {
		a.fail(fmt.Errorf("this document has no pages yet"))
		return
	}
	format := e.format.Selected()
	opts := library.ExportOptions{
		Size:    pageSizes[e.size.Selected()],
		Margins: e.margins.Checked(),
		Quality: qualities[e.quality.Selected()],
		Author:  a.store.Settings.Author,
		Font:    a.font,
	}
	if format == formatPDF && e.protect.Checked() {
		if opts.Password = e.password.Text(); opts.Password == "" {
			a.fail(fmt.Errorf("enter a password, or turn password protection off"))
			return
		}
	}
	if e.stamp.Checked() {
		opts.Watermark = strings.TrimSpace(e.watermark.Text())
	}
	e.shown = false
	store := a.store
	pages := make([]*library.Page, len(d.Pages))
	for i, p := range d.Pages {
		pages[i] = p.Clone()
	}
	if format == formatPDF && e.text.Checked() && ocr.Supported() {
		opts.Text = func(i int) ([]library.TextWord, error) { return e.pageWords(d.ID, pages[i]) }
	}
	snapshot := *d
	snapshot.Pages = pages
	only := e.page
	base := safeName(d.Name)
	var files []exportFile
	a.work("Preparing "+base, func(progress func(float64)) error {
		opts.Progress = func(done, total int) { progress(float64(done) / float64(total)) }
		src := func(i int) (image.Image, error) { return store.RenderPage(d.ID, pages[i]) }
		var err error
		files, err = exportFiles(&snapshot, format, only, base, src, opts)
		return err
	}, func() {
		go func() {
			if err := e.deliver(d.Name, files, dest); err != nil {
				a.fail(err)
			}
		}()
	})
}

// pageWords returns a page's words for a searchable PDF, reading them
// now if they aren't known. It runs off the UI goroutine.
func (e *exportSheet) pageWords(docID string, p *library.Page) ([]library.TextWord, error) {
	if t := p.CurrentText(); t != nil {
		return t.Words, nil
	}
	img, err := e.a.store.RenderPage(docID, p)
	if err != nil {
		return nil, err
	}
	t, err := readText(img, p.Rev)
	if err != nil {
		return nil, err
	}
	e.a.g.Post(func() { e.a.setText(docID, p.ID, t) })
	return t.Words, nil
}

// exportFiles makes the export's files. It runs off the UI goroutine.
func exportFiles(d *library.Document, format, only int, base string, src library.PageSource, opts library.ExportOptions) ([]exportFile, error) {
	var buf bytes.Buffer
	switch format {
	case formatJPEG:
		var files []exportFile
		for i := range d.Pages {
			if only >= 0 && i != only {
				continue
			}
			img, err := src(i)
			if err != nil {
				return nil, err
			}
			buf.Reset()
			if err := library.ExportJPEG(&buf, img, opts); err != nil {
				return nil, err
			}
			name := base + ".jpg"
			if len(d.Pages) > 1 {
				name = fmt.Sprintf("%s (page %d).jpg", base, i+1)
			}
			files = append(files, exportFile{name: name, data: bytes.Clone(buf.Bytes())})
			if opts.Progress != nil {
				opts.Progress(i+1, len(d.Pages))
			}
		}
		return files, nil
	case formatLong:
		if err := library.ExportLongImage(&buf, d, src, opts); err != nil {
			return nil, err
		}
		return []exportFile{{name: base + ".jpg", data: buf.Bytes()}}, nil
	}
	if err := library.ExportPDF(&buf, d, src, opts); err != nil {
		return nil, err
	}
	return []exportFile{{name: base + ".pdf", data: buf.Bytes()}}, nil
}

// deliver sends exported files to the share sheet, a place the user
// picks, the photo gallery or the printer. It blocks: call it on a
// goroutine.
func (e *exportSheet) deliver(title string, files []exportFile, dest destination) error {
	a := e.a
	ctx := context.Background()
	if dest == toFiles {
		for _, f := range files {
			if err := saveAs(ctx, a, f); err != nil {
				return err
			}
		}
		a.notify(savedMessage(files))
		return nil
	}
	paths, err := writeExports(a.store.Dir, files)
	if err != nil {
		return err
	}
	switch dest {
	case toPhotos:
		for _, p := range paths {
			if err := a.g.SaveToPhotos(ctx, p, "Scanly"); err != nil {
				return err
			}
		}
		a.notify(fmt.Sprintf("Saved %d picture%s to Photos", len(paths), plural(len(paths))))
		return nil
	case toPrinter:
		return printing.Print(ctx, a.g, paths[0], printing.Options{JobName: title})
	}
	if len(paths) == 1 {
		return a.g.Share(ctx, giode.ShareFile{Path: paths[0], Title: files[0].name})
	}
	return a.g.Share(ctx, giode.ShareFiles{Paths: paths, Title: title})
}

// writeExports puts files in the app's export folder, replacing the
// last export's.
func writeExports(base string, files []exportFile) ([]string, error) {
	dir := filepath.Join(base, "export")
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = filepath.Join(dir, f.name)
		if err := os.WriteFile(paths[i], f.data, 0o644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func saveAs(ctx context.Context, a *App, f exportFile) error {
	w, err := a.g.SaveFile(ctx, f.name)
	if err != nil {
		return err
	}
	if _, err := w.Write(f.data); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func savedMessage(files []exportFile) string {
	if len(files) == 1 {
		return fmt.Sprintf("Saved %s (%s)", files[0].name, fileSize(len(files[0].data)))
	}
	return fmt.Sprintf("Saved %d files", len(files))
}

// safeName makes a document name usable as a file name.
func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(s))
	if s == "" {
		return "Scan"
	}
	return s
}
