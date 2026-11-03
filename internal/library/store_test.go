package library

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/xmhrnx/doclens/internal/scan"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 300, 400))
	for i := range img.Pix {
		img.Pix[i] = 230
	}
	var buf bytes.Buffer
	if err := scan.EncodeJPEG(&buf, img, 90); err != nil {
		t.Fatal(err)
	}
	d := s.NewDocument("", "")
	name, err := s.AddPhoto(d.ID, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	p := NewPage(name, 300, 400, scan.Detection{Quad: scan.FullQuad(300, 400)}, scan.FilterMagic)
	p.Strokes = []scan.Stroke{{Points: []scan.Point{{X: 0.1, Y: 0.1}, {X: 0.5, Y: 0.5}}, Width: 0.01, Color: color.RGBA{0, 0, 200, 255}}}
	d.Pages = append(d.Pages, p)
	page, err := s.RenderPage(d.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if page.Bounds().Dx() < 250 {
		t.Errorf("render %v", page.Bounds())
	}
	data, err := s.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Persist(data); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Docs) != 1 || len(s2.Docs[0].Pages) != 1 || len(s2.Docs[0].Pages[0].Strokes) != 1 {
		t.Fatalf("reloaded %+v", s2.Docs)
	}
	var pdfBuf bytes.Buffer
	src := func(i int) (image.Image, error) { return s2.RenderPage(d.ID, s2.Docs[0].Pages[i]) }
	if err := ExportPDF(&pdfBuf, s2.Docs[0], src, ExportOptions{Quality: QualityMedium}); err != nil {
		t.Fatal(err)
	}
	d.Layout = LayoutIDCard
	if err := ExportPDF(&pdfBuf, d, src, ExportOptions{Quality: QualityLow}); err != nil {
		t.Fatal(err)
	}
	if err := ExportLongImage(&pdfBuf, d, src, ExportOptions{Quality: QualityLow}); err != nil {
		t.Fatal(err)
	}
}

// TestOversizedPhotoKeepsOutline covers camera shots stored at full
// resolution while their outline was found at captureSide: the outline
// must still land on the page.
func TestOversizedPhotoKeepsOutline(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// A 4080x3060 photo: dark desk, white page at (1000,750)-(3000,2250).
	img := image.NewRGBA(image.Rect(0, 0, 4080, 3060))
	for y := 0; y < 3060; y++ {
		for x := 0; x < 4080; x++ {
			v := uint8(40)
			if x >= 1000 && x < 3000 && y >= 750 && y < 2250 {
				v = 250
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, v, v, 255
		}
	}
	var buf bytes.Buffer
	if err := scan.EncodeJPEG(&buf, img, 90); err != nil {
		t.Fatal(err)
	}
	d := s.NewDocument("", "")
	name, err := s.AddPhoto(d.ID, buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	// The outline as found on the 3264 px copy (0.8 times).
	q := scan.Quad{{X: 800, Y: 600}, {X: 2400, Y: 600}, {X: 2400, Y: 1800}, {X: 800, Y: 1800}}
	p := NewPage(name, 3264, 2448, scan.Detection{Quad: q, Found: true}, scan.FilterOriginal)
	photo, err := s.LoadPhoto(d.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	if b := photo.Bounds(); b.Dx() != 3264 || b.Dy() != 2448 {
		t.Fatalf("photo is %v, want 3264x2448", b)
	}
	page, err := s.RenderPage(d.ID, p)
	if err != nil {
		t.Fatal(err)
	}
	// The whole page is paper: no desk along its edges.
	b := page.Bounds()
	for _, pt := range []image.Point{{b.Dx() / 2, 4}, {b.Dx() / 2, b.Dy() - 5}, {4, b.Dy() / 2}, {b.Dx() - 5, b.Dy() / 2}} {
		if c := color.GrayModel.Convert(page.At(pt.X, pt.Y)).(color.Gray); c.Y < 200 {
			t.Errorf("desk at %v (%d): the outline missed the page", pt, c.Y)
		}
	}
}

func TestAutoCaptureStartsOff(t *testing.T) {
	dir := t.TempDir()
	// A library saved before the default changed.
	if err := os.WriteFile(filepath.Join(dir, "library.json"), []byte(`{"settings":{"autoCapture":true,"autoCrop":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Settings.AutoCapture {
		t.Error("automatic capture should start off")
	}
}
