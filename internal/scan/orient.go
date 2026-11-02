package scan

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	_ "image/gif" // decoders for imported pictures
	"image/jpeg"
	_ "image/png"
	"io"

	_ "golang.org/x/image/webp"
)

// Rotate90 turns an image clockwise by quarter turns (0-3).
func Rotate90(img image.Image, turns int) image.Image {
	turns = ((turns % 4) + 4) % 4
	if turns == 0 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	switch src := img.(type) {
	case *image.Gray:
		var out *image.Gray
		if turns == 2 {
			out = image.NewGray(image.Rect(0, 0, w, h))
		} else {
			out = image.NewGray(image.Rect(0, 0, h, w))
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				nx, ny := rotXY(x, y, w, h, turns)
				out.Pix[ny*out.Stride+nx] = src.Pix[(y)*src.Stride+x]
			}
		}
		return out
	default:
		s := toRGBA(img)
		var out *image.RGBA
		if turns == 2 {
			out = image.NewRGBA(image.Rect(0, 0, w, h))
		} else {
			out = image.NewRGBA(image.Rect(0, 0, h, w))
		}
		parallelRows(h, func(y0, y1 int) {
			for y := y0; y < y1; y++ {
				for x := 0; x < w; x++ {
					nx, ny := rotXY(x, y, w, h, turns)
					copy(out.Pix[ny*out.Stride+nx*4:ny*out.Stride+nx*4+4], s.Pix[y*s.Stride+x*4:])
				}
			}
		})
		return out
	}
}

func rotXY(x, y, w, h, turns int) (int, int) {
	switch turns {
	case 1:
		return h - 1 - y, x
	case 2:
		return w - 1 - x, h - 1 - y
	default:
		return y, w - 1 - x
	}
}

// RotateQuad maps a quad in an image to the same outline after the
// image is turned clockwise by quarter turns.
func RotateQuad(q Quad, w, h, turns int) Quad {
	turns = ((turns % 4) + 4) % 4
	for i := range q {
		x, y := q[i].X, q[i].Y
		switch turns {
		case 1:
			q[i] = Point{float64(h-1) - y, x}
		case 2:
			q[i] = Point{float64(w-1) - x, float64(h-1) - y}
		case 3:
			q[i] = Point{y, float64(w-1) - x}
		}
	}
	return q.Order()
}

// Decode reads an image (JPEG, PNG, GIF or WebP) and turns it upright
// according to its EXIF orientation, as cameras store photos sideways
// and record how to show them.
func Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return applyOrientation(img, exifOrientation(data)), nil
}

// applyOrientation undoes one of the 8 EXIF orientations.
func applyOrientation(img image.Image, o int) image.Image {
	switch o {
	case 2:
		return flipH(img)
	case 3:
		return Rotate90(img, 2)
	case 4:
		return flipH(Rotate90(img, 2))
	case 5:
		return flipH(Rotate90(img, 1))
	case 6:
		return Rotate90(img, 1)
	case 7:
		return flipH(Rotate90(img, 3))
	case 8:
		return Rotate90(img, 3)
	}
	return img
}

func flipH(img image.Image) image.Image {
	s := toRGBA(img)
	w, h := s.Rect.Dx(), s.Rect.Dy()
	out := image.NewRGBA(s.Rect)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			copy(out.Pix[y*out.Stride+(w-1-x)*4:y*out.Stride+(w-1-x)*4+4], s.Pix[y*s.Stride+x*4:])
		}
	}
	return out
}

// exifOrientation returns the orientation tag of a JPEG's EXIF block,
// 1 when there is none.
func exifOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		size := int(binary.BigEndian.Uint16(data[i+2:]))
		if marker == 0xDA || size < 2 || i+2+size > len(data) {
			return 1
		}
		seg := data[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for k := 0; k < n; k++ {
		e := off + 2 + 12*k
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			v := int(bo.Uint16(t[e+8:]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// EncodeJPEG writes img as a JPEG; grayscale images stay single
// channel, which makes them about a third of the size.
func EncodeJPEG(w io.Writer, img image.Image, quality int) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}

// Downscale shrinks img so its longer side is at most maxSide, with
// area averaging.
func Downscale(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	if max(b.Dx(), b.Dy()) <= maxSide {
		return img
	}
	if g, ok := img.(*image.Gray); ok {
		rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Rect, g, b.Min, draw.Src)
		img = rgba
	}
	p, _ := downscaleRGB(toRGBA(img), maxSide)
	out := image.NewRGBA(image.Rect(0, 0, p[0].W, p[0].H))
	for i := range p[0].Pix {
		out.Pix[i*4] = to8(p[0].Pix[i])
		out.Pix[i*4+1] = to8(p[1].Pix[i])
		out.Pix[i*4+2] = to8(p[2].Pix[i])
		out.Pix[i*4+3] = 255
	}
	return out
}
