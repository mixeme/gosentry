package main

import "github.com/pysentry/pysentry/src/gui"

func main() {
	// The executable entry point intentionally delegates all startup work to the
	// GUI package. Keeping main small makes it easier to add platform-specific
	// packaging later without mixing window setup, storage, and scheduler logic.
	gui.Run()
}
