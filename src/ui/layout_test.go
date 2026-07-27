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
