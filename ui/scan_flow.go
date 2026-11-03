package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"time"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/camera"
	"github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/doclens/internal/library"
	"github.com/xmhrnx/doclens/internal/scan"
)

// scanMode is how the camera is used.
type scanMode int

const (
	scanSingle scanMode = iota // one page
	scanBatch                  // page after page until the camera is closed
	scanIDCard                 // front and back of a card on one sheet
	scanBook                   // a two-page spread, split in two
)

const (
	captureSide = 3264 // photos are kept at up to this size
	viewSide    = 1400 // the editors work on copies this large
)

// photo is a captured or imported picture. Book pages share one.
type photo struct {
	jpeg  []byte
	name  string // file name once stored in the document
	w, h  int
	view  image.Image // a smaller copy for the editors
	scale float64     // view pixels per photo pixel
	// load returns the photo at full size, for sharp previews. It runs
	// off the UI goroutine.
	load func() (image.Image, error)
}

// pendingPage is a page between capture and saving, or an existing
// page being edited.
type pendingPage struct {
	src      *photo
	quad     scan.Quad // in photo pixels
	auto     scan.Quad // what detection found
	detected bool
	rotation int
	filter   scan.Filter
	adjust   scan.Adjust
	page     *library.Page // the page being edited, if any
}

// scanFlow takes pages from the camera or the photo library through
// cropping and enhancing into a document.
type scanFlow struct {
	a       *App
	mode    scanMode
	doc     *library.Document // nil until saved: a new document
	replace *library.Page     // a retake replaces this page
	editing bool              // pages are existing ones
	pages   []*pendingPage
	origin  screen
	// autoCrop is the setting, read on the UI goroutine when the flow
	// starts: detection runs elsewhere.
	autoCrop bool
	filter   scan.Filter
	// cam is the live camera screen, when the device has a camera Giode
	// can drive; nil means the system camera app takes the photos.
	cam *cameraScreen
}

// newFlow starts a flow from the screen on top.
func newFlow(a *App, mode scanMode, doc *library.Document) *scanFlow {
	st := a.store.Settings
	return &scanFlow{
		a: a, mode: mode, doc: doc, origin: a.stack[len(a.stack)-1],
		autoCrop: st.AutoCrop, filter: st.DefaultFilter,
	}
}

// startScan opens the camera. doc is the document to add pages to,
// nil for a new one.
func startScan(a *App, mode scanMode, doc *library.Document) {
	newFlow(a, mode, doc).openCamera()
}

// startRetake replaces a page with a new photo.
func startRetake(a *App, doc *library.Document, p *library.Page) {
	f := newFlow(a, scanSingle, doc)
	f.replace = p
	f.openCamera()
}

// openCamera shows the live scanner, or hands over to the system
// camera app where there's no camera Giode can drive.
func (f *scanFlow) openCamera() {
	if !camera.Supported() {
		f.capture()
		return
	}
	f.cam = newCameraScreen(f)
	f.a.push(f.cam)
}

// startImport adds pictures from the photo library.
func startImport(a *App, doc *library.Document) {
	newFlow(a, scanBatch, doc).importPhotos()
}

// importPhotos lets the user pick photos, finds the page in each and
// opens the review.
func (f *scanFlow) importPhotos() {
	a := f.a
	go func() {
		files, err := a.g.PickPhotos(context.Background(), giode.PhotoOptions{MaxSide: captureSide}, 0)
		if err != nil {
			a.fail(err)
			return
		}
		a.g.Post(func() { f.importFiles(files) })
	}()
}

// importFiles finds the page in each picture file, closing them, and
// opens the review.
func (f *scanFlow) importFiles(files []giode.PickedFile) {
	a := f.a
	a.work(fmt.Sprintf("Finding pages in %d picture%s", len(files), plural(len(files))), func(progress func(float64)) error {
		defer func() {
			for _, file := range files {
				file.Close()
			}
		}()
		var pages []*pendingPage
		for i, file := range files {
			data, err := io.ReadAll(file)
			if err != nil {
				return fmt.Errorf("%s: %w", file.Name, err)
			}
			// Turned upright by its EXIF tag, if it has one.
			img, err := scan.Decode(bytes.NewReader(data))
			if err != nil {
				return fmt.Errorf("%s: %w", file.Name, err)
			}
			pp, err := f.prepare(img, data)
			if err != nil {
				return err
			}
			pages = append(pages, pp...)
			progress(float64(i+1) / float64(len(files)))
		}
		a.g.Post(func() {
			f.pages = append(f.pages, pages...)
			f.review()
		})
		return nil
	}, nil)
}

// startEdit opens existing pages in the crop editor (crop) or the
// enhance screen.
func startEdit(a *App, doc *library.Document, pages []*library.Page, crop bool) {
	f := newFlow(a, scanSingle, doc)
	f.editing = true
	store := a.store
	clones := make([]*library.Page, len(pages))
	for i, p := range pages {
		clones[i] = p.Clone()
	}
	// Pages split from one photo (a book spread) share it.
	shared := map[string]int{}
	for _, p := range clones {
		shared[p.Photo]++
	}
	a.work("Opening", func(func(float64)) error {
		var out []*pendingPage
		photos := map[string]*photo{}
		found := map[string]scan.Detection{}
		for i, p := range clones {
			ph := photos[p.Photo]
			if ph == nil {
				img, err := store.LoadPhoto(doc.ID, p)
				if err != nil {
					return err
				}
				ph = newPhoto(img, nil)
				ph.name = p.Photo
				saved := p
				ph.load = func() (image.Image, error) { return store.LoadPhoto(doc.ID, saved) }
				photos[p.Photo] = ph
				// "Auto" in the crop editor looks for the page afresh.
				if crop && shared[p.Photo] == 1 {
					found[p.Photo] = scan.Detect(img)
				}
			}
			auto, detected := p.Quad, p.Detected
			if d, ok := found[p.Photo]; ok && d.Found {
				auto, detected = d.Quad, true
			}
			out = append(out, &pendingPage{
				src: ph, quad: p.Quad, auto: auto, detected: detected,
				rotation: p.Rotation, filter: p.Filter, adjust: p.Adjust, page: pages[i],
			})
		}
		a.g.Post(func() {
			f.pages = out
			if crop {
				a.push(newCropScreen(f))
			} else {
				a.push(newEnhanceScreen(f))
			}
		})
		return nil
	}, nil)
}

// capture takes one photo with the system camera app and continues by
// mode.
func (f *scanFlow) capture() {
	a := f.a
	go func() {
		ctx := context.Background()
		file, err := a.g.PickPhoto(ctx, giode.PhotoOptions{Source: giode.Camera, MaxSide: captureSide})
		if errors.Is(err, giode.ErrNotSupported) {
			// This device has no camera: take a picture file instead.
			file, err = a.g.PickPhoto(ctx, giode.PhotoOptions{Source: giode.PhotoLibrary, MaxSide: captureSide})
		}
		if err != nil {
			a.g.Post(func() { f.captureEnded(err) })
			return
		}
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			a.fail(err)
			return
		}
		a.g.Post(func() { f.process(data) })
	}()
}

// captureEnded handles the camera closing without a photo: the end of
// a batch, or a cancel.
func (f *scanFlow) captureEnded(err error) {
	if !errors.Is(err, giode.ErrCanceled) {
		f.a.fail(err)
	}
	if len(f.pages) > 0 {
		f.review()
	}
}

// process finds the page in a photo the system camera took.
func (f *scanFlow) process(data []byte) {
	a := f.a
	a.work("Finding the page", func(func(float64)) error {
		pages, err := f.detectIn(data)
		if err != nil {
			return err
		}
		a.g.Post(func() { f.add(pages) })
		return nil
	}, nil)
}

// detectIn decodes a JPEG photo and finds its page(s). It runs off the
// UI goroutine.
func (f *scanFlow) detectIn(data []byte) ([]*pendingPage, error) {
	img, err := scan.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return f.prepare(img, data)
}

// prepare detects the page(s) in a photo. data is the photo's JPEG, or
// nil to encode img.
func (f *scanFlow) prepare(img image.Image, data []byte) ([]*pendingPage, error) {
	img = scan.Downscale(img, captureSide)
	// The page's outline is found in img, so the stored photo must be
	// img: a full-resolution camera shot is larger, and a photo turned
	// by its EXIF tag would be stored sideways.
	if data != nil && !scan.MatchesJPEG(data, img) {
		data = nil
	}
	if data == nil {
		var buf bytes.Buffer
		if err := scan.EncodeJPEG(&buf, img, 92); err != nil {
			return nil, err
		}
		data = buf.Bytes()
	}
	ph := newPhoto(img, data)
	det := scan.Detection{Quad: scan.FullQuad(ph.w, ph.h)}
	if f.autoCrop {
		det = scan.Detect(img)
	}
	filter := f.filter
	if f.mode == scanBook {
		left, right := scan.BookHalves(det.Quad)
		return []*pendingPage{
			{src: ph, quad: left, auto: left, detected: det.Found, filter: filter},
			{src: ph, quad: right, auto: right, detected: det.Found, filter: filter},
		}, nil
	}
	return []*pendingPage{{src: ph, quad: det.Quad, auto: det.Quad, detected: det.Found, filter: filter}}, nil
}

func newPhoto(img image.Image, data []byte) *photo {
	b := img.Bounds()
	view := scan.Downscale(img, viewSide)
	ph := &photo{jpeg: data, w: b.Dx(), h: b.Dy(), view: view, scale: float64(view.Bounds().Dx()) / float64(b.Dx())}
	if data != nil {
		ph.load = func() (image.Image, error) { return scan.Decode(bytes.NewReader(data)) }
	}
	return ph
}

// add takes freshly detected pages and decides what comes next.
func (f *scanFlow) add(pages []*pendingPage) {
	f.pages = append(f.pages, pages...)
	giode.Haptic(giode.HapticSuccess)
	if f.cam != nil {
		f.addLive(pages)
		return
	}
	switch f.mode {
	case scanBatch:
		f.a.notify(fmt.Sprintf("Page %d captured. Close the camera when you're done.", len(f.pages)))
		f.capture()
	case scanIDCard:
		if len(f.pages) == 1 {
			f.a.notify("Front captured. Now turn the card over.")
			f.capture()
			return
		}
		f.review()
	default:
		if f.a.store.Settings.SkipReview && len(pages) == 1 && pages[0].detected {
			f.save()
			return
		}
		f.review()
	}
}

// addLive continues after a shot from the live camera, which stays
// open for the next page of a batch or the back of a card.
func (f *scanFlow) addLive(pages []*pendingPage) {
	switch {
	case f.mode == scanBatch:
		f.a.notify(fmt.Sprintf("Page %d captured", len(f.pages)))
	case f.mode == scanIDCard && len(f.pages) == 1:
		f.a.notify("Front captured. Now turn the card over.")
	case f.mode != scanIDCard && f.a.store.Settings.SkipReview && len(pages) == 1 && pages[0].detected:
		f.save()
	default:
		f.review()
	}
}

// review opens the crop editor on the captured pages.
func (f *scanFlow) review() {
	f.a.push(newCropScreen(f))
}

// close returns to where the flow started, releasing its images.
func (f *scanFlow) close() {
	for _, p := range f.pages {
		properties.ForgetImage(p.src.view)
	}
	f.a.popTo(f.origin)
}

// save stores the pages: a new document, pages added to one, a
// retaken page or edited pages. Rendering runs in the background.
func (f *scanFlow) save() {
	a := f.a
	store := a.store
	doc := f.doc
	isNew := doc == nil
	if isNew {
		doc = store.NewDocument("", homeFolder(a))
		if f.mode == scanIDCard {
			doc.Layout = library.LayoutIDCard
			doc.Name = "ID card " + doc.Created.Format("Jan 2, 2006")
		}
	}
	var saved []*library.Page
	for _, pp := range f.pages {
		if pp.src.name == "" {
			name, err := store.AddPhoto(doc.ID, pp.src.jpeg)
			if err != nil {
				a.fail(err)
				return
			}
			pp.src.name = name
		}
		p := pp.page
		if p == nil {
			p = library.NewPage(pp.src.name, pp.src.w, pp.src.h, scan.Detection{Quad: pp.quad, Found: pp.detected}, pp.filter)
		}
		p.Photo, p.Quad, p.Rotation, p.Filter, p.Adjust = pp.src.name, pp.quad, pp.rotation, pp.filter, pp.adjust
		if pp.page != nil {
			p.Touch()
		}
		saved = append(saved, p)
	}
	switch {
	case f.editing:
		// The pages were edited in place.
	case f.replace != nil:
		for i, p := range doc.Pages {
			if p == f.replace {
				saved[0].ID = p.ID
				saved[0].Rev = p.Rev + 1
				saved[0].Strokes, saved[0].Stamps = p.Strokes, p.Stamps
				doc.Pages[i] = saved[0]
			}
		}
	default:
		doc.Pages = append(doc.Pages, saved...)
	}
	doc.Modified = time.Now()
	a.save()
	f.close()
	if isNew {
		a.push(newDocumentScreen(a, doc))
	}
	renderInBackground(a, doc, saved)
}

// homeFolder files new scans under the folder open on the home screen.
func homeFolder(a *App) string {
	if f := a.home.folder; f != filterAll && f != filterFavorites {
		return f
	}
	return ""
}
