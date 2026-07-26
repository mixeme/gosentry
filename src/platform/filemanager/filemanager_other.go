//go:build !windows && !linux

package filemanager

// openCommand has no handler to name on platforms GoSentry does not ship for.
// An empty name makes Open report that the action is unavailable instead of
// running something arbitrary.
func openCommand(dir string) (string, []string) {
	return "", nil
}
