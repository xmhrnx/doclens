package ui

import (
	"fmt"
	"image"

	"github.com/xmhrnx/giode/styles/properties"

	"github.com/mehran/scanly/internal/library"
	"github.com/mehran/scanly/internal/scan"
)

// screenSide is the longest side of page images shown on screen: sharp
// on a phone, without holding full-resolution scans on the GPU.
const screenSide = 2200

func downscaleForScreen(img image.Image) image.Image { return scan.Downscale(img, screenSide) }

type imageResult struct{ img image.Image }

// imageCache loads rendered pages in the background and keeps the most
// recently used ones. It is used from the UI goroutine only.
type imageCache struct {
	app     *App
	max     int
	load    func(docID string, p *library.Page) (imageResult, error)
	imgs    map[string]image.Image
	order   []string
	pending map[string]bool
	failed  map[string]bool
	sem     chan struct{}
}

func newImageCache(a *App, max int, load func(string, *library.Page) (imageResult, error)) *imageCache {
	return &imageCache{
		app: a, max: max, load: load,
		imgs: map[string]image.Image{}, pending: map[string]bool{}, failed: map[string]bool{},
		sem: make(chan struct{}, 2),
	}
}

func cacheKey(docID string, p *library.Page) string {
	return fmt.Sprintf("%s/%s/%d", docID, p.ID, p.Rev)
}

// get returns the page's image, or nil while it loads.
func (c *imageCache) get(docID string, p *library.Page) image.Image {
	key := cacheKey(docID, p)
	if img, ok := c.imgs[key]; ok {
		return img
	}
	if c.pending[key] || c.failed[key] {
		return nil
	}
	c.pending[key] = true
	page := p.Clone()
	go func() {
		c.sem <- struct{}{}
		res, err := c.load(docID, page)
		<-c.sem
		c.app.g.Post(func() {
			delete(c.pending, key)
			if err != nil {
				c.failed[key] = true
				c.app.fail(fmt.Errorf("couldn't show a page: %w", err))
				return
			}
			c.put(key, res.img)
		})
	}()
	return nil
}

// put adds an image, evicting the least recently added beyond max.
func (c *imageCache) put(key string, img image.Image) {
	if _, ok := c.imgs[key]; !ok {
		c.order = append(c.order, key)
	}
	c.imgs[key] = img
	for len(c.order) > c.max {
		old := c.order[0]
		c.order = c.order[1:]
		if im, ok := c.imgs[old]; ok {
			properties.ForgetImage(im)
		}
		delete(c.imgs, old)
	}
}

// seed stores an image rendered elsewhere (right after a scan), so it
// shows without loading.
func (c *imageCache) seed(docID string, p *library.Page, img image.Image) {
	c.put(cacheKey(docID, p), img)
}
