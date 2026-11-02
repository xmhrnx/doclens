package scan

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"
)

func grayFrame(rgba *image.RGBA, side int) *image.Gray {
	small := Downscale(rgba, side)
	b := small.Bounds()
	g := image.NewGray(b)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			g.Set(x, y, color.GrayModel.Convert(small.At(x, y)))
		}
	}
	return g
}

func BenchmarkDetectFrame(b *testing.B) {
	rng := rand.New(rand.NewSource(3))
	o := synthOpts{W: 1500, H: 2000, Paper: color.RGBA{245, 243, 238, 255},
		Bg1: color.RGBA{90, 60, 40, 255}, Bg2: color.RGBA{120, 80, 50, 255}, Noise: 6, TextLines: 30,
		Quad: Quad{{210, 260}, {1290, 330}, {1350, 1780}, {140, 1700}}}
	frame := grayFrame(synthPhoto(o, rng), 640)
	b.Run("Detect", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			Detect(frame)
		}
	})
	b.Run("Preview", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			DetectPreview(frame)
		}
	})
}

func TestDetectPreviewOnGrayFrame(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	o := synthOpts{W: 1500, H: 2000, Paper: color.RGBA{245, 243, 238, 255},
		Bg1: color.RGBA{60, 60, 70, 255}, Bg2: color.RGBA{80, 80, 90, 255}, Noise: 6, TextLines: 30,
		Quad: Quad{{300, 250}, {1250, 330}, {1300, 1750}, {200, 1700}}}
	frame := grayFrame(synthPhoto(o, rng), 640)
	d := DetectPreview(frame)
	s := float64(frame.Rect.Dx()) / 1500
	if !d.Found {
		t.Fatal("not found")
	}
	// An on-screen outline needs about 1% of the frame's diagonal.
	if e := maxCornerError(d.Quad, o.Quad.Scale(s, s)); e > 0.01*math.Hypot(640*0.75, 640) {
		t.Errorf("corners off by %.1f px at 640 px: %v", e, d.Quad)
	}
}

func TestMorphMatchesNaive(t *testing.T) {
	rng := rand.New(rand.NewSource(9))
	p := newPlane(37, 23)
	for i := range p.Pix {
		p.Pix[i] = rng.Float32()
	}
	for _, r := range []int{1, 2, 5} {
		for _, dil := range []bool{true, false} {
			got := morph(p, r, dil)
			for y := 0; y < p.H; y++ {
				for x := 0; x < p.W; x++ {
					want := p.at(x, y)
					for dy := -r; dy <= r; dy++ {
						for dx := -r; dx <= r; dx++ {
							v := p.at(x+dx, y+dy)
							if (dil && v > want) || (!dil && v < want) {
								want = v
							}
						}
					}
					if g := got.Pix[y*p.W+x]; g != want {
						t.Fatalf("r=%d dilate=%v at %d,%d: %v want %v", r, dil, x, y, g, want)
					}
				}
			}
		}
	}
}
