// Package scan finds documents in photos and turns them into clean,
// flat page images: edge detection, perspective correction and
// enhancement filters. It has no UI dependencies.
package scan

import (
	"math"
	"sort"
)

// Point is a position in image pixels.
type Point struct{ X, Y float64 }

func (p Point) Add(q Point) Point          { return Point{p.X + q.X, p.Y + q.Y} }
func (p Point) Sub(q Point) Point          { return Point{p.X - q.X, p.Y - q.Y} }
func (p Point) Mul(s float64) Point        { return Point{p.X * s, p.Y * s} }
func (p Point) Dot(q Point) float64        { return p.X*q.X + p.Y*q.Y }
func (p Point) Cross(q Point) float64      { return p.X*q.Y - p.Y*q.X }
func (p Point) Len() float64               { return math.Hypot(p.X, p.Y) }
func (p Point) Dist(q Point) float64       { return p.Sub(q).Len() }
func (p Point) Scale(sx, sy float64) Point { return Point{p.X * sx, p.Y * sy} }

// Quad is a document outline: top-left, top-right, bottom-right and
// bottom-left corners, clockwise in image coordinates (y down).
type Quad [4]Point

// FullQuad is the outline of a whole w x h image.
func FullQuad(w, h int) Quad {
	fw, fh := float64(w-1), float64(h-1)
	return Quad{{0, 0}, {fw, 0}, {fw, fh}, {0, fh}}
}

// Scale returns q with every corner scaled.
func (q Quad) Scale(sx, sy float64) Quad {
	for i := range q {
		q[i] = q[i].Scale(sx, sy)
	}
	return q
}

// Area is the quad's area (positive for a clockwise quad in y-down
// coordinates).
func (q Quad) Area() float64 {
	a := 0.0
	for i := range q {
		a += q[i].Cross(q[(i+1)%4])
	}
	return a / 2
}

// Convex reports whether the quad is strictly convex and clockwise.
func (q Quad) Convex() bool {
	for i := range q {
		a, b, c := q[i], q[(i+1)%4], q[(i+2)%4]
		if b.Sub(a).Cross(c.Sub(b)) <= 0 {
			return false
		}
	}
	return true
}

// Angles returns the interior angles in degrees.
func (q Quad) Angles() [4]float64 {
	var out [4]float64
	for i := range q {
		p, c, n := q[(i+3)%4], q[i], q[(i+1)%4]
		u, v := p.Sub(c), n.Sub(c)
		cos := u.Dot(v) / (u.Len()*v.Len() + 1e-12)
		out[i] = math.Acos(math.Max(-1, math.Min(1, cos))) * 180 / math.Pi
	}
	return out
}

// Order returns the quad's corners as top-left, top-right,
// bottom-right, bottom-left, whatever order they were given in.
func (q Quad) Order() Quad {
	c := Point{}
	for _, p := range q {
		c = c.Add(p)
	}
	c = c.Mul(0.25)
	pts := q
	sort.Slice(pts[:], func(i, j int) bool {
		return math.Atan2(pts[i].Y-c.Y, pts[i].X-c.X) < math.Atan2(pts[j].Y-c.Y, pts[j].X-c.X)
	})
	// Sorted by angle with y down is clockwise; start at the corner
	// with the smallest x+y.
	start := 0
	for i := range pts {
		if pts[i].X+pts[i].Y < pts[start].X+pts[start].Y {
			start = i
		}
	}
	var out Quad
	for i := range out {
		out[i] = pts[(start+i)%4]
	}
	return out
}

// Clamp keeps every corner inside a w x h image.
func (q Quad) Clamp(w, h int) Quad {
	for i := range q {
		q[i].X = math.Max(0, math.Min(float64(w-1), q[i].X))
		q[i].Y = math.Max(0, math.Min(float64(h-1), q[i].Y))
	}
	return q
}

// line is a 2D line in normal form: N·p = D with |N| = 1.
type line struct {
	N Point
	D float64
}

func lineThrough(a, b Point) line {
	d := b.Sub(a)
	n := Point{-d.Y, d.X}
	l := n.Len()
	if l == 0 {
		return line{N: Point{1, 0}, D: a.X}
	}
	n = n.Mul(1 / l)
	return line{N: n, D: n.Dot(a)}
}

// intersect returns the intersection of two lines, false when they're
// (nearly) parallel.
func intersect(a, b line) (Point, bool) {
	det := a.N.X*b.N.Y - a.N.Y*b.N.X
	if math.Abs(det) < 1e-6 {
		return Point{}, false
	}
	return Point{
		X: (a.D*b.N.Y - a.N.Y*b.D) / det,
		Y: (a.N.X*b.D - a.D*b.N.X) / det,
	}, true
}

// convexHull returns the convex hull of pts, clockwise in y-down
// coordinates (Andrew's monotone chain).
func convexHull(pts []Point) []Point {
	if len(pts) < 3 {
		return nil
	}
	p := append([]Point(nil), pts...)
	sort.Slice(p, func(i, j int) bool {
		if p[i].X != p[j].X {
			return p[i].X < p[j].X
		}
		return p[i].Y < p[j].Y
	})
	hull := make([]Point, 0, 2*len(p))
	for _, pt := range p {
		for len(hull) >= 2 && hull[len(hull)-1].Sub(hull[len(hull)-2]).Cross(pt.Sub(hull[len(hull)-1])) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, pt)
	}
	lower := len(hull) + 1
	for i := len(p) - 2; i >= 0; i-- {
		pt := p[i]
		for len(hull) >= lower && hull[len(hull)-1].Sub(hull[len(hull)-2]).Cross(pt.Sub(hull[len(hull)-1])) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, pt)
	}
	hull = hull[:len(hull)-1]
	// The chain above is counter-clockwise in y-up terms, which is
	// clockwise on screen (y down) only after reversing.
	if polyArea(hull) < 0 {
		for i, j := 0, len(hull)-1; i < j; i, j = i+1, j-1 {
			hull[i], hull[j] = hull[j], hull[i]
		}
	}
	return hull
}

func polyArea(p []Point) float64 {
	a := 0.0
	for i := range p {
		a += p[i].Cross(p[(i+1)%len(p)])
	}
	return a / 2
}

// hullToQuad reduces a convex polygon to the quadrilateral around it
// that adds the least area: each step removes the edge whose
// neighbours, extended until they meet, add the smallest triangle.
// Circumscribing (rather than dropping vertices) restores corners
// that a finger or a fold cut off.
func hullToQuad(hull []Point) (Quad, bool) {
	p := append([]Point(nil), hull...)
	p = dedupe(p)
	for len(p) > 4 {
		n := len(p)
		best, bestCost := -1, math.Inf(1)
		var bestPt Point
		for i := 0; i < n; i++ {
			// Edge p[i]-p[i+1]; neighbours p[i-1]-p[i] and p[i+1]-p[i+2].
			a, b := p[(i-1+n)%n], p[i]
			c, d := p[(i+1)%n], p[(i+2)%n]
			x, ok := intersect(lineThrough(a, b), lineThrough(c, d))
			if !ok {
				continue
			}
			// The new corner must lie beyond the removed edge.
			if b.Sub(x).Dot(b.Sub(a)) > 0 || c.Sub(x).Dot(c.Sub(d)) > 0 {
				continue
			}
			cost := math.Abs(x.Sub(b).Cross(c.Sub(b))) / 2
			if cost < bestCost {
				best, bestCost, bestPt = i, cost, x
			}
		}
		if best < 0 {
			// No valid extension: drop the vertex with the smallest
			// triangle instead.
			bi, bc := 0, math.Inf(1)
			for i := 0; i < n; i++ {
				a, b, c := p[(i-1+n)%n], p[i], p[(i+1)%n]
				if t := math.Abs(b.Sub(a).Cross(c.Sub(a))); t < bc {
					bi, bc = i, t
				}
			}
			p = append(p[:bi], p[bi+1:]...)
			continue
		}
		next := make([]Point, 0, n-1)
		for i := 0; i < n; i++ {
			switch {
			case i == best:
				next = append(next, bestPt)
			case i == (best+1)%n:
			default:
				next = append(next, p[i])
			}
		}
		p = next
	}
	if len(p) != 4 {
		return Quad{}, false
	}
	q := Quad{p[0], p[1], p[2], p[3]}.Order()
	return q, q.Convex()
}

func dedupe(p []Point) []Point {
	out := p[:0]
	for i, pt := range p {
		if i > 0 && pt.Dist(out[len(out)-1]) < 0.5 {
			continue
		}
		out = append(out, pt)
	}
	if len(out) > 1 && out[0].Dist(out[len(out)-1]) < 0.5 {
		out = out[:len(out)-1]
	}
	return out
}
