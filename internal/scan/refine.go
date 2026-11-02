package scan

import (
	"image"
	"math"
	"sort"
)

const refineSide = 1600

// refine moves each side of q onto the exact edge in a high resolution
// copy of the photo: it samples the edge across each side, fits a line
// through the samples while ignoring outliers, and intersects the
// lines for sub-pixel corners. Sides it can't place stay where they
// were.
func refine(src *image.RGBA, q Quad) Quad {
	planes, scale := downscaleRGB(src, refineSide)
	l := gaussianBlur(planes.luma(), 1.2)
	ch := []*plane{l, gaussianBlur(planes[0], 1.2), gaussianBlur(planes[2], 1.2)}
	g := colorGradient(ch)
	sq := q.Scale(scale, scale)
	w, h := float64(g.W), float64(g.H)

	var lines [4]line
	for i := 0; i < 4; i++ {
		a, b := sq[i], sq[(i+1)%4]
		lines[i] = lineThrough(a, b)
		if onBorder(a, b, w, h) {
			continue
		}
		if fit, ok := fitSide(g, a, b, math.Max(w, h)); ok {
			lines[i] = fit
		}
	}
	var out Quad
	diag := math.Hypot(w, h)
	for i := 0; i < 4; i++ {
		p, ok := intersect(lines[(i+3)%4], lines[i])
		if !ok || p.Dist(sq[i]) > 0.04*diag {
			p = sq[i]
		}
		out[i] = p
	}
	if !out.Convex() {
		return q
	}
	return out.Scale(1/scale, 1/scale)
}

// fitSide finds the edge points along a-b and fits a line through
// them.
func fitSide(g *gradient, a, b Point, long float64) (line, bool) {
	d := b.Sub(a)
	length := d.Len()
	if length < 20 {
		return line{}, false
	}
	dir := d.Mul(1 / length)
	n := Point{-dir.Y, dir.X}
	reach := math.Max(6, 0.012*long)
	const samples = 60
	var pts []Point
	for k := 0; k < samples; k++ {
		t := 0.08 + 0.84*float64(k)/float64(samples-1)
		p := a.Add(d.Mul(t))
		bestO, bestV := 0.0, 0.0
		var vals []float64
		var offs []float64
		for o := -reach; o <= reach; o++ {
			q := p.Add(n.Mul(o))
			x, y := int(q.X+0.5), int(q.Y+0.5)
			if x < 1 || y < 1 || x >= g.W-1 || y >= g.H-1 {
				vals = append(vals, 0)
				offs = append(offs, o)
				continue
			}
			i := y*g.W + x
			m := float64(g.Mag[i])
			v := 0.0
			if m > 0 {
				align := math.Abs(float64(g.GX[i])*n.X+float64(g.GY[i])*n.Y) / m
				if align > 0.9 {
					v = m * align
				}
			}
			// Prefer edges near the coarse estimate.
			v *= 1 - 0.35*math.Abs(o)/reach
			vals = append(vals, v)
			offs = append(offs, o)
			if v > bestV {
				bestV, bestO = v, o
			}
		}
		if bestV <= 0 {
			continue
		}
		// Parabolic interpolation around the peak for sub-pixel
		// accuracy.
		idx := int(bestO + reach)
		if idx > 0 && idx < len(vals)-1 {
			l, c, r := vals[idx-1], vals[idx], vals[idx+1]
			if den := l - 2*c + r; den < 0 {
				bestO = offs[idx] + 0.5*(l-r)/den
			}
		}
		pts = append(pts, p.Add(n.Mul(bestO)))
	}
	if len(pts) < samples/3 {
		return line{}, false
	}
	// Fit, drop the worst residuals, fit again.
	for iter := 0; iter < 3; iter++ {
		l, ok := fitLine(pts)
		if !ok {
			return line{}, false
		}
		res := make([]float64, len(pts))
		for i, p := range pts {
			res[i] = math.Abs(l.N.Dot(p) - l.D)
		}
		sorted := append([]float64(nil), res...)
		sort.Float64s(sorted)
		med := sorted[len(sorted)/2]
		limit := math.Max(1.0, 2.5*med)
		kept := pts[:0:0]
		for i, p := range pts {
			if res[i] <= limit {
				kept = append(kept, p)
			}
		}
		if len(kept) < samples/3 {
			return l, true
		}
		if len(kept) == len(pts) {
			return l, true
		}
		pts = kept
	}
	return fitLine(pts)
}

// fitLine is a total least squares line fit.
func fitLine(pts []Point) (line, bool) {
	if len(pts) < 2 {
		return line{}, false
	}
	var c Point
	for _, p := range pts {
		c = c.Add(p)
	}
	c = c.Mul(1 / float64(len(pts)))
	var sxx, sxy, syy float64
	for _, p := range pts {
		d := p.Sub(c)
		sxx += d.X * d.X
		sxy += d.X * d.Y
		syy += d.Y * d.Y
	}
	// Direction is the principal eigenvector of the covariance.
	theta := 0.5 * math.Atan2(2*sxy, sxx-syy)
	dir := Point{math.Cos(theta), math.Sin(theta)}
	n := Point{-dir.Y, dir.X}
	return line{N: n, D: n.Dot(c)}, true
}
