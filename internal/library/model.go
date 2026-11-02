// Package library keeps the user's scanned documents: their pages,
// the photos they came from, edits, folders and settings, stored as
// JSON and JPEG files in the app's data directory.
package library

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/xmhrnx/doclens/internal/scan"
)

// Layout says how a document's pages are put on paper.
type Layout string

const (
	// LayoutPages puts every page on its own sheet.
	LayoutPages Layout = ""
	// LayoutIDCard puts each pair of pages (front and back of a card)
	// on one A4 sheet, at the card's real size.
	LayoutIDCard Layout = "idcard"
)

// Document is a scanned document: one or more pages.
type Document struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	FolderID string    `json:"folder,omitempty"`
	Favorite bool      `json:"favorite,omitempty"`
	Layout   Layout    `json:"layout,omitempty"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
	Pages    []*Page   `json:"pages"`
	Tags     []string  `json:"tags,omitempty"`
}

// Page is one page: a photo and everything done to it. The finished
// page is rendered from these on demand, so every edit can be undone
// or changed later.
type Page struct {
	ID    string `json:"id"`
	Photo string `json:"photo"` // file name in the document's folder
	// PhotoW and PhotoH are the photo's size in pixels.
	PhotoW   int           `json:"pw"`
	PhotoH   int           `json:"ph"`
	Quad     scan.Quad     `json:"quad"`
	Detected bool          `json:"detected"`
	Rotation int           `json:"rotation,omitempty"`
	Filter   scan.Filter   `json:"filter"`
	Adjust   scan.Adjust   `json:"adjust"`
	Strokes  []scan.Stroke `json:"strokes,omitempty"`
	Stamps   []StampRef    `json:"stamps,omitempty"`
	// Rev changes with every edit; cached renders are named after it.
	Rev int `json:"rev"`
	// Text is the text recognized on the finished page, if any.
	Text *PageText `json:"text,omitempty"`
}

// PageText is a page's recognized text.
type PageText struct {
	// Rev is the page revision it was read from: a later edit (a crop,
	// a rotation) makes it stale.
	Rev   int        `json:"rev"`
	Text  string     `json:"text"`
	Words []TextWord `json:"words,omitempty"`
}

// TextWord is a word and its box on the finished page, in fractions of
// the page's width and height from the top left.
type TextWord struct {
	Text string     `json:"t"`
	Box  [4]float64 `json:"b"` // left, top, right, bottom
}

// CurrentText returns the page's text if it was read from the page as
// it is now.
func (p *Page) CurrentText() *PageText {
	if p.Text != nil && p.Text.Rev == p.Rev {
		return p.Text
	}
	return nil
}

// StampRef places a saved signature on a page; see scan.Stamp.
type StampRef struct {
	Signature string  `json:"signature"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	W         float64 `json:"w"`
	// Aspect is the signature's width / height.
	Aspect float64 `json:"aspect"`
}

// Params returns what scan.Render needs for the page.
func (p *Page) Params() scan.PageParams {
	return scan.PageParams{Quad: p.Quad, Rotation: p.Rotation, Filter: p.Filter, Adjust: p.Adjust}
}

// Clone returns a deep copy, for work on another goroutine.
func (p *Page) Clone() *Page {
	c := *p
	c.Strokes = append([]scan.Stroke(nil), p.Strokes...)
	c.Stamps = append([]StampRef(nil), p.Stamps...)
	if p.Text != nil {
		t := *p.Text
		t.Words = append([]TextWord(nil), p.Text.Words...)
		c.Text = &t
	}
	return &c
}

// Touch records an edit.
func (p *Page) Touch() { p.Rev++ }

// Folder groups documents.
type Folder struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Signature is a saved handwritten signature, stored as a PNG with a
// transparent background.
type Signature struct {
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	// Aspect is width / height.
	Aspect float64 `json:"aspect"`
}

// Matches reports whether the document matches a search query (name
// or tags, case-insensitive).
func (d *Document) Matches(q string) bool {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(d.Name), q) {
		return true
	}
	for _, t := range d.Tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return d.MatchingPage(q) >= 0
}

// MatchingPage returns the first page whose recognized text contains
// the query (case-insensitive), or -1.
func (d *Document) MatchingPage(q string) int {
	q = strings.TrimSpace(strings.ToLower(q))
	if q == "" {
		return -1
	}
	for i, p := range d.Pages {
		if p.Text != nil && strings.Contains(strings.ToLower(p.Text.Text), q) {
			return i
		}
	}
	return -1
}

// newID returns a random identifier.
func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand doesn't fail on supported platforms
	}
	return hex.EncodeToString(b)
}

// DefaultName names a new document after the time it was scanned.
func DefaultName(t time.Time) string {
	return "Scan " + t.Format("Jan 2, 2006 15.04")
}

// NewDocumentID returns an identifier for a document whose files are
// written before it joins the library (see AddDocument).
func NewDocumentID() string { return newID() }
