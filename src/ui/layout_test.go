package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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

// TestCancelRowOverlapAddsBackOneInnerPadding is the regression guard for the
// Settings tab's Theme row sitting flush against the Notifications checkbox:
// the wrapper must add exactly the padding rowOverlap removes, on the top edge
// only, so the row below is unaffected and the width does not change.
func TestCancelRowOverlapAddsBackOneInnerPadding(t *testing.T) {
	testApp := test.NewApp()
	defer testApp.Quit()

	child := widget.NewSelect([]string{"System"}, nil)
	wrapped := cancelRowOverlap(child)

	childMin, wrappedMin := child.MinSize(), wrapped.MinSize()
	if got, want := wrappedMin.Height, childMin.Height-rowOverlap(); got != want {
		t.Errorf("wrapped height = %v, want %v (child %v plus one inner padding)", got, want, childMin.Height)
	}
	if got, want := wrappedMin.Width, childMin.Width; got != want {
		t.Errorf("wrapped width = %v, want the child's %v", got, want)
	}

	wrapped.Resize(wrappedMin)
	if got, want := child.Position().Y, -rowOverlap(); got != want {
		t.Errorf("child Y = %v, want %v", got, want)
	}
	if got := child.Position().X; got != 0 {
		t.Errorf("child X = %v, want 0", got)
	}
	if got, want := child.Size().Height, childMin.Height; got != want {
		t.Errorf("child height = %v, want %v: the padding must not be taken out of the row", got, want)
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
