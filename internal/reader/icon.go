package reader

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	_ "image/gif"  // icons come as GIF
	_ "image/jpeg" // and as JPEG
	"image/png"
	"io"
)

// IconColor is the colour a site's icon mostly is, for the mark before its
// tab (user, 2026-10-28): the pixels that show, averaged, lifted when too
// dark to see on the panel. ok is false for an icon that cannot be had or
// read — an SVG one, say.
func (c *Client) IconColor(ctx context.Context, url string) (r, g, b uint8, ok bool) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return 0, 0, 0, false
	}
	if resp.IsPage() {
		return 0, 0, 0, false
	}
	raw, err := resp.Bytes(1 << 20)
	if err != nil {
		return 0, 0, 0, false
	}
	img, err := decodeIcon(raw)
	if err != nil {
		return 0, 0, 0, false
	}
	return average(img)
}

// Bytes reads a download whole, up to max bytes, and lets it go.
func (r *Response) Bytes(max int64) ([]byte, error) {
	if r.download == nil {
		return r.Body, nil
	}
	defer r.download.Close()
	return io.ReadAll(io.LimitReader(r.download, max))
}

// decodeIcon reads an icon: an ICO, or any image Go reads (PNG, GIF,
// JPEG).
func decodeIcon(b []byte) (image.Image, error) {
	if len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0 {
		return decodeICO(b)
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	return img, err
}

var pngSig = []byte("\x89PNG\r\n\x1a\n")

// decodeICO reads the biggest image of an ICO: a PNG inside it, or a
// bitmap — 32-bit, or of 24 bits or a palette, as older icons are
// (Wikipedia's is 16 colours).
func decodeICO(b []byte) (image.Image, error) {
	if len(b) < 6 {
		return nil, errors.New("ico: short")
	}
	n := int(binary.LittleEndian.Uint16(b[4:]))
	best, bestW := -1, 0
	for i := 0; i < n && 6+16*i+16 <= len(b); i++ {
		w := int(b[6+16*i])
		if w == 0 {
			w = 256
		}
		if w > bestW {
			best, bestW = i, w
		}
	}
	if best < 0 {
		return nil, errors.New("ico: no image")
	}
	e := b[6+16*best:]
	size, off := int(binary.LittleEndian.Uint32(e[8:])), int(binary.LittleEndian.Uint32(e[12:]))
	if off < 0 || size < 0 || off+size > len(b) {
		return nil, errors.New("ico: out of range")
	}
	data := b[off : off+size]
	if bytes.HasPrefix(data, pngSig) {
		return png.Decode(bytes.NewReader(data))
	}
	if len(data) < 40 {
		return nil, errors.New("ico: short bitmap")
	}
	head := int(binary.LittleEndian.Uint32(data[0:]))
	w := int(int32(binary.LittleEndian.Uint32(data[4:])))
	h := int(int32(binary.LittleEndian.Uint32(data[8:]))) / 2 // the AND mask's height is in it too
	bpp := int(binary.LittleEndian.Uint16(data[14:]))
	if bpp != 1 && bpp != 4 && bpp != 8 && bpp != 24 && bpp != 32 || w <= 0 || h <= 0 || head < 40 {
		return nil, errors.New("ico: not a bitmap webu reads")
	}
	// The colours a 1-, 4- or 8-bit bitmap's pixels index, after the header.
	var palette []color.NRGBA
	if bpp <= 8 {
		n := int(binary.LittleEndian.Uint32(data[32:]))
		if n == 0 || n > 1<<bpp {
			n = 1 << bpp
		}
		if head+4*n > len(data) {
			return nil, errors.New("ico: short palette")
		}
		for i := range n {
			p := data[head+4*i:]
			palette = append(palette, color.NRGBA{R: p[2], G: p[1], B: p[0], A: 255})
		}
	}
	// Rows are padded to 4 bytes, the bottom one first; after them the AND
	// mask, a bit a pixel, set where the pixel does not show.
	stride, maskStride := (w*bpp+31)/32*4, (w+31)/32*4
	pix := head + 4*len(palette)
	mask := pix + stride*h
	if mask > len(data) {
		return nil, errors.New("ico: short bitmap")
	}
	hasMask := bpp != 32 && mask+maskStride*h <= len(data) // 32-bit has its own alpha
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := data[pix+(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			var c color.NRGBA
			switch bpp {
			case 32:
				c = color.NRGBA{R: row[4*x+2], G: row[4*x+1], B: row[4*x], A: row[4*x+3]}
			case 24:
				c = color.NRGBA{R: row[3*x+2], G: row[3*x+1], B: row[3*x], A: 255}
			default:
				bit := x * bpp
				if i := int(row[bit/8]>>(8-bpp-bit%8)) & (1<<bpp - 1); i < len(palette) {
					c = palette[i]
				}
			}
			if hasMask && data[mask+(h-1-y)*maskStride+x/8]>>(7-x%8)&1 == 1 {
				c.A = 0
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img, nil
}

// average is the mean colour of img's pixels that show, lifted toward
// white when it would be too dark to see.
func average(img image.Image) (r, g, b uint8, ok bool) {
	var sr, sg, sb, n uint64
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A < 128 {
				continue
			}
			sr, sg, sb, n = sr+uint64(c.R), sg+uint64(c.G), sb+uint64(c.B), n+1
		}
	}
	if n == 0 {
		return 0, 0, 0, false
	}
	fr, fg, fb := float64(sr)/float64(n), float64(sg)/float64(n), float64(sb)/float64(n)
	if max(fr, fg, fb) < 100 {
		// A black icon is a black mark on a dark panel: half way to white.
		// A deep red or blue is a colour, and stays one.
		fr, fg, fb = (fr+255)/2, (fg+255)/2, (fb+255)/2
	}
	return uint8(fr), uint8(fg), uint8(fb), true
}
