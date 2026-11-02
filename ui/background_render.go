package ui

import (
	"sync"

	"github.com/xmhrnx/doclens/internal/library"
	"github.com/xmhrnx/doclens/internal/scan"
)

// renderInBackground renders finished pages and their thumbnails so
// they're ready to view and share, and tidies up old files.
func renderInBackground(a *App, doc *library.Document, pages []*library.Page) {
	clones := make([]*library.Page, len(pages))
	for i, p := range pages {
		clones[i] = p.Clone()
	}
	go func() {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 2)
		for _, p := range clones {
			wg.Add(1)
			sem <- struct{}{}
			go func(p *library.Page) {
				defer wg.Done()
				defer func() { <-sem }()
				img, err := a.store.RenderPage(doc.ID, p)
				if err != nil {
					a.fail(err)
					return
				}
				thumb := scan.Downscale(img, 480)
				a.g.Post(func() { a.thumbs.seed(doc.ID, p, thumb) })
			}(p)
		}
		wg.Wait()
		a.g.Post(func() {
			pruneInBackground(a, doc)
			// Read the text once the renders it needs are on disk.
			a.text.queue(doc, pages)
		})
	}()
}

// pruneInBackground deletes a document's stale renders and unused
// photos, working on a copy so the UI can keep changing the original.
func pruneInBackground(a *App, doc *library.Document) {
	cp := *doc
	cp.Pages = make([]*library.Page, len(doc.Pages))
	for i, p := range doc.Pages {
		cp.Pages[i] = p.Clone()
	}
	go func() {
		if err := a.store.PruneRenders(&cp); err != nil {
			a.fail(err)
		}
		if err := a.store.PrunePhotos(&cp); err != nil {
			a.fail(err)
		}
	}()
}
