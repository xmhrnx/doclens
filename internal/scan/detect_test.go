package scan

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"testing"
)

// synthPhoto renders a page with text lines in perspective onto a
// textured background, with uneven light, a shadow, blur-like noise
// and sensor noise.
type synthOpts struct {
	W, H      int
	Quad      Quad
	Paper     color.RGBA
	Bg1, Bg2  color.RGBA
	Shadow    bool
	Noise     float64
	TextLines int
}

func synthPhoto(o synthOpts, rng *rand.Rand) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, o.W, o.H))
	pw, ph := 850.0, 1100.0
	fwd, _ := quadToRect(o.Quad, pw, ph)
	for y := 0; y < o.H; y++ {
		for x := 0; x < o.W; x++ {
			// Background: two-tone stripes like wood grain.
			t := 0.5 + 0.5*math.Sin(float64(x)*0.05+math.Sin(float64(y)*0.01)*3)
			c := lerpRGBA(o.Bg1, o.Bg2, t)
			u, v := fwd.apply(float64(x), float64(y))
			if u >= 0 && v >= 0 && u < pw && v < ph {
				c = o.Paper
				// Text lines.
				if o.TextLines > 0 && u > 80 && u < pw-80 && v > 100 && v < ph-100 {
					row := int((v - 100) / 28)
					if row < o.TextLines && math.Mod(v-100, 28) < 9 {
						word := math.Mod(u+float64(row*37), 90)
						if word < 70 {
							c = color.RGBA{30, 30, 40, 255}
						}
					}
				}
			}
			// Uneven light.
			light := 0.75 + 0.35*float64(x)/float64(o.W) - 0.1*float64(y)/float64(o.H)
			if o.Shadow && x < o.W/3+y/4 {
				light *= 0.7
			}
			n := rng.NormFloat64() * o.Noise
			img.SetRGBA(x, y, color.RGBA{
				clamp8(float64(c.R)*light + n),
				clamp8(float64(c.G)*light + n),
				clamp8(float64(c.B)*light + n),
				255,
			})
		}
	}
	return img
}

func quadToRect(q Quad, w, h float64) (homography, bool) {
	// Solve the homography mapping q's corners to the rectangle.
	rect := Quad{{0, 0}, {w, 0}, {w, h}, {0, h}}
	var a [8][9]float64
	for i := 0; i < 4; i++ {
		x, y := q[i].X, q[i].Y
		u, v := rect[i].X, rect[i].Y
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -u * x, -u * y, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -v * x, -v * y, v}
	}
	for c := 0; c < 8; c++ {
		p := c
		for r := c + 1; r < 8; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		a[c], a[p] = a[p], a[c]
		for r := 0; r < 8; r++ {
			if r == c {
				continue
			}
			f := a[r][c] / a[c][c]
			for k := c; k < 9; k++ {
				a[r][k] -= f * a[c][k]
			}
		}
	}
	var hm homography
	for i := 0; i < 8; i++ {
		hm[i] = a[i][8] / a[i][i]
	}
	hm[8] = 1
	return hm, true
}

func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		255,
	}
}

func clamp8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func maxCornerError(a, b Quad) float64 {
	e := 0.0
	for i := range a {
		e = math.Max(e, a[i].Dist(b[i]))
	}
	return e
}

func TestDetectSynthetic(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	cases := []struct {
		name string
		o    synthOpts
	}{
		{"white on dark wood", synthOpts{W: 1500, H: 2000, Paper: color.RGBA{245, 243, 238, 255},
			Bg1: color.RGBA{90, 60, 40, 255}, Bg2: color.RGBA{120, 80, 50, 255}, Noise: 6, TextLines: 30,
			Quad: Quad{{210, 260}, {1290, 330}, {1350, 1780}, {140, 1700}}}},
		{"strong perspective", synthOpts{W: 1500, H: 2000, Paper: color.RGBA{250, 250, 250, 255},
			Bg1: color.RGBA{40, 40, 45, 255}, Bg2: color.RGBA{70, 70, 75, 255}, Noise: 8, TextLines: 30,
			Quad: Quad{{400, 300}, {1100, 320}, {1420, 1850}, {80, 1820}}}},
		{"low contrast white on light gray with shadow", synthOpts{W: 1500, H: 2000, Paper: color.RGBA{248, 248, 245, 255},
			Bg1: color.RGBA{175, 175, 170, 255}, Bg2: color.RGBA{190, 188, 185, 255}, Noise: 5, TextLines: 30, Shadow: true,
			Quad: Quad{{260, 200}, {1240, 240}, {1260, 1800}, {220, 1760}}}},
		{"colored page same brightness", synthOpts{W: 1500, H: 2000, Paper: color.RGBA{120, 160, 220, 255},
			Bg1: color.RGBA{150, 160, 110, 255}, Bg2: color.RGBA{150, 160, 110, 255}, Noise: 4, TextLines: 20,
			Quad: Quad{{300, 350}, {1200, 300}, {1250, 1650}, {260, 1700}}}},
		{"rotated receipt", synthOpts{W: 1500, H: 2000, Paper: color.RGBA{240, 240, 236, 255},
			Bg1: color.RGBA{30, 60, 90, 255}, Bg2: color.RGBA{40, 70, 100, 255}, Noise: 6, TextLines: 30,
			Quad: Quad{{700, 150}, {1150, 450}, {700, 1850}, {250, 1550}}}},
		{"page running off the frame", synthOpts{W: 1500, H: 2000, Paper: color.RGBA{246, 246, 244, 255},
			Bg1: color.RGBA{60, 50, 45, 255}, Bg2: color.RGBA{80, 65, 55, 255}, Noise: 6, TextLines: 40,
			Quad: Quad{{150, -40}, {1360, -40}, {1380, 1800}, {130, 1820}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img := synthPhoto(c.o, rng)
			d := Detect(img)
			want := c.o.Quad.Order().Clamp(c.o.W, c.o.H)
			err := maxCornerError(d.Quad, want)
			diag := math.Hypot(float64(c.o.W), float64(c.o.H))
			t.Logf("found=%v conf=%.2f err=%.1fpx (%.2f%% of diagonal)", d.Found, d.Confidence, err, 100*err/diag)
			if !d.Found {
				t.Fatalf("no document found; want %v", want)
			}
			if err > 0.012*diag {
				t.Errorf("corners off by %.1fpx: got %v want %v", err, d.Quad, want)
			}
		})
	}
}

func TestDetectNothing(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for i := range img.Pix {
		img.Pix[i] = uint8(120 + rng.Intn(20))
	}
	d := Detect(img)
	if d.Found {
		t.Errorf("found a document in noise: %v (%.2f)", d.Quad, d.Confidence)
	}
}

func TestOutputSizeRecoversA4(t *testing.T) {
	// A head-on A4 page.
	q := Quad{{100, 100}, {1100, 100}, {1100, 1514}, {100, 1514}}
	w, h := OutputSize(q, 1200, 1700)
	if r := float64(h) / float64(w); math.Abs(r-math.Sqrt2) > 0.01 {
		t.Errorf("ratio %.3f, want %.3f", r, math.Sqrt2)
	}
}

func TestOutputSizePerspective(t *testing.T) {
	// A Letter page photographed at an angle: project a rectangle
	// through a known homography.
	// Camera looking down at 30 degrees: build the quad from 3D.
	f := 1400.0
	cx, cy := 750.0, 1000.0
	pw, ph := 8.5, 11.0
	tilt := 30 * math.Pi / 180
	project := func(x, y float64) Point {
		// Page in plane z=0, centered; camera at distance 20, tilted.
		X, Y := x-pw/2, y-ph/2
		Yc := Y*math.Cos(tilt) - 0*math.Sin(tilt)
		Zc := Y*math.Sin(tilt) + 20
		return Point{cx + f*X/Zc, cy + f*Yc/Zc}
	}
	q := Quad{project(0, 0), project(pw, 0), project(pw, ph), project(0, ph)}
	w, h := OutputSize(q, 1500, 2000)
	r := float64(h) / float64(w)
	if math.Abs(r-ph/pw) > 0.02 {
		t.Errorf("ratio %.3f, want %.3f (quad %v)", r, ph/pw, q)
	}
}

func TestHullToQuadRestoresCutCorner(t *testing.T) {
	// A square with one corner cut off.
	hull := []Point{{0, 0}, {100, 0}, {100, 80}, {80, 100}, {0, 100}}
	q, ok := hullToQuad(hull)
	if !ok {
		t.Fatal("no quad")
	}
	want := Quad{{0, 0}, {100, 0}, {100, 100}, {0, 100}}
	if e := maxCornerError(q, want); e > 0.5 {
		t.Errorf("got %v", q)
	}
}

// TestSmallPageOnBusyDesk is a small sheet on a dark desk with long
// straight lines running the frame's height (a keyboard's edge, the
// desk's): with the frame's own edges they make a large outline, which
// must not beat the sheet.
func TestSmallPageOnBusyDesk(t *testing.T) {
	rng := rand.New(rand.NewSource(21))
	o := synthOpts{W: 1500, H: 2000, Paper: color.RGBA{238, 238, 232, 255},
		Bg1: color.RGBA{28, 26, 34, 255}, Bg2: color.RGBA{36, 34, 44, 255}, Noise: 6, TextLines: 24,
		Quad: Quad{{560, 760}, {1080, 790}, {1060, 1450}, {540, 1420}}}
	img := synthPhoto(o, rng)
	bar := func(x0, x1 int, c color.RGBA) {
		for y := 0; y < o.H; y++ {
			for x := x0; x < x1; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
	bar(1260, 1300, color.RGBA{200, 205, 210, 255}) // keyboard edge
	bar(1300, 1500, color.RGBA{150, 150, 160, 255}) // keys
	bar(150, 175, color.RGBA{170, 120, 70, 255})    // desk edge
	frame := Downscale(img, 640)
	fs := float64(o.H) / float64(frame.Bounds().Dy())
	for name, d := range map[string]Detection{
		"full":    Detect(img),
		"preview": func() Detection { d := DetectPreview(frame); d.Quad = d.Quad.Scale(fs, fs); return d }(),
	} {
		if err := maxCornerError(d.Quad, o.Quad); !d.Found || err > 0.02*2500 {
			t.Errorf("%s: found=%v, corners off by %.0f px: %v", name, d.Found, err, d.Quad)
		}
	}
}
