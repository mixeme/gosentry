package app

import (
	"gitea.mixdep.ru/mix/gosentry/src/platform/desktop"
)

// InstallDesktopIcon installs the application's .desktop file and icon on
// Linux (no-op on other platforms). The resulting icon path is stored in
// store.Paths.DesktopIcon so ApplyAutostart can reference it.
func (s *Service) InstallDesktopIcon(appID string, iconBytes []byte) {
	if iconPath, err := desktop.InstallDesktopIntegration(appID, s.store.Paths.ExecutablePath, iconBytes); err == nil {
		s.mu.Lock()
		s.store.Paths.DesktopIcon = iconPath
		s.mu.Unlock()
	}
}

// AutostartStatus reports whether the platform autostart entry matches the
// current StartOnLogin setting in the stored config.
func (s *Service) AutostartStatus() (ok bool, message string) {
	s.mu.Lock()
	enabled := s.store.Config.StartOnLogin
	execPath := s.store.Paths.ExecutablePath
	manager := s.manager
	s.mu.Unlock()
	if manager == nil {
		return false, "autostart not available"
	}
	return manager.Status(enabled, execPath)
}

// ApplyAutostart writes or removes the platform autostart entry to match the
// current StartOnLogin setting in the stored config. Call after UpdateSettings.
func (s *Service) ApplyAutostart() error {
	s.mu.Lock()
	enabled := s.store.Config.StartOnLogin
	execPath := s.store.Paths.ExecutablePath
	iconPath := s.store.Paths.DesktopIcon
	manager := s.manager
	s.mu.Unlock()
	if manager == nil {
		return nil
	}
	return manager.Set(enabled, execPath, iconPath)
}
