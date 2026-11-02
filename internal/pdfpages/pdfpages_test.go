package pdfpages

import "testing"

func TestRenderSize(t *testing.T) {
	cases := []struct {
		pw, ph, dpi float64
		max         int
		w, h        int
	}{
		{595.28, 841.89, 72, 5000, 595, 842},    // A4 at 72 dpi
		{612, 792, 300, 10000, 2550, 3300},      // Letter at 300 dpi
		{595.28, 841.89, 300, 3000, 2121, 3000}, // capped by the long side
		{841.89, 595.28, 300, 3000, 3000, 2121}, // landscape
	}
	for _, c := range cases {
		w, h, ok := renderSize(c.pw, c.ph, c.dpi, c.max)
		if !ok || w != c.w || h != c.h {
			t.Errorf("renderSize(%v, %v, %v, %d) = %d, %d, %v; want %d, %d", c.pw, c.ph, c.dpi, c.max, w, h, ok, c.w, c.h)
		}
	}
	if _, _, ok := renderSize(0, 100, 72, 100); ok {
		t.Error("an empty page should fail")
	}
}

func TestUnsupportedPlatform(t *testing.T) {
	if Supported() {
		t.Skip("this platform renders PDFs")
	}
	if _, err := Open("x.pdf"); err != ErrNotSupported {
		t.Errorf("Open = %v, want ErrNotSupported", err)
	}
}
