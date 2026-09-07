package pipeline

// Dimensions is a target output size for the scale filter.
type Dimensions struct {
	Width  int
	Height int
	Scaled bool // false means "input already fits, omit the scale filter"
}

// FitDimensions computes the output size for a video of inW x inH (as stored
// in the file) with the given rotation metadata, so that it fits inside a
// preset's bounding box without ever upscaling.
//
// Rules, in order:
//  1. Apply rotation: 90/270 swap width and height (phone portrait videos).
//  2. Orient the box to match the input: a landscape box becomes portrait
//     for a portrait input and vice versa, so "1280x720" really means
//     "720p in whichever orientation you shot".
//  3. If the input already fits, return it unchanged with Scaled=false.
//  4. Otherwise scale uniformly so the larger constraint binds.
//  5. Round both sides down to even numbers (libx264 with yuv420p needs
//     even dimensions), minimum 2.
func FitDimensions(inW, inH, rotation, maxW, maxH int) Dimensions {
	if rotation == 90 || rotation == 270 {
		inW, inH = inH, inW
	}
	if inW <= 0 || inH <= 0 || maxW <= 0 || maxH <= 0 {
		return Dimensions{Width: inW, Height: inH, Scaled: false}
	}

	// Orient the bounding box to the input.
	inputPortrait := inH > inW
	boxPortrait := maxH > maxW
	if inputPortrait != boxPortrait {
		maxW, maxH = maxH, maxW
	}

	if inW <= maxW && inH <= maxH {
		// Still enforce even dimensions on an unscaled input; odd sizes
		// exist in the wild (e.g. 1080x1081 screen recordings).
		w, h := even(inW), even(inH)
		return Dimensions{Width: w, Height: h, Scaled: w != inW || h != inH}
	}

	// Uniform scale factor: whichever side overflows more decides.
	rw := float64(maxW) / float64(inW)
	rh := float64(maxH) / float64(inH)
	r := rw
	if rh < rw {
		r = rh
	}
	w := even(int(float64(inW)*r + 0.5))
	h := even(int(float64(inH)*r + 0.5))
	// Rounding can push one side 2px over the box; clamp.
	if w > maxW {
		w = even(maxW)
	}
	if h > maxH {
		h = even(maxH)
	}
	return Dimensions{Width: w, Height: h, Scaled: true}
}

// even rounds n down to the nearest even number, never below 2.
func even(n int) int {
	n -= n % 2
	if n < 2 {
		return 2
	}
	return n
}
