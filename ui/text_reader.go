package ui

import (
	"context"
	"errors"
	"image"
	"sync"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/ocr"

	"github.com/xmhrnx/doclens/internal/library"
)

// readText recognizes the text on a finished page image, with word
// boxes in fractions of the page. It blocks: call it off the UI
// goroutine.
func readText(img image.Image, rev int) (*library.PageText, error) {
	res, err := ocr.Recognize(context.Background(), img, ocr.Options{})
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	t := &library.PageText{Rev: rev, Text: res.Text}
	for _, line := range res.Lines {
		for _, word := range line.Words {
			r := word.Box.Sub(b.Min)
			t.Words = append(t.Words, library.TextWord{
				Text: word.Text,
				Box:  [4]float64{float64(r.Min.X) / w, float64(r.Min.Y) / h, float64(r.Max.X) / w, float64(r.Max.Y) / h},
			})
		}
	}
	return t, nil
}

// textReader recognizes pages' text in the background, one page at a
// time, so search finds words on the pages.
type textReader struct {
	a       *App
	mu      sync.Mutex
	queued  map[string]bool // doc/page/rev keys waiting or done
	work    chan textJob
	started bool
}

type textJob struct {
	docID string
	page  *library.Page // a clone
}

func newTextReader(a *App) *textReader {
	return &textReader{a: a, queued: map[string]bool{}, work: make(chan textJob, 4096)}
}

// enabled reports whether pages are read automatically.
func (r *textReader) enabled() bool {
	return ocr.Supported() && r.a.store.Settings.AutoText
}

// queue reads the text of pages that don't have it yet. Call it on the
// UI goroutine.
func (r *textReader) queue(doc *library.Document, pages []*library.Page) {
	if !r.enabled() {
		return
	}
	if !r.started {
		r.started = true
		go r.run()
	}
	for _, p := range pages {
		if p.CurrentText() != nil {
			continue
		}
		key := cacheKey(doc.ID, p)
		r.mu.Lock()
		seen := r.queued[key]
		r.queued[key] = true
		r.mu.Unlock()
		if !seen {
			select {
			case r.work <- textJob{docID: doc.ID, page: p.Clone()}:
			default: // a long backlog: the rest wait for the next launch
			}
		}
	}
}

// queueAll reads every page in the library that has no text yet.
func (r *textReader) queueAll() {
	for _, d := range r.a.store.Docs {
		r.queue(d, d.Pages)
	}
}

func (r *textReader) run() {
	a := r.a
	for job := range r.work {
		img, err := a.store.RenderPage(job.docID, job.page)
		if err != nil {
			continue // the page may have been deleted meanwhile
		}
		text, err := readText(img, job.page.Rev)
		if errors.Is(err, giode.ErrNotSupported) {
			return
		}
		if err != nil {
			a.fail(err)
			continue
		}
		a.g.Post(func() { a.setText(job.docID, job.page.ID, text) })
	}
}

// setText stores a page's text if the page is still the one read.
func (a *App) setText(docID, pageID string, t *library.PageText) {
	d := a.store.Doc(docID)
	if d == nil {
		return
	}
	for _, p := range d.Pages {
		if p.ID == pageID && p.Rev == t.Rev {
			p.Text = t
			a.save()
			return
		}
	}
}
