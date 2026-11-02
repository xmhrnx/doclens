package scan

import (
	"image"
)

// PageParams says how to turn a photo into a finished page.
type PageParams struct {
	Quad     Quad
	Rotation int // clockwise quarter turns, after flattening
	Filter   Filter
	Adjust   Adjust
}

// Flatten cuts the page out of the photo and corrects the perspective.
func Flatten(photo image.Image, q Quad) image.Image {
	b := photo.Bounds()
	w, h := OutputSize(q, b.Dx(), b.Dy())
	return Warp(photo, inset(q, 0.004), w, h)
}

// Render produces the finished page from the photo: flattened,
// rotated and enhanced. Annotations are drawn separately with
// Annotate, so they can be edited without touching the scan.
func Render(photo image.Image, p PageParams) image.Image {
	flat := Flatten(photo, p.Quad)
	return Enhance(Rotate90(flat, p.Rotation), p.Filter, p.Adjust)
}

// BookHalves splits a book spread's outline into the outlines of its
// left and right pages, cutting along the line joining the midpoints
// of the top and bottom edges.
func BookHalves(q Quad) (Quad, Quad) {
	topMid := q[0].Add(q[1]).Mul(0.5)
	botMid := q[3].Add(q[2]).Mul(0.5)
	return Quad{q[0], topMid, botMid, q[3]}, Quad{topMid, q[1], q[2], botMid}
}

// inset pulls every corner toward the center by a fraction of the
// quad's size, so the page's very edge - where the background bleeds
// in - is left out.
func inset(q Quad, frac float64) Quad {
	c := q[0].Add(q[1]).Add(q[2]).Add(q[3]).Mul(0.25)
	for i := range q {
		q[i] = q[i].Add(c.Sub(q[i]).Mul(frac * 2))
	}
	return q
}
