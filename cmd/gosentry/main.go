package main

import (
	"os"

	"gitea.mixdep.ru/mix/gosentry/src/domain"
	"gitea.mixdep.ru/mix/gosentry/src/ui"
)

func main() {
	// The executable entry point intentionally delegates all startup work to the
	// UI package. Keeping main small makes it easier to add platform-specific
	// packaging later without mixing window setup, storage, and scheduler logic.
	ui.Run(hasArgument(domain.StartInTrayArgument))
}

func hasArgument(argument string) bool {
	for _, current := range os.Args[1:] {
		if current == argument {
			return true
		}
	}
	return false
}
