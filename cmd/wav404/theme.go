package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Colours of the standard widgets (top strip, dialogs). The panel itself
// uses the hardware palette in hwdraw.go; the accent is the same amber.
var (
	colBackground = color.NRGBA{0x12, 0x14, 0x17, 0xff}
	colText       = color.NRGBA{0xe6, 0xe9, 0xee, 0xff}
)

type wavTheme struct{}

func (wavTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return hwAmber
	case theme.ColorNameForeground:
		return colText
	}
	return theme.DefaultTheme().Color(n, theme.VariantDark)
}

func (wavTheme) Font(s fyne.TextStyle) fyne.Resource     { return theme.DefaultTheme().Font(s) }
func (wavTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (wavTheme) Size(n fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(n) }
