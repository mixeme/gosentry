//go:build !linux

package core

func InstallDesktopIntegration(appID string, executablePath string, icon []byte) (string, error) {
	return "", nil
}
