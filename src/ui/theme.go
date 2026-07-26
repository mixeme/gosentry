package ui

import (
	"image/color"

	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// The GoSentry theme derives its palette from the logo and app icon, which use
// exactly two brand colors on white: deep teal (the wordmark and icon tile) and
// amber (the "G" gauge and the terminal prompt). Everything below extends those
// two into a working UI palette:
//   - teal is the primary color (buttons, selection, the active tab indicator),
//   - amber is the focus accent,
//   - success/warning/error carry the job run states a scheduler needs.
//
// The dark variant leans into the app icon: deep teal surfaces so the window
// reads as the icon "come to life", with a lifted teal primary and a brightened
// error red so both stay legible against the dark teal.
var (
	brandTeal      = color.NRGBA{R: 0x0A, G: 0x4A, B: 0x58, A: 0xFF} // wordmark + icon tile
	brandTealMid   = color.NRGBA{R: 0x0F, G: 0x6E, B: 0x82, A: 0xFF} // link on light
	brandTealLight = color.NRGBA{R: 0x3D, G: 0x97, B: 0xA9, A: 0xFF} // primary on dark
	brandAmber     = color.NRGBA{R: 0xF7, G: 0xA8, B: 0x0C, A: 0xFF} // the "G" + prompt
)

// gosentryLight and gosentryDark hold only the colors the brand theme overrides;
// every other ColorName falls through to the base theme, which keeps neutral
// surfaces and text contrast correct in both variants.
//
// The light variant is intentionally more than an accent swap: the window canvas
// is a soft teal while inputs, menus, and dialogs stay white, so cards and fields
// lift off a branded background instead of sitting on plain gray. Text stays dark
// (delegated to the base foreground), which keeps high contrast on both the teal
// canvas and the white surfaces.
var gosentryLight = map[fyne.ThemeColorName]color.Color{
	theme.ColorNamePrimary:           brandTeal,
	theme.ColorNameFocus:             brandAmber,
	theme.ColorNameHyperlink:         brandTealMid,
	theme.ColorNameSuccess:           color.NRGBA{R: 0x2E, G: 0x9E, B: 0x5B, A: 0xFF},
	theme.ColorNameWarning:           brandAmber,
	theme.ColorNameError:             color.NRGBA{R: 0xD6, G: 0x45, B: 0x45, A: 0xFF},
	theme.ColorNameBackground:        color.NRGBA{R: 0xDC, G: 0xEA, B: 0xED, A: 0xFF}, // teal canvas
	theme.ColorNameButton:            color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	theme.ColorNameInputBackground:   color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	theme.ColorNameMenuBackground:    color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	theme.ColorNameOverlayBackground: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
	theme.ColorNameHeaderBackground:  color.NRGBA{R: 0xC7, G: 0xDE, B: 0xE2, A: 0xFF}, // deeper teal for table headers
	theme.ColorNameSeparator:         color.NRGBA{R: 0xB4, G: 0xD0, B: 0xD6, A: 0xFF},
	theme.ColorNameInputBorder:       color.NRGBA{R: 0xB4, G: 0xD0, B: 0xD6, A: 0xFF},
	theme.ColorNameSelection:         color.NRGBA{R: 0x0A, G: 0x4A, B: 0x58, A: 0x33},
	theme.ColorNameHover:             color.NRGBA{R: 0x0A, G: 0x4A, B: 0x58, A: 0x14},
}

var gosentryDark = map[fyne.ThemeColorName]color.Color{
	theme.ColorNamePrimary:           brandTealLight,
	theme.ColorNameFocus:             brandAmber,
	theme.ColorNameHyperlink:         color.NRGBA{R: 0x6B, G: 0xB8, B: 0xCA, A: 0xFF},
	theme.ColorNameSuccess:           color.NRGBA{R: 0x46, G: 0xB8, B: 0x7A, A: 0xFF},
	theme.ColorNameWarning:           brandAmber,
	theme.ColorNameError:             color.NRGBA{R: 0xF2, G: 0x6D, B: 0x6D, A: 0xFF},
	theme.ColorNameForeground:        color.NRGBA{R: 0xEA, G: 0xF2, B: 0xF4, A: 0xFF},
	theme.ColorNamePlaceHolder:       color.NRGBA{R: 0x9B, G: 0xB4, B: 0xBC, A: 0xFF},
	theme.ColorNameBackground:        color.NRGBA{R: 0x0B, G: 0x20, B: 0x27, A: 0xFF},
	theme.ColorNameButton:            color.NRGBA{R: 0x14, G: 0x3A, B: 0x45, A: 0xFF},
	theme.ColorNameInputBackground:   color.NRGBA{R: 0x0F, G: 0x2E, B: 0x37, A: 0xFF},
	theme.ColorNameMenuBackground:    color.NRGBA{R: 0x0F, G: 0x2E, B: 0x37, A: 0xFF},
	theme.ColorNameOverlayBackground: color.NRGBA{R: 0x0F, G: 0x2E, B: 0x37, A: 0xFF},
	theme.ColorNameHeaderBackground:  color.NRGBA{R: 0x0B, G: 0x20, B: 0x27, A: 0xFF},
	theme.ColorNameSeparator:         color.NRGBA{R: 0x20, G: 0x50, B: 0x5C, A: 0xFF},
	theme.ColorNameInputBorder:       color.NRGBA{R: 0x20, G: 0x50, B: 0x5C, A: 0xFF},
	theme.ColorNameSelection:         color.NRGBA{R: 0x3D, G: 0x97, B: 0xA9, A: 0x55},
	theme.ColorNameHover:             color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x14},
}

// gosentryTheme wraps the default theme, overriding only brand colors and
// delegating fonts, icons, sizes, and unbranded colors to it. Embedding the base
// keeps the theme robust against Fyne adding new ColorNames — anything not in the
// override maps still resolves to a sensible default.
type gosentryTheme struct {
	base fyne.Theme
}

func newGoSentryTheme() fyne.Theme {
	return gosentryTheme{base: theme.DefaultTheme()}
}

func (t gosentryTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	overrides := gosentryLight
	if variant == theme.VariantDark {
		overrides = gosentryDark
	}
	if c, ok := overrides[name]; ok {
		return c
	}
	return t.base.Color(name, variant)
}

func (t gosentryTheme) Font(style fyne.TextStyle) fyne.Resource { return t.base.Font(style) }
func (t gosentryTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base.Icon(name) }
func (t gosentryTheme) Size(name fyne.ThemeSizeName) float32 { return t.base.Size(name) }

// themeFor maps a stored Theme choice to a concrete fyne.Theme. Anything other
// than the explicit GoSentry choice (including the empty/legacy value) keeps
// Fyne's built-in theme.
func themeFor(choice domain.Theme) fyne.Theme {
	if choice == domain.ThemeGoSentry {
		return newGoSentryTheme()
	}
	return theme.DefaultTheme()
}

// applyTheme installs the theme for the given choice on the running app. Fyne
// refreshes every canvas when the theme changes, so this works both at startup
// and when the user switches themes from Settings.
func applyTheme(a fyne.App, choice domain.Theme) {
	a.Settings().SetTheme(themeFor(choice))
}
