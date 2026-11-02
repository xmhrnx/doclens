package library

import (
	"math"
	"testing"

	"github.com/xmhrnx/doclens/internal/scan"
)

func TestRotateKeepsAnnotations(t *testing.T) {
	p := &Page{PhotoW: 1000, PhotoH: 1414, Quad: scan.FullQuad(1000, 1414)}
	p.Strokes = []scan.Stroke{{Points: []scan.Point{{X: 0.1, Y: 0.2}}, Width: 0.01}}
	p.Stamps = []StampRef{{X: 0.6, Y: 0.8, W: 0.3, Aspect: 3}}
	orig := *p.Clone()
	p.Rotate(1)
	// A quarter turn clockwise: the left edge becomes the top.
	if got := p.Strokes[0].Points[0]; math.Abs(got.X-0.8) > 1e-9 || math.Abs(got.Y-0.1) > 1e-9 {
		t.Fatalf("after one turn the stroke is at %v, want {0.8 0.1}", got)
	}
	if p.Rotation != 1 || p.Rev != 1 {
		t.Fatalf("rotation %d rev %d", p.Rotation, p.Rev)
	}
	for i := 0; i < 3; i++ {
		p.Rotate(1)
	}
	if p.Rotation != 0 {
		t.Fatalf("rotation %d", p.Rotation)
	}
	got, want := p.Strokes[0].Points[0], orig.Strokes[0].Points[0]
	if math.Abs(got.X-want.X) > 1e-9 || math.Abs(got.Y-want.Y) > 1e-9 {
		t.Errorf("stroke moved: %v want %v", got, want)
	}
	if math.Abs(p.Strokes[0].Width-0.01) > 1e-9 {
		t.Errorf("width %v", p.Strokes[0].Width)
	}
	s, o := p.Stamps[0], orig.Stamps[0]
	if math.Abs(s.X-o.X) > 1e-6 || math.Abs(s.Y-o.Y) > 1e-6 || math.Abs(s.W-o.W) > 1e-6 {
		t.Errorf("stamp %+v want %+v", s, o)
	}
}
