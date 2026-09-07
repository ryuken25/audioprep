package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// forcedTheme wraps Fyne's default theme, pins the light/dark variant (ignoring
// the OS setting) and answers with the palette from the design pass. The values
// are the colour sheet in the design project, which is the same palette the web
// version uses; see design/tokens.css.
//
// Go interfaces are satisfied implicitly: because forcedTheme has the four
// methods fyne.Theme declares (Color, Font, Icon, Size) it *is* a fyne.Theme,
// with no "implements" keyword anywhere.
type forcedTheme struct {
	base    fyne.Theme
	variant fyne.ThemeVariant
	// glass makes the surfaces translucent so the backdrop picture shows
	// through, which is what the VRChat skin is. Fyne cannot blur, so the
	// alpha over an already-soft picture is what reads as frosted glass.
	glass bool
}

func newTheme(dark, glass bool) fyne.Theme {
	v := theme.VariantLight
	if dark {
		v = theme.VariantDark
	}
	return &forcedTheme{base: theme.DefaultTheme(), variant: v, glass: glass}
}

// The palette. Mauve is the structure, gold is the single action colour.
var (
	// Mauve, from the avatar.
	mauve     = color.NRGBA{R: 0x8a, G: 0x61, B: 0x68, A: 0xff}
	mauveDeep = color.NRGBA{R: 0x5b, G: 0x3c, B: 0x3e, A: 0xff}
	mauveText = color.NRGBA{R: 0xd1, G: 0xad, B: 0xb3, A: 0xff}
	// Gold, from the Bali sunset banner. Process, Process another, progress fill.
	gold   = color.NRGBA{R: 0xd7, G: 0xc0, B: 0x93, A: 0xff}
	onGold = color.NRGBA{R: 0x1a, G: 0x14, B: 0x10, A: 0xff}
)

func (t *forcedTheme) dark() bool { return t.variant == theme.VariantDark }

// pick returns d in dark mode and l in light mode; it keeps Color() readable.
func (t *forcedTheme) pick(d, l color.Color) color.Color {
	if t.dark() {
		return d
	}
	return l
}

func rgba(r, g, b, a uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: a} }

func (t *forcedTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return t.pick(rgba(0x0b, 0x0b, 0x0f, 0xff), rgba(0xe6, 0xe6, 0xf0, 0xff))

	case theme.ColorNameOverlayBackground:
		if t.glass {
			return t.pick(rgba(0x14, 0x11, 0x16, 0xbd), rgba(0xf8, 0xf7, 0xfb, 0xd1))
		}
		return t.pick(rgba(0x14, 0x11, 0x16, 0xff), rgba(0xf8, 0xf7, 0xfb, 0xff))

	case theme.ColorNameMenuBackground:
		// Popups must stay opaque or the list behind them shows through.
		return t.pick(rgba(0x14, 0x11, 0x16, 0xff), rgba(0xf8, 0xf7, 0xfb, 0xff))

	case theme.ColorNameHeaderBackground:
		if t.glass {
			return t.pick(rgba(0x14, 0x11, 0x16, 0xbd), rgba(0xf3, 0xf2, 0xf8, 0xd1))
		}
		return t.pick(rgba(0x14, 0x11, 0x16, 0xff), rgba(0xf3, 0xf2, 0xf8, 0xff))

	case theme.ColorNameInputBackground:
		if t.glass {
			return t.pick(rgba(0x11, 0x10, 0x15, 0xb8), rgba(0xff, 0xff, 0xff, 0xc7))
		}
		return t.pick(rgba(0x11, 0x10, 0x15, 0xff), rgba(0xff, 0xff, 0xff, 0xff))

	case theme.ColorNameInputBorder:
		return t.pick(rgba(0xff, 0xff, 0xff, 0x33), rgba(0x1c, 0x17, 0x1d, 0x38))

	case theme.ColorNameButton:
		if t.glass {
			return t.pick(rgba(0xff, 0xff, 0xff, 0x12), rgba(0xff, 0xff, 0xff, 0xb3))
		}
		return t.pick(rgba(0x1b, 0x17, 0x1d, 0xff), rgba(0xff, 0xff, 0xff, 0xff))

	case theme.ColorNameDisabledButton:
		if t.glass {
			return t.pick(rgba(0xff, 0xff, 0xff, 0x0a), rgba(0xff, 0xff, 0xff, 0x73))
		}
		return t.pick(rgba(0x1b, 0x17, 0x1d, 0xff), rgba(0xf3, 0xf2, 0xf8, 0xff))

	case theme.ColorNameDisabled:
		return t.pick(rgba(0x7a, 0x70, 0x80, 0xff), rgba(0x8b, 0x84, 0x96, 0xff))

	case theme.ColorNamePlaceHolder:
		return t.pick(rgba(0xa3, 0x9a, 0xa6, 0xff), rgba(0x5b, 0x54, 0x62, 0xff))

	case theme.ColorNameForeground:
		return t.pick(rgba(0xec, 0xe8, 0xee, 0xff), rgba(0x1c, 0x17, 0x1d, 0xff))

	// Gold: Process, Process another, the progress fill. Nothing else.
	case theme.ColorNamePrimary:
		return gold
	case theme.ColorNameForegroundOnPrimary:
		return onGold

	case theme.ColorNameFocus:
		return t.pick(rgba(0x9a, 0x97, 0xa2, 0x80), rgba(0x68, 0x48, 0x4c, 0x73))
	case theme.ColorNameHover:
		return t.pick(rgba(0x8a, 0x61, 0x68, 0x2e), rgba(0x5b, 0x3c, 0x3e, 0x14))
	case theme.ColorNamePressed:
		return t.pick(rgba(0x8a, 0x61, 0x68, 0x4d), rgba(0x5b, 0x3c, 0x3e, 0x29))
	case theme.ColorNameSelection:
		return t.pick(rgba(0x8a, 0x61, 0x68, 0x59), rgba(0x5b, 0x3c, 0x3e, 0x33))

	case theme.ColorNameSeparator:
		return t.pick(rgba(0xff, 0xff, 0xff, 0x17), rgba(0x1c, 0x17, 0x1d, 0x17))
	case theme.ColorNameScrollBar:
		return t.pick(rgba(0x3d, 0x31, 0x39, 0xff), rgba(0xb6, 0xaf, 0xc2, 0xff))
	case theme.ColorNameShadow:
		return t.pick(rgba(0x00, 0x00, 0x00, 0x80), rgba(0x14, 0x0a, 0x1e, 0x1f))

	case theme.ColorNameHyperlink:
		return t.pick(mauveText, mauveDeep)
	case theme.ColorNameSuccess:
		return t.pick(rgba(0x8f, 0xbf, 0x9f, 0xff), rgba(0x2f, 0x7d, 0x52, 0xff))
	case theme.ColorNameWarning:
		return t.pick(gold, rgba(0x7a, 0x5f, 0x24, 0xff))
	case theme.ColorNameError:
		return t.pick(rgba(0xd1, 0x77, 0x77, 0xff), rgba(0xb2, 0x3b, 0x3b, 0xff))
	}
	return t.base.Color(name, t.variant)
}

func (t *forcedTheme) Font(s fyne.TextStyle) fyne.Resource     { return t.base.Font(s) }
func (t *forcedTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.base.Icon(n) }

func (t *forcedTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 6 // default 4: a little more air between rows
	case theme.SizeNameInnerPadding:
		return 10
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameCardRadius:
		return 10
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 12
	}
	return t.base.Size(n)
}

// isDark and isGlass report what a forcedTheme is pinned to, so a toggle can
// flip one without losing the other.
func isDark(th fyne.Theme) bool {
	if ft, ok := th.(*forcedTheme); ok {
		return ft.variant == theme.VariantDark
	}
	return true
}

func isGlass(th fyne.Theme) bool {
	if ft, ok := th.(*forcedTheme); ok {
		return ft.glass
	}
	return true
}
