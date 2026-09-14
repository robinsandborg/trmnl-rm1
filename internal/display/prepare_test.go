package display_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/robinsandborg/rm1-trmnl/internal/display"
)

type fixture struct {
	Name, Input, Golden     string
	Width, Height, Rotation int
	SkipRotation            bool
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixtures(t *testing.T) []fixture {
	t.Helper()
	var cases []fixture
	if err := json.Unmarshal(readFixture(t, "cases.json"), &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func golden(t *testing.T, name string) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(readFixture(t, name)))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func assertPixels(t *testing.T, got, want image.Image) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds=%v want %v", got.Bounds(), want.Bounds())
	}
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				t.Fatalf("pixel (%d,%d)=(%d,%d,%d,%d) want (%d,%d,%d,%d)", x, y, gr, gg, gb, ga, wr, wg, wb, wa)
			}
		}
	}
}

func TestPrepareMatchesLegacyPixels(t *testing.T) {
	t.Parallel()
	for _, tc := range fixtures(t) {
		t.Run(tc.Name, func(t *testing.T) {
			data := readFixture(t, tc.Input)
			before := bytes.Clone(data)
			got, err := display.Prepare(display.Options{Width: tc.Width, Height: tc.Height, Rotation: tc.Rotation, SkipRotation: tc.SkipRotation}, data)
			if err != nil {
				t.Fatal(err)
			}
			assertPixels(t, got, golden(t, tc.Golden))
			if !bytes.Equal(data, before) {
				t.Fatal("input bytes mutated")
			}
		})
	}
}

func TestPreparePrimaryColorLuminance(t *testing.T) {
	img, err := display.Prepare(display.Options{Width: 4, Height: 2, Rotation: 3}, readFixture(t, "landscape.png"))
	if err != nil {
		t.Fatal(err)
	}
	for x, want := range []uint32{76, 150, 29, 255} {
		r, g, b, a := img.At(x, 0).RGBA()
		if r != want*257 || g != r || b != r || a != 65535 {
			t.Fatalf("pixel %d=(%d,%d,%d,%d); want gray %d", x, r, g, b, a, want)
		}
	}
}

func TestPrepareDecodeError(t *testing.T) {
	img, err := display.Prepare(display.Options{Width: 4, Height: 2}, []byte("not an image"))
	if img != nil || !errors.Is(err, image.ErrFormat) || err.Error() != "decode image: image: unknown format" {
		t.Fatalf("image=%v error=%v", img, err)
	}
}
