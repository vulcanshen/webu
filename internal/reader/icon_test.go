package reader

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

func pngOf(c color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

// ico4Of is an ICO of a 2×2 bitmap of 4 bits a pixel: three pixels of c,
// and a white one the mask hides.
func ico4Of(c color.NRGBA) []byte {
	head := make([]byte, 40)
	binary.LittleEndian.PutUint32(head[0:], 40)
	binary.LittleEndian.PutUint32(head[4:], 2)
	binary.LittleEndian.PutUint32(head[8:], 4)
	binary.LittleEndian.PutUint16(head[12:], 1)
	binary.LittleEndian.PutUint16(head[14:], 4)
	binary.LittleEndian.PutUint32(head[32:], 2) // two colours
	data := append(head, c.B, c.G, c.R, 0, 255, 255, 255, 0)
	// Rows bottom first, each padded to 4 bytes: c and white, then c and c.
	data = append(data, 0x01, 0, 0, 0, 0x00, 0, 0, 0)
	// The mask hides the bottom row's white.
	data = append(data, 0x40, 0, 0, 0, 0x00, 0, 0, 0)
	return icoOf(data, c)
}

// icoOf is an ICO of one 2×2 image: data as it is (a PNG, a bitmap), or a
// 32-bit bitmap of c.
func icoOf(data []byte, c color.NRGBA) []byte {
	if data == nil {
		head := make([]byte, 40)
		binary.LittleEndian.PutUint32(head[0:], 40)
		binary.LittleEndian.PutUint32(head[4:], 2)
		binary.LittleEndian.PutUint32(head[8:], 4) // twice the height: the mask
		binary.LittleEndian.PutUint16(head[12:], 1)
		binary.LittleEndian.PutUint16(head[14:], 32)
		data = head
		for i := 0; i < 4; i++ {
			data = append(data, c.B, c.G, c.R, c.A)
		}
		data = append(data, make([]byte, 8)...) // the mask
	}
	out := make([]byte, 22)
	binary.LittleEndian.PutUint16(out[2:], 1)
	binary.LittleEndian.PutUint16(out[4:], 1)
	out[6], out[7] = 2, 2
	binary.LittleEndian.PutUint32(out[6+8:], uint32(len(data)))
	binary.LittleEndian.PutUint32(out[6+12:], 22)
	return append(out, data...)
}

// A site's icon colour, from a PNG, an ICO of a bitmap (32-bit, or 4-bit
// with its mask) or of a PNG; a black one lifted so it shows; none from a
// page that is not an icon.
func TestIconColor(t *testing.T) {
	red := color.NRGBA{R: 220, G: 20, B: 20, A: 255}
	blue := color.NRGBA{R: 20, G: 40, B: 230, A: 255}
	green := color.NRGBA{R: 30, G: 200, B: 60, A: 255}
	icons := map[string][]byte{
		"/red.png":   pngOf(red),
		"/bmp.ico":   icoOf(nil, blue),
		"/4bit.ico":  ico4Of(green),
		"/png.ico":   icoOf(pngOf(red), red),
		"/black.png": pngOf(color.NRGBA{A: 255}),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b, ok := icons[r.URL.Path]; ok {
			w.Header().Set("Content-Type", "image/x-icon")
			w.Write(b)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	jar, _ := OpenJar("")
	c := NewClient(jar, "test")
	for path, want := range map[string]color.NRGBA{"/red.png": red, "/bmp.ico": blue, "/4bit.ico": green, "/png.ico": red, "/black.png": {R: 127, G: 127, B: 127}} {
		r, g, b, ok := c.IconColor(context.Background(), srv.URL+path)
		if !ok || r != want.R || g != want.G || b != want.B {
			t.Errorf("%s: %d,%d,%d %v, want %d,%d,%d", path, r, g, b, ok, want.R, want.G, want.B)
		}
	}
	if _, _, _, ok := c.IconColor(context.Background(), srv.URL+"/missing.ico"); ok {
		t.Error("a page not found is no icon")
	}
}
