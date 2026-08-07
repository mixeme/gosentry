package app

import (
	"fmt"

	"gitea.mixdep.ru/mix/gosentry/src/platform/desktop"
)

// InstallDesktopIcon installs the application's .desktop file and icon on
// Linux (no-op on other platforms). The resulting icon path is stored in
// store.Paths.DesktopIcon so ApplyAutostart can reference it. A failure is
// reported through ErrorOccurred rather than discarded, so the visible symptom
// (a generic dock icon) has an explanation in History instead of none.
func (s *Service) InstallDesktopIcon(appID string, iconBytes []byte) {
	iconPath, err := desktop.InstallDesktopIntegration(appID, s.store.Paths.ExecutablePath, iconBytes)
	if err != nil {
		s.emit(ErrorOccurred{Err: fmt.Errorf("install desktop icon: %w", err)})
		return
	}
	s.mu.Lock()
	s.store.Paths.DesktopIcon = iconPath
	s.mu.Unlock()
}

// AutostartStatus reports whether the platform autostart entry matches the
// current StartOnLogin and KeepRunningInTray settings in the stored config.
func (s *Service) AutostartStatus() (ok bool, message string) {
	s.mu.Lock()
	enabled := s.store.Config.StartOnLogin
	startInTray := s.store.Config.KeepRunningInTray
	execPath := s.store.Paths.ExecutablePath
	manager := s.manager
	s.mu.Unlock()
	if manager == nil {
		return false, "autostart not available"
	}
	return manager.Status(enabled, startInTray, execPath)
}

// ApplyAutostart writes or removes the platform autostart entry to match the
// current StartOnLogin and KeepRunningInTray settings. Call after UpdateSettings.
func (s *Service) ApplyAutostart() error {
	s.mu.Lock()
	enabled := s.store.Config.StartOnLogin
	startInTray := s.store.Config.KeepRunningInTray
	execPath := s.store.Paths.ExecutablePath
	iconPath := s.store.Paths.DesktopIcon
	manager := s.manager
	s.mu.Unlock()
	if manager == nil {
		return nil
	}
	return manager.Set(enabled, startInTray, execPath, iconPath)
}
