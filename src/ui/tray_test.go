package ui

import "testing"

func TestResolveStartHiddenUsesDomainHelper(t *testing.T) {
	// resolveStartHidden is the UI alias used at startup; it must stay aligned
	// with domain.ResolveStartHidden so run.go and tests share one definition.
	if got := resolveStartHidden(true, false); got {
		t.Fatal("expected hidden start to require both CLI flag and keepInTray")
	}
	if !resolveStartHidden(true, true) {
		t.Fatal("expected hidden start when both flags are set")
	}
}
