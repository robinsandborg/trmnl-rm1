package display

import (
	"bytes"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// Status creates a network-independent image in the same input orientation as
// server images. Rendering still goes through the configured preparation path.
func Status(message string, cached []byte) []byte {
	canvas := image.NewGray(image.Rect(0, 0, 1872, 1404))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if img, err := prepareLandscapeImage(Options{Width: 1872, Height: 1404}, cached); err == nil {
		b := img.Bounds()
		for y := 0; y < 1404; y++ {
			for x := 0; x < 1872; x++ {
				canvas.Set(x, y, img.At(b.Min.X+x*b.Dx()/1872, b.Min.Y+y*b.Dy()/1404))
			}
		}
	}
	small := image.NewGray(image.Rect(0, 0, 312, 36))
	draw.Draw(small, small.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	d := font.Drawer{Dst: small, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13, Dot: fixed.P(8, 23)}
	d.DrawString(message)
	for y := 0; y < 216; y++ {
		for x := 0; x < 1872; x++ {
			canvas.SetGray(x, y, small.GrayAt(x/6, y/6))
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, canvas)
	return buf.Bytes()
}
