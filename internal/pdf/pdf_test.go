package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testPage(t *testing.T, w, h int) Page {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(1, 1, color.Black)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return Page{JPEG: buf.Bytes(), W: w, H: h}
}

func TestLayout(t *testing.T) {
	pw, ph, x, y, iw, ih := layout(Page{W: 1000, H: 2000}, Options{Size: SizeA4, Margin: 20})
	if pw != 595.28 || ph != 841.89 {
		t.Errorf("page %vx%v", pw, ph)
	}
	if ih > ph-40+0.01 || iw/ih < 0.49 || iw/ih > 0.51 || x < 20 || y < 19.99 {
		t.Errorf("image at %v,%v size %vx%v", x, y, iw, ih)
	}
	pw, ph, _, _, _, _ = layout(Page{W: 2000, H: 1000}, Options{Size: SizeLetter})
	if pw != 792 {
		t.Errorf("landscape image should get a landscape page, got %vx%v", pw, ph)
	}
}

// TestReadable checks the output with pypdf when it's installed,
// including opening the encrypted document with its password.
func TestReadable(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3")
	}
	if exec.Command(py, "-c", "import pypdf, cryptography").Run() != nil {
		t.Skip("pypdf not installed")
	}
	dir := t.TempDir()
	pages := []Page{testPage(t, 400, 560), testPage(t, 600, 300)}
	pages[0].Words = []Word{
		{Text: "Invoice", X0: 0.1, Y0: 0.1, X1: 0.4, Y1: 0.15},
		{Text: "Café(1)", X0: 0.45, Y0: 0.1, X1: 0.8, Y1: 0.15},
	}
	for _, pw := range []string{"", "s3cret pässword"} {
		path := filepath.Join(dir, "doc.pdf")
		var buf bytes.Buffer
		if err := Write(&buf, pages, Options{Size: SizeA4, Title: "Rechnung März", Password: pw}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		script := `
import sys, pypdf
r = pypdf.PdfReader(sys.argv[1])
if r.is_encrypted:
    assert r.decrypt("wrong") == 0, "wrong password accepted"
    assert r.decrypt(sys.argv[2]) != 0, "password rejected"
assert len(r.pages) == 2
img = r.pages[0].images[0]
assert img.image.size == (400, 560), img.image.size
print(r.metadata.title)
text = r.pages[0].extract_text()
assert "Invoice" in text and "Café(1)" in text, repr(text)
`
		out, err := exec.Command(py, "-c", script, path, pw).CombinedOutput()
		if err != nil {
			t.Fatalf("password %q: %v\n%s", pw, err, out)
		}
		if !strings.Contains(string(out), "Rechnung März") {
			t.Errorf("title: %s", out)
		}
	}
}
