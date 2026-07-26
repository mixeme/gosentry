//go:build linux

package filemanager

// openCommand returns the XDG invocation for dir. xdg-open picks whichever
// file manager the desktop environment has registered for directories.
func openCommand(dir string) (string, []string) {
	return "xdg-open", []string{dir}
}
