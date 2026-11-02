package library

import "github.com/mehran/scanly/internal/scan"

// Rotate turns a finished page a quarter turn clockwise (or back, with
// -1), keeping its drawings and signatures where they were on the
// paper.
func (p *Page) Rotate(turns int) {
	turns = ((turns % 4) + 4) % 4
	if turns == 0 {
		return
	}
	// The page's width and height before this rotation.
	w, h := scan.OutputSize(p.Quad, p.PhotoW, p.PhotoH)
	if p.Rotation%2 == 1 {
		w, h = h, w
	}
	for t := 0; t < turns; t++ {
		fw, fh := float64(w), float64(h)
		for i := range p.Strokes {
			pts := make([]scan.Point, len(p.Strokes[i].Points))
			for k, pt := range p.Strokes[i].Points {
				pts[k] = scan.Point{X: 1 - pt.Y, Y: pt.X}
			}
			p.Strokes[i].Points = pts
			// Widths are fractions of the page width, which changes.
			p.Strokes[i].Width *= fw / fh
		}
		for i := range p.Stamps {
			s := &p.Stamps[i]
			// Keep the signature upright and its size, centered on the
			// same spot of the paper. Lengths below are in pixels.
			sw := s.W * fw
			sh := sw / max(s.Aspect, 0.01)
			cx, cy := s.X*fw+sw/2, s.Y*fh+sh/2
			ncx, ncy := fh-cy, cx // the page is now fh wide, fw tall
			s.W = sw / fh
			s.X = (ncx - sw/2) / fh
			s.Y = (ncy - sh/2) / fw
		}
		w, h = h, w
	}
	p.Rotation = (p.Rotation + turns) % 4
	p.Touch()
}
