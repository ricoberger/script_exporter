//go:build windows
// +build windows

package main

import (
	"log/slog"
	"os"

	"golang.org/x/sys/windows/svc"
)

type windowsService struct {
	stopCh chan<- bool
}

func (ws *windowsService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				ws.stopCh <- true
				break loop
			default:
				slog.Warn("Unexpected control request", slog.Any("request", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func main() {
	isService, err := svc.IsWindowsService()
	if err != nil {
		slog.Error("Failed to determine if Script Exporter is executed as Windows service", slog.Any("error", err))
		os.Exit(1)
	}

	stopCh := make(chan bool)
	if isService {
		go func() {
			err = svc.Run("script_exporter", &windowsService{stopCh: stopCh})
			if err != nil {
				slog.Error("Failed to run Windows service", slog.Any("error", err))
			}
		}()
	}

	os.Exit(run(stopCh))
}
