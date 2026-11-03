package ui

import (
	"image"
	"sync"

	"github.com/xmhrnx/giode/styles/properties"

	"github.com/xmhrnx/doclens/internal/scan"
)

// previewKey identifies a preview: a page with all its settings.
type previewKey struct {
	page     *pendingPage
	quad     scan.Quad
	rotation int
	filter   scan.Filter
	adjust   scan.Adjust
}

// flatKey identifies a flattened page.
type flatKey struct {
	page *pendingPage
	quad scan.Quad
}

// pagePreviewer renders pending pages for the enhance screen in the
// background: the large preview with the chosen look, and small
// previews of every filter for the filter strip. It is used from the UI
// goroutine; renders run one at a time and only the latest request is
// kept.
type pagePreviewer struct {
	a      *App
	flats  map[flatKey]image.Image
	shown  image.Image
	key    previewKey
	busy   bool
	queued *previewKey

	// full is the full-resolution photo of fullOf, decoded once.
	fullMu sync.Mutex
	full   image.Image
	fullOf *photo

	looks     map[scan.Filter]image.Image
	looksKey  previewKey
	looksBusy bool
}

func newPagePreviewer(a *App) *pagePreviewer {
	return &pagePreviewer{a: a, flats: map[flatKey]image.Image{}}
}

func keyOf(p *pendingPage) previewKey {
	return previewKey{page: p, quad: p.quad, rotation: p.rotation, filter: p.filter, adjust: p.adjust}
}

// preview returns the latest preview of p, and whether it's current.
func (r *pagePreviewer) preview(p *pendingPage) (image.Image, bool) {
	k := keyOf(p)
	if r.shown != nil && r.key == k {
		return r.shown, true
	}
	r.request(k)
	if r.key.page != p {
		return nil, false
	}
	return r.shown, false
}

func (r *pagePreviewer) request(k previewKey) {
	if r.busy {
		r.queued = &k
		return
	}
	r.busy = true
	fk := flatKey{k.page, k.quad}
	flat := r.flats[fk]
	go func() {
		if flat == nil {
			flat = r.sharpFlatten(k.page, k.quad)
		}
		img := scan.Enhance(scan.Rotate90(flat, k.rotation), k.filter, k.adjust)
		r.a.g.Post(func() {
			r.flats[fk] = flat
			if r.shown != nil {
				properties.ForgetImage(r.shown)
			}
			r.shown, r.key, r.busy = img, k, false
			if q := r.queued; q != nil {
				r.queued = nil
				if *q != k {
					r.request(*q)
				}
			}
		})
	}()
}

// flatten cuts the page out of the editor's copy of the photo.
// previewSide is the longest side of the large preview: sharp across a
// phone's screen.
const previewSide = 2000

// sharpFlatten cuts the page out of the full-resolution photo, so the
// preview shows the page as it will be saved. It runs on the render
// goroutine; the last photo stays decoded for further looks at the
// same page.
func (r *pagePreviewer) sharpFlatten(p *pendingPage, q scan.Quad) image.Image {
	if p.src.load == nil {
		return flatten(p, q)
	}
	r.fullMu.Lock()
	full := r.full
	if r.fullOf != p.src {
		full = nil
	}
	r.fullMu.Unlock()
	if full == nil {
		var err error
		if full, err = p.src.load(); err != nil {
			r.a.fail(err)
			return flatten(p, q)
		}
		r.fullMu.Lock()
		r.full, r.fullOf = full, p.src
		r.fullMu.Unlock()
	}
	return scan.Downscale(scan.Flatten(full, q), previewSide)
}

// flatten cuts the page out of the editors' smaller copy of the photo.
func flatten(p *pendingPage, q scan.Quad) image.Image {
	s := p.src.scale
	return scan.Flatten(p.src.view, q.Scale(s, s))
}

// filterLooks returns small previews of p in every filter, or nil
// while they're made.
func (r *pagePreviewer) filterLooks(p *pendingPage) map[scan.Filter]image.Image {
	k := previewKey{page: p, quad: p.quad, rotation: p.rotation}
	if r.looksKey == k && r.looks != nil {
		return r.looks
	}
	if !r.looksBusy {
		r.looksBusy = true
		go func() {
			small := scan.Downscale(scan.Rotate90(flatten(p, k.quad), k.rotation), 200)
			looks := map[scan.Filter]image.Image{}
			for _, f := range scan.Filters {
				looks[f] = scan.Enhance(small, f, scan.Adjust{})
			}
			r.a.g.Post(func() {
				for _, img := range r.looks {
					properties.ForgetImage(img)
				}
				r.looks, r.looksKey, r.looksBusy = looks, k, false
			})
		}()
	}
	if r.looksKey.page == p {
		return r.looks
	}
	return nil
}

// release drops every image.
func (r *pagePreviewer) release() {
	for _, img := range r.flats {
		properties.ForgetImage(img)
	}
	for _, img := range r.looks {
		properties.ForgetImage(img)
	}
	if r.shown != nil {
		properties.ForgetImage(r.shown)
	}
	r.flats, r.looks, r.shown = map[flatKey]image.Image{}, nil, nil
	r.fullMu.Lock()
	r.full, r.fullOf = nil, nil
	r.fullMu.Unlock()
}
