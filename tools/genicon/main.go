//go:build ignore

// genicon draws the app icon: a waveform over a play triangle on a dark
// rounded square. Pure Go, no dependencies, so the icon is reproducible.
//
//	go run tools/genicon/main.go assets/icon.png
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

const size = 512

var (
	bg     = color.NRGBA{0x0b, 0x0f, 0x14, 0xff}
	amber  = color.NRGBA{0xf5, 0x9e, 0x0b, 0xff}
	amber2 = color.NRGBA{0xfb, 0xbf, 0x24, 0xff}
	white  = color.NRGBA{0xf8, 0xfa, 0xfc, 0xff}
)

func main() {
	out := "assets/icon.png"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	// Rounded square background.
	const r = 96.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if inRoundedRect(float64(x)+0.5, float64(y)+0.5, 0, 0, size, size, r) {
				img.SetNRGBA(x, y, bg)
			}
		}
	}

	// Play triangle, slightly right of centre so it looks optically centred.
	tri := [3][2]float64{{150, 116}, {150, 396}, {400, 256}}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			if inTriangle(px, py, tri) {
				// Subtle vertical gradient.
				t := (py - 116) / 280
				img.SetNRGBA(x, y, lerp(amber2, amber, t))
			}
		}
	}

	// Waveform: vertical bars across the middle, heights follow a bell-ish
	// envelope with a little wobble so it reads as audio, not a bar chart.
	heights := []float64{0.18, 0.32, 0.55, 0.42, 0.78, 0.95, 0.62, 0.88, 0.48, 0.70, 0.36, 0.22}
	barW, gap := 16.0, 14.0
	total := float64(len(heights))*barW + float64(len(heights)-1)*gap
	x0 := (size - total) / 2
	for i, h := range heights {
		bh := h * 220
		bx := x0 + float64(i)*(barW+gap)
		by := (size - bh) / 2
		fillRoundedRect(img, bx, by, barW, bh, barW/2, white)
	}

	f, err := os.Create(out)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func inRoundedRect(px, py, x, y, w, h, r float64) bool {
	if px < x || py < y || px > x+w || py > y+h {
		return false
	}
	// Inside the straight parts?
	if (px >= x+r && px <= x+w-r) || (py >= y+r && py <= y+h-r) {
		return true
	}
	// Corner circles.
	cx := x + r
	if px > x+w-r {
		cx = x + w - r
	}
	cy := y + r
	if py > y+h-r {
		cy = y + h - r
	}
	return math.Hypot(px-cx, py-cy) <= r
}

func fillRoundedRect(img *image.NRGBA, x, y, w, h, r float64, c color.NRGBA) {
	for py := int(y); py <= int(y+h)+1; py++ {
		for px := int(x); px <= int(x+w)+1; px++ {
			if inRoundedRect(float64(px)+0.5, float64(py)+0.5, x, y, w, h, r) {
				img.SetNRGBA(px, py, c)
			}
		}
	}
}

func inTriangle(px, py float64, t [3][2]float64) bool {
	sign := func(a, b, c [2]float64) float64 {
		return (a[0]-c[0])*(b[1]-c[1]) - (b[0]-c[0])*(a[1]-c[1])
	}
	p := [2]float64{px, py}
	d1 := sign(p, t[0], t[1])
	d2 := sign(p, t[1], t[2])
	d3 := sign(p, t[2], t[0])
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.NRGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}
