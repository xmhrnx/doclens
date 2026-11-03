package scan

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// TestPipelineOutputs renders a synthetic shadowed photo through every
// filter. With SCAN_OUT set, it writes the results there to look at.
func TestPipelineOutputs(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	o := synthOpts{W: 1200, H: 1600, Paper: color.RGBA{238, 232, 220, 255},
		Bg1: color.RGBA{70, 50, 35, 255}, Bg2: color.RGBA{95, 70, 50, 255}, Noise: 5, TextLines: 28, Shadow: true,
		Quad: Quad{{180, 200}, {1020, 260}, {1080, 1450}, {120, 1390}}}
	photo := synthPhoto(o, rng)
	d := Detect(photo)
	if !d.Found {
		t.Fatal("not found")
	}
	dir := os.Getenv("SCAN_OUT")
	if dir != "" {
		writePNG(t, filepath.Join(dir, "photo.png"), photo)
	}
	for _, f := range Filters {
		page := Render(photo, PageParams{Quad: d.Quad, Filter: f})
		b := page.Bounds()
		if b.Dx() < 500 || b.Dy() < 600 {
			t.Errorf("%s: page too small: %v", f, b)
		}
		if f.IsGray() {
			if _, ok := page.(*image.Gray); !ok {
				t.Errorf("%s: want a gray image, got %T", f, page)
			}
		}
		if f == FilterMagic || f == FilterBW {
			// The paper, shadow included, must come out white.
			if l := meanLuma(page, image.Rect(b.Dx()/20, b.Dy()/40, b.Dx()/6, b.Dy()/14)); l < 235 {
				t.Errorf("%s: shadowed paper is %.0f, want near white", f, l)
			}
			if l := meanLuma(page, image.Rect(b.Dx()*5/6, b.Dy()/40, b.Dx()*19/20, b.Dy()/14)); l < 235 {
				t.Errorf("%s: lit paper is %.0f, want near white", f, l)
			}
		}
		if dir != "" {
			writePNG(t, filepath.Join(dir, string(f)+".png"), page)
		}
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, page, &jpeg.Options{Quality: 85}); err != nil {
			t.Fatal(err)
		}
	}
}

func meanLuma(img image.Image, r image.Rectangle) float64 {
	sum, n := 0.0, 0.0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			sum += float64(c.Y)
			n++
		}
	}
	return sum / n
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestExifOrientation(t *testing.T) {
	// A minimal APP1 block with orientation 6 (big endian TIFF).
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	data := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	data = append(data, seg...)
	data = append(data, 0xFF, 0xDA, 0, 2)
	if o := exifOrientation(data); o != 6 {
		t.Errorf("orientation %d, want 6", o)
	}
}

func TestRotate90(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	r := Rotate90(img, 1)
	if b := r.Bounds(); b.Dx() != 2 || b.Dy() != 3 {
		t.Fatalf("bounds %v", b)
	}
	// Top-left goes to top-right after a clockwise turn.
	if c := color.RGBAModel.Convert(r.At(1, 0)).(color.RGBA); c.R != 255 {
		t.Errorf("pixel moved wrong: %v", c)
	}
}

func TestStrokeIsSolid(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	var pts []Point
	for i := 0; i <= 40; i++ {
		pts = append(pts, Point{X: 0.1 + 0.8*float64(i)/40, Y: 0.5})
	}
	out := Annotate(img, []Stroke{{Points: pts, Width: 0.04, Color: color.RGBA{A: 255}}}, nil).(*image.RGBA)
	for x := 30; x < 170; x++ {
		if a := out.Pix[50*out.Stride+x*4+3]; a < 250 {
			t.Fatalf("gap in the stroke at x=%d (alpha %d)", x, a)
		}
	}
}

func TestMatchesJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	if !MatchesJPEG(buf.Bytes(), img) {
		t.Error("same size: should match")
	}
	if MatchesJPEG(buf.Bytes(), image.NewRGBA(image.Rect(0, 0, 32, 24))) {
		t.Error("a downscaled copy should not match")
	}
}
