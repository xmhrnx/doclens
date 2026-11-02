package scan

import (
	"image"
	"math"
	"sort"
)

// Detection is the outcome of looking for a document in a photo.
type Detection struct {
	// Quad is the document outline in the photo's pixel coordinates.
	Quad Quad
	// Found is false when no convincing outline was found; Quad then
	// covers the whole photo.
	Found bool
	// Confidence is in [0, 1].
	Confidence float64
}

const analysisSide = 640

// Detect finds the outline of a document (a page, receipt, card or
// whiteboard) in a photo.
//
// It combines three independent ways of proposing outlines - straight
// lines from a Hough transform, contours of edge regions, and a
// foreground/background color segmentation - scores every proposal by
// how well its four sides follow real edges in the photo, and refines
// the winner's corners on a higher resolution copy.
func Detect(img image.Image) Detection {
	return detect(img, analysisSide, true)
}

// DetectPreview is Detect for live camera frames: it skips the
// high-resolution corner refinement, which a 640 pixel frame doesn't
// need, and so runs several times a second on a phone.
func DetectPreview(img image.Image) Detection {
	return detect(img, previewSide, false)
}

// previewSide is the analysis size for live frames: enough to place an
// outline on screen.
const previewSide = 400

func detect(img image.Image, side int, refineCorners bool) Detection {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 16 || h < 16 {
		return Detection{Quad: FullQuad(w, h)}
	}
	src := toRGBA(img)
	planes, scale := downscaleRGB(src, side)
	aw, ah := planes[0].W, planes[0].H

	// Close away text and thin lines so they don't compete with the
	// page outline, then smooth. A grayscale picture (a camera frame)
	// has one channel worth the work, not three.
	r := max(2, aw/220)
	channels := planes[:]
	if _, gray := img.(*image.Gray); gray {
		channels = planes[:1]
	}
	smooth := make([]*plane, len(channels))
	for c := range channels {
		smooth[c] = gaussianBlur(closing(channels[c], r), 1.4)
	}
	g := colorGradient(smooth)
	sc := newScorer(g)

	var color [3]*plane
	for c := range color {
		color[c] = smooth[min(c, len(smooth)-1)]
	}
	var cands []Quad
	cands = append(cands, houghCandidates(g, aw, ah)...)
	cands = append(cands, contourCandidates(g, aw, ah)...)
	cands = append(cands, segmentCandidates(color, aw, ah)...)

	best, bestScore := Quad{}, -1.0
	for _, q := range cands {
		if s := sc.score(q); s > bestScore {
			best, bestScore = q, s
		}
	}
	if bestScore < 0.42 {
		return Detection{Quad: FullQuad(w, h), Confidence: math.Max(0, bestScore)}
	}
	q := best.Scale(1/scale, 1/scale)
	if refineCorners {
		q = refine(src, q)
	}
	q = q.Clamp(w, h)
	return Detection{Quad: q, Found: true, Confidence: math.Min(1, bestScore)}
}

// borderCredit is the score of a side lying on the photo's edge.
const borderCredit = 0.8

// scorer rates candidate outlines against the photo's edges.
type scorer struct {
	g    *gradient
	norm float32 // a strong edge's gradient magnitude
}

func newScorer(g *gradient) *scorer {
	p := &plane{W: g.W, H: g.H, Pix: g.Mag}
	// Relative to the photo's strongest edges, but never so low that
	// texture or sensor noise counts as an edge: a Sobel response of
	// 0.2 is a brightness step of about 5%.
	n := max(p.percentile(0.97, 0.01), 0.2)
	return &scorer{g: g, norm: n}
}

// score returns how plausible q is as a document outline, in [0, 1],
// or a negative number for an impossible shape.
func (s *scorer) score(q Quad) float64 {
	w, h := float64(s.g.W), float64(s.g.H)
	if !q.Convex() {
		return -1
	}
	area := q.Area() / (w * h)
	if area < 0.06 || area > 1.15 {
		return -1
	}
	for _, a := range q.Angles() {
		if a < 50 || a > 130 {
			return -1
		}
	}
	margin := 0.08 * math.Max(w, h)
	for _, p := range q {
		if p.X < -margin || p.Y < -margin || p.X > w-1+margin || p.Y > h-1+margin {
			return -1
		}
	}
	// Opposite sides of a photographed page don't differ wildly.
	for i := 0; i < 2; i++ {
		a := q[i].Dist(q[i+1])
		b := q[i+2].Dist(q[(i+3)%4])
		if math.Min(a, b)/math.Max(a, b) < 0.45 {
			return -1
		}
	}
	var sum, low, real float64 = 0, 1, 0
	borders := 0
	for i := 0; i < 4; i++ {
		a, b := q[i], q[(i+1)%4]
		if onBorder(a, b, w, h) {
			// A page cut off by the frame: its side is the frame
			// itself, which proves nothing either way.
			borders++
			sum += borderCredit
			continue
		}
		v := s.side(a, b)
		sum += v
		real += v
		low = math.Min(low, v)
	}
	// At least two sides must be found in the photo, and they must be
	// convincing on their own.
	if borders > 2 || real/float64(4-borders) < 0.45 {
		return -1
	}
	mean := sum / 4
	areaScore := math.Sqrt(math.Max(0, math.Min(1, (area-0.05)/0.5)))
	skew := 0.0
	for _, a := range q.Angles() {
		skew += math.Abs(a - 90)
	}
	skewPenalty := math.Max(0, skew/4-12) / 60
	return 0.55*mean + 0.25*low + 0.2*areaScore - skewPenalty
}

// side measures how much of the segment a-b runs along a real edge
// whose gradient is perpendicular to it.
func (s *scorer) side(a, b Point) float64 {
	g := s.g
	w, h := float64(g.W), float64(g.H)
	d := b.Sub(a)
	length := d.Len()
	if length < 4 {
		return 0
	}
	dir := d.Mul(1 / length)
	n := Point{-dir.Y, dir.X}
	samples := max(24, int(length/2))
	total, outside := 0.0, 0
	for k := 0; k < samples; k++ {
		t := 0.06 + 0.88*float64(k)/float64(samples-1)
		p := a.Add(d.Mul(t))
		if p.X < 1 || p.Y < 1 || p.X > w-2 || p.Y > h-2 {
			outside++
			continue
		}
		best := 0.0
		for o := -2.0; o <= 2.0; o++ {
			q := p.Add(n.Mul(o))
			x, y := int(q.X+0.5), int(q.Y+0.5)
			if x < 0 || y < 0 || x >= g.W || y >= g.H {
				continue
			}
			i := y*g.W + x
			m := float64(g.Mag[i])
			if m == 0 {
				continue
			}
			align := math.Abs(float64(g.GX[i])*n.X+float64(g.GY[i])*n.Y) / m
			if align < 0.85 {
				continue
			}
			v := math.Min(1, m/float64(s.norm)*1.6) * align
			best = math.Max(best, v)
		}
		total += best
	}
	inside := samples - outside
	if inside == 0 {
		return 0.3
	}
	frac := float64(outside) / float64(samples)
	return (total/float64(inside))*(1-frac) + 0.3*frac
}

func onBorder(a, b Point, w, h float64) bool {
	const t = 2.5
	switch {
	case a.X < t && b.X < t, a.Y < t && b.Y < t:
		return true
	case a.X > w-1-t && b.X > w-1-t, a.Y > h-1-t && b.Y > h-1-t:
		return true
	}
	return false
}

// houghCandidates pairs the strongest roughly horizontal lines with
// the strongest roughly vertical ones (and the frame's own edges, for
// pages that run off the photo).
func houghCandidates(g *gradient, w, h int) []Quad {
	high := (&plane{W: w, H: h, Pix: g.Mag}).percentile(0.88, 0.01)
	edges := canny(g, high*0.4, high)
	minVotes := int(0.12 * float64(min(w, h)))
	lines := hough(edges, g, 28, minVotes)
	var horiz, vert []houghLine
	for _, l := range lines {
		deg := l.Theta * 180 / math.Pi
		if deg > 45 && deg < 135 {
			horiz = append(horiz, l)
		} else {
			vert = append(vert, l)
		}
	}
	if len(horiz) > 10 {
		horiz = horiz[:10]
	}
	if len(vert) > 10 {
		vert = vert[:10]
	}
	fw, fh := float64(w-1), float64(h-1)
	horiz = append(horiz,
		houghLine{Theta: math.Pi / 2, Rho: 0, Border: true},
		houghLine{Theta: math.Pi / 2, Rho: fh, Border: true})
	vert = append(vert,
		houghLine{Theta: 0, Rho: 0, Border: true},
		houghLine{Theta: 0, Rho: fw, Border: true})

	var out []Quad
	for i := 0; i < len(horiz); i++ {
		for j := i + 1; j < len(horiz); j++ {
			for k := 0; k < len(vert); k++ {
				for m := k + 1; m < len(vert); m++ {
					if q, ok := quadFromLines(horiz[i], horiz[j], vert[k], vert[m]); ok {
						out = append(out, q)
					}
				}
			}
		}
	}
	return out
}

func quadFromLines(h1, h2, v1, v2 houghLine) (Quad, bool) {
	var pts [4]Point
	ls := [4][2]houghLine{{h1, v1}, {h1, v2}, {h2, v2}, {h2, v1}}
	for i, pair := range ls {
		p, ok := intersect(pair[0].toLine(), pair[1].toLine())
		if !ok {
			return Quad{}, false
		}
		pts[i] = p
	}
	q := Quad(pts).Order()
	return q, q.Convex()
}

// contourCandidates fits outlines around connected edge regions.
func contourCandidates(g *gradient, w, h int) []Quad {
	var out []Quad
	for _, pct := range []float64{0.85, 0.93} {
		high := (&plane{W: w, H: h, Pix: g.Mag}).percentile(pct, 0.01)
		edges := dilate(canny(g, high*0.45, high), w, h, 1)
		comps := components(edges, w, h, (w+h)/4)
		sort.Slice(comps, func(i, j int) bool { return len(comps[i]) > len(comps[j]) })
		if len(comps) > 6 {
			comps = comps[:6]
		}
		for _, c := range comps {
			if q, ok := hullToQuad(convexHull(c)); ok {
				out = append(out, q)
			}
		}
	}
	return out
}

// segmentCandidates splits the photo into page and background by
// color - seeded with the middle of the frame as page and its border
// as background - and outlines the page region. It works at half the
// analysis size: it only proposes outlines, and the scorer judges
// them at full size.
func segmentCandidates(full [3]*plane, fw, fh int) []Quad {
	var ch [3]*plane
	for c := range ch {
		ch[c] = halve(full[c])
	}
	w, h := ch[0].W, ch[0].H
	quads := segmentHalf(ch, w, h)
	for i := range quads {
		quads[i] = quads[i].Scale(float64(fw)/float64(w), float64(fh)/float64(h))
	}
	return quads
}

// halve averages 2x2 blocks.
func halve(p *plane) *plane {
	w, h := p.W/2, p.H/2
	out := newPlane(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := 2*y*p.W + 2*x
			out.Pix[y*w+x] = (p.Pix[i] + p.Pix[i+1] + p.Pix[i+p.W] + p.Pix[i+p.W+1]) / 4
		}
	}
	return out
}

func segmentHalf(ch [3]*plane, w, h int) []Quad {
	n := w * h
	feat := func(i int) [3]float32 {
		r, g, b := ch[0].Pix[i], ch[1].Pix[i], ch[2].Pix[i]
		l := 0.299*r + 0.587*g + 0.114*b
		return [3]float32{l * 1.4, (r - g) * 1.0, ((r+g)/2 - b) * 1.0}
	}
	var fg, bg [3]float32
	var nf, nb float32
	bw, bh := max(2, w/20), max(2, h/20)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			f := feat(y*w + x)
			switch {
			case x > w*3/10 && x < w*7/10 && y > h*3/10 && y < h*7/10:
				for k := range fg {
					fg[k] += f[k]
				}
				nf++
			case x < bw || y < bh || x >= w-bw || y >= h-bh:
				for k := range bg {
					bg[k] += f[k]
				}
				nb++
			}
		}
	}
	for k := range fg {
		fg[k] /= nf
		bg[k] /= nb
	}
	dist := func(a, b [3]float32) float32 {
		d0, d1, d2 := a[0]-b[0], a[1]-b[1], a[2]-b[2]
		return d0*d0 + d1*d1 + d2*d2
	}
	if dist(fg, bg) < 0.004 {
		return nil
	}
	mask := make([]bool, n)
	for iter := 0; iter < 6; iter++ {
		var sf, sb [3]float32
		var cf, cb float32
		for i := 0; i < n; i++ {
			f := feat(i)
			if dist(f, fg) < dist(f, bg) {
				mask[i] = true
				for k := range sf {
					sf[k] += f[k]
				}
				cf++
			} else {
				mask[i] = false
				for k := range sb {
					sb[k] += f[k]
				}
				cb++
			}
		}
		if cf == 0 || cb == 0 {
			return nil
		}
		for k := range fg {
			fg[k] = sf[k] / cf
			bg[k] = sb[k] / cb
		}
	}
	// Clean up specks and holes.
	m := &plane{W: w, H: h, Pix: make([]float32, n)}
	for i, v := range mask {
		if v {
			m.Pix[i] = 1
		}
	}
	m = morph(morph(m, 1, false), 1, true) // open
	m = morph(morph(m, 2, true), 2, false) // close
	for i := range mask {
		mask[i] = m.Pix[i] > 0.5
	}
	comps := components(mask, w, h, n/20)
	var out []Quad
	for _, c := range comps {
		// Only the region's outline matters for the hull.
		var boundary []Point
		for _, p := range c {
			x, y := int(p.X), int(p.Y)
			if x == 0 || y == 0 || x == w-1 || y == h-1 ||
				!mask[y*w+x-1] || !mask[y*w+x+1] || !mask[(y-1)*w+x] || !mask[(y+1)*w+x] {
				boundary = append(boundary, p)
			}
		}
		if q, ok := hullToQuad(convexHull(boundary)); ok {
			out = append(out, q)
		}
	}
	return out
}
