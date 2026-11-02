package library

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mehran/scanly/internal/scan"
)

// Store is the library on disk. Its fields are changed on one
// goroutine (the UI's); Snapshot copies them for saving elsewhere.
type Store struct {
	Dir        string       `json:"-"`
	Docs       []*Document  `json:"docs"`
	Folders    []*Folder    `json:"folders"`
	Signatures []*Signature `json:"signatures"`
	Settings   Settings     `json:"settings"`
}

const indexFile = "library.json"

// Open loads the library in dir, creating an empty one the first time.
func Open(dir string) (*Store, error) {
	s := &Store{Dir: dir, Settings: DefaultSettings()}
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "signatures"), 0o755); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, indexFile))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("library index is damaged: %w", err)
	}
	return s, nil
}

// Snapshot serializes the library for Persist.
func (s *Store) Snapshot() ([]byte, error) {
	return json.MarshalIndent(s, "", " ")
}

// Persist writes a snapshot to disk atomically. It may run on any
// goroutine.
func (s *Store) Persist(data []byte) error {
	return writeAtomic(filepath.Join(s.Dir, indexFile), data)
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// NewDocument creates an empty document, newest first in the library.
func (s *Store) NewDocument(name, folder string) *Document {
	now := time.Now()
	if name == "" {
		name = DefaultName(now)
	}
	d := &Document{ID: newID(), Name: name, FolderID: folder, Created: now, Modified: now}
	s.Docs = append([]*Document{d}, s.Docs...)
	return d
}

// Doc finds a document by ID.
func (s *Store) Doc(id string) *Document {
	for _, d := range s.Docs {
		if d.ID == id {
			return d
		}
	}
	return nil
}

// RemoveDocument takes a document out of the library, keeping its
// files so it can be restored; PurgeDocument deletes them.
func (s *Store) RemoveDocument(id string) (*Document, int) {
	for i, d := range s.Docs {
		if d.ID == id {
			s.Docs = slices.Delete(s.Docs, i, i+1)
			return d, i
		}
	}
	return nil, -1
}

// RestoreDocument puts a removed document back at its old position.
func (s *Store) RestoreDocument(d *Document, at int) {
	at = max(0, min(at, len(s.Docs)))
	s.Docs = slices.Insert(s.Docs, at, d)
}

// PurgeDocument deletes a removed document's files.
func (s *Store) PurgeDocument(id string) error {
	return os.RemoveAll(s.DocDir(id))
}

// DocDir is the folder holding a document's photos and renders.
func (s *Store) DocDir(id string) string { return filepath.Join(s.Dir, "docs", id) }

// AddPhoto stores a photo (JPEG bytes) for a document and returns its
// file name.
func (s *Store) AddPhoto(docID string, jpeg []byte) (string, error) {
	dir := s.DocDir(docID)
	if err := os.MkdirAll(filepath.Join(dir, "render"), 0o755); err != nil {
		return "", err
	}
	name := newID() + ".jpg"
	if err := os.WriteFile(filepath.Join(dir, name), jpeg, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// PhotoPath is the path of a page's photo.
func (s *Store) PhotoPath(docID string, p *Page) string {
	return filepath.Join(s.DocDir(docID), p.Photo)
}

// RenderPath and ThumbPath are the cached finished page and its
// thumbnail for the page's current revision.
func (s *Store) RenderPath(docID string, p *Page) string {
	return filepath.Join(s.DocDir(docID), "render", fmt.Sprintf("%s-%d.jpg", p.ID, p.Rev))
}

func (s *Store) ThumbPath(docID string, p *Page) string {
	return filepath.Join(s.DocDir(docID), "render", fmt.Sprintf("%s-%d-t.jpg", p.ID, p.Rev))
}

// MoveDocsToFolder files documents under a folder ("" for none).
func (s *Store) MoveDocsToFolder(ids []string, folder string) {
	for _, d := range s.Docs {
		if slices.Contains(ids, d.ID) {
			d.FolderID = folder
		}
	}
}

// NewFolder adds a folder.
func (s *Store) NewFolder(name, color string) *Folder {
	f := &Folder{ID: newID(), Name: name, Color: color}
	s.Folders = append(s.Folders, f)
	return f
}

// DeleteFolder removes a folder; its documents stay, unfiled.
func (s *Store) DeleteFolder(id string) {
	s.Folders = slices.DeleteFunc(s.Folders, func(f *Folder) bool { return f.ID == id })
	for _, d := range s.Docs {
		if d.FolderID == id {
			d.FolderID = ""
		}
	}
}

// Folder finds a folder by ID.
func (s *Store) Folder(id string) *Folder {
	for _, f := range s.Folders {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// AddSignature saves a signature image.
func (s *Store) AddSignature(img image.Image) (*Signature, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	b := img.Bounds()
	sig := &Signature{ID: newID(), Created: time.Now(), Aspect: float64(b.Dx()) / float64(max(1, b.Dy()))}
	if err := os.WriteFile(s.SignaturePath(sig.ID), buf.Bytes(), 0o644); err != nil {
		return nil, err
	}
	s.Signatures = append([]*Signature{sig}, s.Signatures...)
	return sig, nil
}

// DeleteSignature removes a saved signature. Pages it was placed on
// lose it at their next render.
func (s *Store) DeleteSignature(id string) error {
	s.Signatures = slices.DeleteFunc(s.Signatures, func(g *Signature) bool { return g.ID == id })
	err := os.Remove(s.SignaturePath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// SignaturePath is the PNG of a saved signature.
func (s *Store) SignaturePath(id string) string {
	return filepath.Join(s.Dir, "signatures", id+".png")
}

// LoadSignature reads a saved signature.
func (s *Store) LoadSignature(id string) (image.Image, error) {
	f, err := os.Open(s.SignaturePath(id))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

// PruneRenders deletes cached renders of earlier page revisions and of
// pages no longer in the document.
func (s *Store) PruneRenders(d *Document) error {
	dir := filepath.Join(s.DocDir(d.ID), "render")
	keep := map[string]bool{}
	for _, p := range d.Pages {
		keep[filepath.Base(s.RenderPath(d.ID, p))] = true
		keep[filepath.Base(s.ThumbPath(d.ID, p))] = true
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !keep[e.Name()] {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// PrunePhotos deletes photos no page of the document uses any more.
func (s *Store) PrunePhotos(d *Document) error {
	used := map[string]bool{}
	for _, p := range d.Pages {
		used[p.Photo] = true
	}
	entries, err := os.ReadDir(s.DocDir(d.ID))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jpg") && !used[e.Name()] {
			if err := os.Remove(filepath.Join(s.DocDir(d.ID), e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewPage makes a page for a photo with the outline found in it, or
// the whole photo when none was found.
func NewPage(photo string, w, h int, det scan.Detection, filter scan.Filter) *Page {
	return &Page{
		ID: newID(), Photo: photo, PhotoW: w, PhotoH: h,
		Quad: det.Quad, Detected: det.Found, Filter: filter,
	}
}

// CopyPages copies pages and their photos into a new document's
// folder, returning the new document's ID and the copies. It only
// touches files, so it may run on any goroutine; AddDocument then puts
// the document in the library.
func (s *Store) CopyPages(srcDocID string, pages []*Page) (string, []*Page, error) {
	id := newID()
	var out []*Page
	err := s.copyPagesInto(srcDocID, id, pages, func(c []*Page) { out = c })
	return id, out, err
}

// CopyPagesInto copies pages and their photos into an existing
// document's folder and returns the copies, for merging. Like
// CopyPages it only touches files.
func (s *Store) CopyPagesInto(srcDocID, dstDocID string, pages []*Page) ([]*Page, error) {
	var out []*Page
	err := s.copyPagesInto(srcDocID, dstDocID, pages, func(c []*Page) { out = c })
	return out, err
}

func (s *Store) copyPagesInto(srcDocID, dstDocID string, pages []*Page, done func([]*Page)) error {
	if err := os.MkdirAll(filepath.Join(s.DocDir(dstDocID), "render"), 0o755); err != nil {
		return err
	}
	copied := map[string]string{}
	var out []*Page
	for _, p := range pages {
		c := p.Clone()
		c.ID = newID()
		name, ok := copied[p.Photo]
		if !ok {
			data, err := os.ReadFile(s.PhotoPath(srcDocID, p))
			if err != nil {
				return err
			}
			name = newID() + ".jpg"
			if err := os.WriteFile(filepath.Join(s.DocDir(dstDocID), name), data, 0o644); err != nil {
				return err
			}
			copied[p.Photo] = name
		}
		c.Photo = name
		// Carry the finished render over, so it needn't be redone.
		if data, err := os.ReadFile(s.RenderPath(srcDocID, p)); err == nil {
			if err := os.WriteFile(s.RenderPath(dstDocID, c), data, 0o644); err != nil {
				return err
			}
		}
		if data, err := os.ReadFile(s.ThumbPath(srcDocID, p)); err == nil {
			if err := os.WriteFile(s.ThumbPath(dstDocID, c), data, 0o644); err != nil {
				return err
			}
		}
		out = append(out, c)
	}
	done(out)
	return nil
}

// AddDocument puts a document made by CopyPages in the library,
// newest first.
func (s *Store) AddDocument(id, name, folder string, pages []*Page) *Document {
	now := time.Now()
	d := &Document{ID: id, Name: name, FolderID: folder, Created: now, Modified: now, Pages: pages}
	s.Docs = append([]*Document{d}, s.Docs...)
	return d
}
