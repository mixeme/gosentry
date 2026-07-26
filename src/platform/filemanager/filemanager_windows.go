package filemanager

import "path/filepath"

// openCommand returns the Explorer invocation for dir. The path is cleaned
// because Explorer ignores an argument that mixes separators, and it is passed
// as a single argument so spaces need no quoting.
func openCommand(dir string) (string, []string) {
	return "explorer", []string{filepath.Clean(dir)}
}
