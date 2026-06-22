package ui

import (
	"io"
	"net"
	"strings"
	"time"

	"fyne.io/fyne/v2"
)

const singleInstanceAddress = "127.0.0.1:37653"
const singleInstanceShowCommand = "show"

func acquireSingleInstance(showExisting bool) (net.Listener, bool) {
	listener, err := net.Listen("tcp", singleInstanceAddress)
	if err == nil {
		return listener, true
	}

	connection, dialErr := net.DialTimeout("tcp", singleInstanceAddress, time.Second)
	if dialErr == nil {
		// The first instance listens only on localhost and understands one tiny
		// command: "show". That keeps the implementation dependency-free and easy
		// to inspect, which matters more here than introducing a named-pipe or
		// platform-specific IPC abstraction just to focus an existing window.
		if showExisting {
			_, _ = io.WriteString(connection, singleInstanceShowCommand)
		}
		_ = connection.Close()
		return nil, false
	}

	// If the port is unavailable but does not answer as GoSentry, continue
	// startup instead of making the application impossible to open because of an
	// unrelated local listener. In the normal duplicate-start case the dial above
	// succeeds and this process exits after waking the first instance.
	return nil, true
}

func serveSingleInstance(listener net.Listener, w fyne.Window) {
	if listener == nil {
		return
	}
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			command, _ := io.ReadAll(io.LimitReader(connection, 32))
			_ = connection.Close()
			if strings.TrimSpace(string(command)) != singleInstanceShowCommand {
				continue
			}
			// Accept runs on its own goroutine, so focusing the window must be
			// marshaled onto the main thread like every other widget update.
			fyne.Do(func() {
				w.Show()
				w.RequestFocus()
			})
		}
	}()
}
