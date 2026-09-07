package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// forcedTheme wraps Fyne's default theme and pins the light/dark variant,
// ignoring the OS setting. Fyne asks the theme for every colour with the
// variant it *thinks* is active; we simply answer with the one the user
// chose. This is the pattern Fyne recommends now that theme.DarkTheme() is
// deprecated.
//
// Go interfaces are satisfied implicitly: because forcedTheme has the four
// methods fyne.Theme declares (Color, Font, Icon, Size) it *is* a fyne.Theme,
// with no "implements" keyword anywhere.
type forcedTheme struct {
	base    fyne.Theme
	variant fyne.ThemeVariant
}

func newTheme(dark bool) fyne.Theme {
	v := theme.VariantLight
	if dark {
		v = theme.VariantDark
	}
	return &forcedTheme{base: theme.DefaultTheme(), variant: v}
}

// accent is the one colour we override: the gold from the owner's banner,
// matching the web version (see design/brand.json).
var accent = color.NRGBA{R: 0xd7, G: 0xc0, B: 0x93, A: 0xff}

func (t *forcedTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		return accent
	case theme.ColorNameFocus:
		return color.NRGBA{R: 0xd7, G: 0xc0, B: 0x93, A: 0x66}
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0x1a, G: 0x14, B: 0x10, A: 0xff} // dark text on gold buttons
	}
	return t.base.Color(name, t.variant)
}

func (t *forcedTheme) Font(s fyne.TextStyle) fyne.Resource     { return t.base.Font(s) }
func (t *forcedTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return t.base.Icon(n) }
func (t *forcedTheme) Size(n fyne.ThemeSizeName) float32       { return t.base.Size(n) }

// isDark reports which variant a forcedTheme is pinned to; used by the
// toggle button to flip it.
func isDark(th fyne.Theme) bool {
	if ft, ok := th.(*forcedTheme); ok {
		return ft.variant == theme.VariantDark
	}
	return true
}
