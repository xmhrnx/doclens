package pdf

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// textLayer returns content stream operators that draw the words
// invisibly (text render mode 3) over the image placed at x, y with
// size iw x ih: readers find, select and copy them, nobody sees them.
func textLayer(words []Word, x, y, iw, ih float64) []byte {
	if len(words) == 0 {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("\nBT 3 Tr")
	for _, w := range words {
		enc := winAnsi(w.Text)
		if enc == "" || w.X1 <= w.X0 || w.Y1 <= w.Y0 {
			continue
		}
		boxW, boxH := (w.X1-w.X0)*iw, (w.Y1-w.Y0)*ih
		size := boxH * 0.9
		// Stretch the word across its box, so a selection covers the
		// word on the image.
		natural := helveticaWidth(enc) * size
		scale := 100.0
		if natural > 0 {
			scale = 100 * boxW / natural
		}
		// The baseline sits about a fifth of the box above its bottom.
		bx, by := x+w.X0*iw, y+(1-w.Y1)*ih+boxH*0.2
		fmt.Fprintf(&b, " /F0 %s Tf %s Tz 1 0 0 1 %s %s Tm (%s) Tj", num(size), num(scale), num(bx), num(by), escape(enc))
	}
	b.WriteString(" ET")
	return b.Bytes()
}

// winAnsi encodes text for the standard Helvetica font, dropping
// characters it doesn't have.
func winAnsi(s string) string {
	enc := charmap.Windows1252.NewEncoder()
	var b strings.Builder
	for _, r := range s {
		if out, err := enc.String(string(r)); err == nil {
			b.WriteString(out)
		}
	}
	return b.String()
}

func escape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '(', ')', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			if c < 32 || c >= 127 {
				fmt.Fprintf(&b, "\\%03o", c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	return b.String()
}

// helveticaWidth is the width of s in Helvetica at size 1, from the
// font's metrics for ASCII and an average elsewhere.
func helveticaWidth(s string) float64 {
	w := 0.0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 32 && c < 127 {
			w += float64(helvetica[c-32]) / 1000
		} else {
			w += 0.556
		}
	}
	return w
}

// helvetica holds Helvetica's advance widths for characters 32 to 126,
// in thousandths of the font size (Adobe's AFM).
var helvetica = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}
