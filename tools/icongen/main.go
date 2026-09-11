// Command icongen draws the GrabOne application icon and writes the assets the
// Windows build needs: build/appicon.png and build/windows/icon.ico.
//
// The icon is generated rather than committed as an opaque binary so the mark
// can be adjusted by editing geometry here and re-running:
//
//	go run ./tools/icongen
//
// The mark is a download arrow over a tray, on a rounded square with the same
// gradient the interface uses for its brand mark. It is drawn at four times the
// target size and averaged down, which is what gives the curves their smooth
// edges at small sizes.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// supersample is the factor the icon is drawn at before being averaged down.
const supersample = 4

// icoSizes are the sizes packed into the .ico file. Windows picks the closest
// one for each context, from the 16 pixel title bar to the 256 pixel preview.
var icoSizes = []int{16, 24, 32, 48, 64, 128, 256}

// Brand colours, matching the gradient of the in-app brand mark.
var (
	gradientTop    = color.RGBA{R: 0x5b, G: 0x8c, B: 0xff, A: 0xff}
	gradientBottom = color.RGBA{R: 0x8a, G: 0x6b, B: 0xff, A: 0xff}
	glyphColor     = color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "icongen:", err)
		os.Exit(1)
	}
}

func run() error {
	appIcon := render(1024)
	if err := writePNG(filepath.Join("build", "appicon.png"), appIcon); err != nil {
		return err
	}

	images := make([]*image.RGBA, 0, len(icoSizes))
	for _, size := range icoSizes {
		images = append(images, render(size))
	}
	if err := writeICO(filepath.Join("build", "windows", "icon.ico"), images); err != nil {
		return err
	}

	fmt.Println("wrote build/appicon.png and build/windows/icon.ico")
	return nil
}

// render draws the icon at the given size.
func render(size int) *image.RGBA {
	large := size * supersample
	canvas := image.NewRGBA(image.Rect(0, 0, large, large))

	edge := float64(large)
	radius := edge * 0.22

	for y := 0; y < large; y++ {
		for x := 0; x < large; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5

			if !insideRoundedSquare(px, py, edge, radius) {
				continue
			}

			pixel := gradientAt(py / edge)
			if insideGlyph(px/edge, py/edge) {
				pixel = glyphColor
			}
			canvas.SetRGBA(x, y, pixel)
		}
	}

	return downscale(canvas, size)
}

// insideRoundedSquare reports whether a point falls inside a rounded square
// filling the canvas.
func insideRoundedSquare(x, y, edge, radius float64) bool {
	// Distance from the square's inner rectangle, which is the standard rounded
	// rectangle test: only the corners need the circular check.
	dx := math.Max(math.Max(radius-x, x-(edge-radius)), 0)
	dy := math.Max(math.Max(radius-y, y-(edge-radius)), 0)
	return dx*dx+dy*dy <= radius*radius
}

// gradientAt returns the background colour at a vertical position in 0..1.
func gradientAt(position float64) color.RGBA {
	blend := func(from, to uint8) uint8 {
		return uint8(float64(from) + (float64(to)-float64(from))*position)
	}
	return color.RGBA{
		R: blend(gradientTop.R, gradientBottom.R),
		G: blend(gradientTop.G, gradientBottom.G),
		B: blend(gradientTop.B, gradientBottom.B),
		A: 0xff,
	}
}

// insideGlyph reports whether a normalised point falls inside the download mark:
// a stem, an arrow head and a tray beneath them.
func insideGlyph(x, y float64) bool {
	// Stem of the arrow.
	if x >= 0.435 && x <= 0.565 && y >= 0.200 && y <= 0.545 {
		return true
	}

	// Arrow head: a triangle narrowing to a point at the bottom.
	if y >= 0.500 && y <= 0.735 {
		progress := (y - 0.500) / (0.735 - 0.500)
		halfWidth := 0.175 * (1 - progress)
		if math.Abs(x-0.5) <= halfWidth {
			return true
		}
	}

	// Tray, with rounded ends so it stays legible at 16 pixels.
	const (
		trayTop    = 0.775
		trayBottom = 0.850
		trayLeft   = 0.285
		trayRight  = 0.715
	)
	trayRadius := (trayBottom - trayTop) / 2
	if y >= trayTop && y <= trayBottom {
		if x >= trayLeft+trayRadius && x <= trayRight-trayRadius {
			return true
		}
		centerY := (trayTop + trayBottom) / 2
		for _, centerX := range []float64{trayLeft + trayRadius, trayRight - trayRadius} {
			dx, dy := x-centerX, y-centerY
			if dx*dx+dy*dy <= trayRadius*trayRadius {
				return true
			}
		}
	}
	return false
}

// downscale averages blocks of the supersampled canvas, which anti-aliases both
// the rounded corners and the glyph.
func downscale(source *image.RGBA, size int) *image.RGBA {
	result := image.NewRGBA(image.Rect(0, 0, size, size))
	samples := supersample * supersample

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var red, green, blue, alpha int

			for sy := 0; sy < supersample; sy++ {
				for sx := 0; sx < supersample; sx++ {
					pixel := source.RGBAAt(x*supersample+sx, y*supersample+sy)
					// Sum premultiplied values so transparent pixels do not
					// darken the edges.
					red += int(pixel.R)
					green += int(pixel.G)
					blue += int(pixel.B)
					alpha += int(pixel.A)
				}
			}

			result.SetRGBA(x, y, color.RGBA{
				R: uint8(red / samples),
				G: uint8(green / samples),
				B: uint8(blue / samples),
				A: uint8(alpha / samples),
			})
		}
	}
	return result
}

func writePNG(path string, source image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer file.Close()

	if err := png.Encode(file, source); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}

// writeICO packs the images into an icon file. Each entry holds a PNG, which
// Windows has supported since Vista and which keeps the file small.
func writeICO(path string, images []*image.RGBA) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	encoded := make([][]byte, 0, len(images))
	for _, current := range images {
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, current); err != nil {
			return fmt.Errorf("encode icon entry: %w", err)
		}
		encoded = append(encoded, buffer.Bytes())
	}

	var out bytes.Buffer

	// ICONDIR: reserved, type 1 (icon), image count.
	writeLE(&out, uint16(0), uint16(1), uint16(len(encoded)))

	// The image data follows the directory, so the first offset is past both.
	offset := 6 + 16*len(encoded)
	for index, data := range encoded {
		size := images[index].Bounds().Dx()

		// A 256 pixel image is recorded as 0 in the single byte dimensions.
		dimension := byte(size)
		if size >= 256 {
			dimension = 0
		}

		out.WriteByte(dimension) // width
		out.WriteByte(dimension) // height
		out.WriteByte(0)         // palette size, 0 for true colour
		out.WriteByte(0)         // reserved
		writeLE(&out, uint16(1), uint16(32))
		writeLE(&out, uint32(len(data)), uint32(offset))

		offset += len(data)
	}

	for _, data := range encoded {
		out.Write(data)
	}

	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func writeLE(buffer *bytes.Buffer, values ...any) {
	for _, value := range values {
		_ = binary.Write(buffer, binary.LittleEndian, value)
	}
}
