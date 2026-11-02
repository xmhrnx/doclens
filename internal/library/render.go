package library

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"

	"github.com/mehran/scanly/internal/scan"
)

const thumbSide = 480

// LoadPhoto reads a page's photo.
func (s *Store) LoadPhoto(docID string, p *Page) (image.Image, error) {
	f, err := os.Open(s.PhotoPath(docID, p))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return jpeg.Decode(f)
}

// RenderPage returns the finished page - flattened, enhanced and
// annotated - from the cache, rendering and caching it first when
// needed. p should be a Clone when called off the UI goroutine.
func (s *Store) RenderPage(docID string, p *Page) (image.Image, error) {
	if img, err := decodeFile(s.RenderPath(docID, p)); err == nil {
		return img, nil
	}
	photo, err := s.LoadPhoto(docID, p)
	if err != nil {
		return nil, err
	}
	page := scan.Render(photo, p.Params())
	page = scan.Annotate(page, p.Strokes, s.stamps(p))
	var buf bytes.Buffer
	if err := scan.EncodeJPEG(&buf, page, 92); err != nil {
		return nil, err
	}
	if err := writeAtomic(s.RenderPath(docID, p), buf.Bytes()); err != nil {
		return nil, err
	}
	buf.Reset()
	if err := scan.EncodeJPEG(&buf, scan.Downscale(page, thumbSide), 82); err != nil {
		return nil, err
	}
	if err := writeAtomic(s.ThumbPath(docID, p), buf.Bytes()); err != nil {
		return nil, err
	}
	return page, nil
}

// Thumbnail returns a page's thumbnail, rendering the page if needed.
func (s *Store) Thumbnail(docID string, p *Page) (image.Image, error) {
	if img, err := decodeFile(s.ThumbPath(docID, p)); err == nil {
		return img, nil
	}
	page, err := s.RenderPage(docID, p)
	if err != nil {
		return nil, err
	}
	return scan.Downscale(page, thumbSide), nil
}

// stamps loads the signatures placed on a page, skipping deleted ones.
func (s *Store) stamps(p *Page) []scan.Stamp {
	var out []scan.Stamp
	for _, ref := range p.Stamps {
		img, err := s.LoadSignature(ref.Signature)
		if err != nil {
			continue
		}
		out = append(out, scan.Stamp{Image: img, X: ref.X, Y: ref.Y, W: ref.W})
	}
	return out
}

func decodeFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return jpeg.Decode(f)
}

// RenderBase renders a page without its annotations, at most maxSide
// pixels long, for the markup and signing editors. It isn't cached.
func (s *Store) RenderBase(docID string, p *Page, maxSide int) (image.Image, error) {
	photo, err := s.LoadPhoto(docID, p)
	if err != nil {
		return nil, err
	}
	return scan.Downscale(scan.Render(photo, p.Params()), maxSide), nil
}
