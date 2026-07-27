package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// TestRowOverlapMatchesInnerPadding pins rowOverlap to theme.InnerPadding, the
// property that lets it follow a theme with a different SizeNameInnerPadding
// instead of drifting from a hand-tuned literal.
func TestRowOverlapMatchesInnerPadding(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	if got, want := rowOverlap(), -theme.InnerPadding(); got != want {
		t.Errorf("rowOverlap() = %v, want %v", got, want)
	}
	if rowOverlap() >= 0 {
		t.Errorf("rowOverlap() = %v, want a negative value", rowOverlap())
	}

	testApp.Settings().SetTheme(test.NewTheme())
	if got, want := rowOverlap(), -theme.InnerPadding(); got != want {
		t.Errorf("under a different theme, rowOverlap() = %v, want %v", got, want)
	}
}

// TestCaptionColumnWidth covers the shapes F10's shared helper has to handle:
// no captions, one, and several of varying length at two text sizes.
func TestCaptionColumnWidth(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	if got := captionColumnWidth(); got != 0 {
		t.Errorf("no captions: got %v, want 0", got)
	}
	solo := captionColumnWidth("Solo")
	if solo <= 0 {
		t.Errorf("one caption: got %v, want > 0", solo)
	}
	widest := captionColumnWidth("Short", "A Much Longer Caption")
	if widest <= solo {
		t.Errorf("widest of several captions = %v, want it wider than a single short one (%v)", widest, solo)
	}

	testApp.Settings().SetTheme(test.NewTheme())
	if got := captionColumnWidth("Short", "A Much Longer Caption"); got <= 0 {
		t.Errorf("under a different theme: got %v, want > 0", got)
	}
}
