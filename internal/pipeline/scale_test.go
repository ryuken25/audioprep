package pipeline

import "testing"

func TestFitDimensions(t *testing.T) {
	// Table-driven tests are the idiomatic Go way to cover many cases of a
	// pure function: one slice of inputs and expectations, one loop.
	cases := []struct {
		name          string
		inW, inH, rot int
		maxW, maxH    int
		wantW, wantH  int
		wantScaled    bool
	}{
		{"1080p landscape into 720p box", 1920, 1080, 0, 1280, 720, 1280, 720, true},
		{"720p already fits", 1280, 720, 0, 1280, 720, 1280, 720, false},
		{"small input never upscaled", 640, 360, 0, 1280, 720, 640, 360, false},
		{"4k into 720p", 3840, 2160, 0, 1280, 720, 1280, 720, true},

		// Portrait input: the box flips to 720x1280.
		{"portrait 1080x1920 into 720p box", 1080, 1920, 0, 1280, 720, 720, 1280, true},
		{"portrait already fits", 720, 1280, 0, 1280, 720, 720, 1280, false},

		// Rotation metadata: stored landscape, displayed portrait.
		{"phone clip stored 1920x1080 rot 90", 1920, 1080, 90, 1280, 720, 720, 1280, true},
		{"phone clip rot 270", 1920, 1080, 270, 1280, 720, 720, 1280, true},
		{"rot 180 does not swap", 1920, 1080, 180, 1280, 720, 1280, 720, true},

		// TikTok box is portrait 1080x1920 as stored in the preset table
		// (MaxWidth 1920, MaxHeight 1080); landscape input flips it.
		{"landscape into tiktok box", 3840, 2160, 0, 1920, 1080, 1920, 1080, true},
		{"portrait 4k into tiktok box", 2160, 3840, 0, 1920, 1080, 1080, 1920, true},

		// Odd aspect ratios: the binding side decides, the other shrinks.
		{"ultrawide", 3440, 1440, 0, 1280, 720, 1280, 536, true},
		{"square", 1000, 1000, 0, 1280, 720, 720, 720, true},
		{"tall 9:19.5 phone", 1080, 2340, 0, 1280, 720, 590, 1280, true},

		// Even-dimension enforcement.
		{"odd input that fits gets evened", 1279, 719, 0, 1280, 720, 1278, 718, true},
		{"odd result rounded down", 1281, 721, 0, 1280, 720, 1278, 720, true},

		// Degenerate.
		{"zero box means no scaling", 1920, 1080, 0, 0, 0, 1920, 1080, false},
		{"tiny input", 3, 3, 0, 1280, 720, 2, 2, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FitDimensions(tc.inW, tc.inH, tc.rot, tc.maxW, tc.maxH)
			if got.Width != tc.wantW || got.Height != tc.wantH || got.Scaled != tc.wantScaled {
				t.Errorf("FitDimensions(%d,%d,rot%d,box %dx%d) = %dx%d scaled=%v; want %dx%d scaled=%v",
					tc.inW, tc.inH, tc.rot, tc.maxW, tc.maxH,
					got.Width, got.Height, got.Scaled, tc.wantW, tc.wantH, tc.wantScaled)
			}
			if got.Width%2 != 0 || got.Height%2 != 0 {
				t.Errorf("odd dimension in result: %dx%d", got.Width, got.Height)
			}
		})
	}
}

func TestFitDimensions_NeverExceedsBox(t *testing.T) {
	// Fuzz-ish sweep: whatever the input, the output must fit the oriented box.
	for w := 100; w <= 4000; w += 137 {
		for h := 100; h <= 4000; h += 149 {
			d := FitDimensions(w, h, 0, 1280, 720)
			maxW, maxH := 1280, 720
			if h > w {
				maxW, maxH = 720, 1280
			}
			if d.Width > maxW || d.Height > maxH {
				t.Fatalf("%dx%d -> %dx%d exceeds %dx%d", w, h, d.Width, d.Height, maxW, maxH)
			}
			if d.Width > w || d.Height > h {
				t.Fatalf("%dx%d -> %dx%d upscaled", w, h, d.Width, d.Height)
			}
		}
	}
}

func TestEven(t *testing.T) {
	for in, want := range map[int]int{0: 2, 1: 2, 2: 2, 3: 2, 719: 718, 720: 720, 1281: 1280} {
		if got := even(in); got != want {
			t.Errorf("even(%d) = %d, want %d", in, got, want)
		}
	}
}
