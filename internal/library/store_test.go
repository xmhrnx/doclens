package library

import (
	"bytes"
	"image"
	"image/color"
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
