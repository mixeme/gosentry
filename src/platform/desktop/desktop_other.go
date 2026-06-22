//go:build !linux

package desktop

func InstallDesktopIntegration(appID string, executablePath string, icon []byte) (string, error) {
	return "", nil
}
