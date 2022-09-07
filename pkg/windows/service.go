//go:build windows
// +build windows

package windows

import (
	"fmt"
	"log"

	"golang.org/x/sys/windows/svc"
)

// WindowsExporterService channel for service stop
type WindowsExporterService struct {
	stopCh chan<- bool
}

// NewWindowsExporterService return new WindowsExporterService
func NewWindowsExporterService(ch chan<- bool) *WindowsExporterService {
	return &WindowsExporterService{stopCh: ch}
}

// Execute run programm directly or for service
func (s *WindowsExporterService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
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
				s.stopCh <- true
				break loop
			default:
				log.Fatalf(fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}
