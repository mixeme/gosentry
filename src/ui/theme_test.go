package ui

import (
	"image/color"
	"testing"

	"gitea.mixdep.ru/mix/gosentry/src/domain"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// The GoSentry theme must expose the brand colors on the semantically correct
// ColorNames in each variant. These are the touchpoints a user actually sees —
// the primary color drives buttons and the active tab, focus drives the accent —
// so they are worth pinning against accidental edits to the palette maps.
func TestGoSentryThemeBrandColors(t *testing.T) {
	th := newGoSentryTheme()
	cases := []struct {
		name    string
		color   fyne.ThemeColorName
		variant int
		want    color.Color
	}{
		{"light primary is teal", theme.ColorNamePrimary, 0, brandTeal},
		{"light focus is amber", theme.ColorNameFocus, 0, brandAmber},
		{"light canvas is a teal tint", theme.ColorNameBackground, 0, color.NRGBA{R: 0xDC, G: 0xEA, B: 0xED, A: 0xFF}},
		{"light inputs stay white", theme.ColorNameInputBackground, 0, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}},
		{"dark primary is lifted teal", theme.ColorNamePrimary, 1, brandTealLight},
		{"dark focus is amber", theme.ColorNameFocus, 1, brandAmber},
	}
	for _, tc := range cases {
		variant := theme.VariantLight
		if tc.variant == 1 {
			variant = theme.VariantDark
		}
		got := th.Color(tc.color, variant)
		if got != tc.want {
			t.Errorf("%s: Color(%s) = %v, want %v", tc.name, tc.color, got, tc.want)
		}
	}
}

// Unbranded color names must fall through to the base theme rather than render as
// zero-value (transparent) colors, so the theme only recolors what it intends to.
func TestGoSentryThemeDelegatesUnbrandedColors(t *testing.T) {
	th := newGoSentryTheme()
	base := theme.DefaultTheme()
	// ScrollBar is not in either override map, so it must match the base theme.
	got := th.Color(theme.ColorNameScrollBar, theme.VariantDark)
	want := base.Color(theme.ColorNameScrollBar, theme.VariantDark)
	if got != want {
		t.Errorf("unbranded ColorNameScrollBar = %v, want base %v", got, want)
	}
}

// themeFor maps the stored choice to the right theme: the GoSentry choice and the
// empty legacy value yield the branded teal primary; only the explicit default
// choice yields Fyne's built-in theme.
func TestThemeForChoice(t *testing.T) {
	for _, choice := range []domain.Theme{domain.ThemeGoSentry, ""} {
		branded := themeFor(choice)
		if got := branded.Color(theme.ColorNamePrimary, theme.VariantLight); got != brandTeal {
			t.Errorf("themeFor(%q) primary = %v, want brand teal %v", choice, got, brandTeal)
		}
	}
	def := themeFor(domain.ThemeDefault)
	if got := def.Color(theme.ColorNamePrimary, theme.VariantLight); got == brandTeal {
		t.Errorf("themeFor(default) should not use the brand teal primary")
	}
}

// The dropdown label helpers must round-trip, and the empty/legacy value must map
// to the GoSentry label so the select never shows a blank option.
func TestThemeLabelRoundTrip(t *testing.T) {
	if got := themeFromLabel(themeLabel(domain.ThemeGoSentry)); got != domain.ThemeGoSentry {
		t.Errorf("round-trip gosentry = %q", got)
	}
	if got := themeFromLabel(themeLabel(domain.ThemeDefault)); got != domain.ThemeDefault {
		t.Errorf("round-trip default = %q", got)
	}
	if got := themeLabel(""); got != themeLabelGoSentry {
		t.Errorf("empty theme label = %q, want %q", got, themeLabelGoSentry)
	}
}
