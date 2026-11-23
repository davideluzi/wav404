package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Palette: flat graphite with a teal accent. Deliberately unlike Roland's
// red, glowing, numbered pads.
var (
	colBackground = color.NRGBA{0x12, 0x14, 0x17, 0xff}
	colPadEmpty   = color.NRGBA{0x1b, 0x1f, 0x24, 0xff}
	colPadFull    = color.NRGBA{0x26, 0x2d, 0x36, 0xff}
	colPadPlaying = color.NRGBA{0x1d, 0x4a, 0x46, 0xff} // dark teal
	colAccent     = color.NRGBA{0x35, 0xc2, 0xb0, 0xff}
	colText       = color.NRGBA{0xe6, 0xe9, 0xee, 0xff}
	colMuted      = color.NRGBA{0x7d, 0x85, 0x90, 0xff}
)

type wavTheme struct{}

func (wavTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return colAccent
	case theme.ColorNameForeground:
		return colText
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (wavTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (wavTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (wavTheme) Size(n fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(n) }
