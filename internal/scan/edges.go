package scan

import (
	"math"
)

// gradient holds per-pixel image gradients: the direction and size of
// the strongest change across the color channels.
type gradient struct {
	W, H   int
	GX, GY []float32
	Mag    []float32
}

// colorGradient runs a Sobel operator on every channel and keeps, per
// pixel, the channel that changes most - so a blue page on a green
// desk has an edge even where their brightness matches.
func colorGradient(ch []*plane) *gradient {
	w, h := ch[0].W, ch[0].H
	g := &gradient{W: w, H: h, GX: make([]float32, w*h), GY: make([]float32, w*h), Mag: make([]float32, w*h)}
	parallelRows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < w; x++ {
				var bx, by, bm float32
				for _, p := range ch {
					tl, t, tr := p.at(x-1, y-1), p.at(x, y-1), p.at(x+1, y-1)
					l, r := p.at(x-1, y), p.at(x+1, y)
					bl, b, br := p.at(x-1, y+1), p.at(x, y+1), p.at(x+1, y+1)
					gx := (tr + 2*r + br) - (tl + 2*l + bl)
					gy := (bl + 2*b + br) - (tl + 2*t + tr)
					if m := gx*gx + gy*gy; m > bm {
						bx, by, bm = gx, gy, m
					}
				}
				i := y*w + x
				g.GX[i], g.GY[i], g.Mag[i] = bx, by, float32(math.Sqrt(float64(bm)))
			}
		}
	})
	return g
}

// canny returns a thin binary edge map with hysteresis thresholds.
func canny(g *gradient, low, high float32) []bool {
	w, h := g.W, g.H
	thin := make([]float32, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			m := g.Mag[i]
			if m < low {
				continue
			}
			// Quantize the gradient direction to 4 neighbours pairs.
			ax, ay := math.Abs(float64(g.GX[i])), math.Abs(float64(g.GY[i]))
			var a, b float32
			switch {
			case ay <= ax*0.4142:
				a, b = g.Mag[i-1], g.Mag[i+1]
			case ax <= ay*0.4142:
				a, b = g.Mag[i-w], g.Mag[i+w]
			case (g.GX[i] > 0) == (g.GY[i] > 0):
				a, b = g.Mag[i-w-1], g.Mag[i+w+1]
			default:
				a, b = g.Mag[i-w+1], g.Mag[i+w-1]
			}
			if m >= a && m > b {
				thin[i] = m
			}
		}
	}
	edges := make([]bool, w*h)
	stack := make([]int, 0, 1024)
	for i, m := range thin {
		if m < high || edges[i] {
			continue
		}
		edges[i] = true
		stack = append(stack[:0], i)
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := j%w, j/w
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					k := ny*w + nx
					if !edges[k] && thin[k] >= low {
						edges[k] = true
						stack = append(stack, k)
					}
				}
			}
		}
	}
	return edges
}

// dilate grows a binary map by r pixels (square).
func dilate(m []bool, w, h, r int) []bool {
	tmp := make([]bool, len(m))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !m[y*w+x] {
				continue
			}
			for dx := -r; dx <= r; dx++ {
				if nx := x + dx; nx >= 0 && nx < w {
					tmp[y*w+nx] = true
				}
			}
		}
	}
	out := make([]bool, len(m))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !tmp[y*w+x] {
				continue
			}
			for dy := -r; dy <= r; dy++ {
				if ny := y + dy; ny >= 0 && ny < h {
					out[ny*w+x] = true
				}
			}
		}
	}
	return out
}

// components labels 8-connected regions of a binary map and returns,
// for each region, its pixel coordinates.
func components(m []bool, w, h, minPixels int) [][]Point {
	seen := make([]bool, len(m))
	var out [][]Point
	stack := make([]int, 0, 4096)
	for i := range m {
		if !m[i] || seen[i] {
			continue
		}
		var pts []Point
		seen[i] = true
		stack = append(stack[:0], i)
		for len(stack) > 0 {
			j := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := j%w, j/w
			pts = append(pts, Point{float64(x), float64(y)})
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= w || ny >= h {
						continue
					}
					k := ny*w + nx
					if m[k] && !seen[k] {
						seen[k] = true
						stack = append(stack, k)
					}
				}
			}
		}
		if len(pts) >= minPixels {
			out = append(out, pts)
		}
	}
	return out
}

// houghLine is a detected straight edge: x·cos(θ) + y·sin(θ) = ρ.
type houghLine struct {
	Theta, Rho float64
	Votes      int
	Border     bool // one of the image's own edges
}

func (l houghLine) toLine() line {
	return line{N: Point{math.Cos(l.Theta), math.Sin(l.Theta)}, D: l.Rho}
}

// hough finds the strongest straight lines in an edge map. Each edge
// pixel votes only for angles near its own gradient direction, which
// keeps the accumulator clean and fast.
func hough(edges []bool, g *gradient, maxLines int, minVotes int) []houghLine {
	w, h := g.W, g.H
	const thetaBins = 180
	diag := int(math.Ceil(math.Hypot(float64(w), float64(h))))
	rhoBins := 2*diag + 1
	acc := make([]int32, thetaBins*rhoBins)
	var cosT, sinT [thetaBins]float64
	for t := 0; t < thetaBins; t++ {
		a := float64(t) * math.Pi / thetaBins
		cosT[t], sinT[t] = math.Cos(a), math.Sin(a)
	}
	const spread = 4
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if !edges[i] {
				continue
			}
			a := math.Atan2(float64(g.GY[i]), float64(g.GX[i]))
			if a < 0 {
				a += math.Pi
			}
			c := int(math.Round(a*thetaBins/math.Pi)) % thetaBins
			for d := -spread; d <= spread; d++ {
				// A wrapped angle needs no special care: ρ is computed
				// with its own cosine and sine.
				t := (c + d + thetaBins) % thetaBins
				r := float64(x)*cosT[t] + float64(y)*sinT[t]
				ri := int(math.Round(r)) + diag
				acc[t*rhoBins+ri]++
			}
		}
	}
	// Non-maximum suppression in a (θ, ρ) window.
	const tw, rw = 6, 10
	var lines []houghLine
	for t := 0; t < thetaBins; t++ {
		for r := 0; r < rhoBins; r++ {
			v := acc[t*rhoBins+r]
			if int(v) < minVotes {
				continue
			}
			peak := true
		search:
			for dt := -tw; dt <= tw; dt++ {
				tt := t + dt
				rr0 := r
				if tt < 0 {
					tt += thetaBins
					rr0 = 2*diag - r // θ wraps: ρ changes sign
				} else if tt >= thetaBins {
					tt -= thetaBins
					rr0 = 2*diag - r
				}
				for dr := -rw; dr <= rw; dr++ {
					rr := rr0 + dr
					if rr < 0 || rr >= rhoBins || (dt == 0 && dr == 0) {
						continue
					}
					o := acc[tt*rhoBins+rr]
					if o > v || (o == v && (tt*rhoBins+rr) < t*rhoBins+r) {
						peak = false
						break search
					}
				}
			}
			if peak {
				lines = append(lines, houghLine{
					Theta: float64(t) * math.Pi / thetaBins,
					Rho:   float64(r - diag),
					Votes: int(v),
				})
			}
		}
	}
	// Strongest first.
	for i := 1; i < len(lines); i++ {
		for j := i; j > 0 && lines[j].Votes > lines[j-1].Votes; j-- {
			lines[j], lines[j-1] = lines[j-1], lines[j]
		}
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}
