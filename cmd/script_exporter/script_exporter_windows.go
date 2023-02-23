//go:build windows
// +build windows

// Script_exporter is a Prometheus exporter to execute programs and
// scripts and collect metrics from their output and their exit
// status.
package main

import (
	"os"

	"github.com/go-kit/log/level"

	"github.com/ricoberger/script_exporter/pkg/exporter"
	win "github.com/ricoberger/script_exporter/pkg/windows"
	"golang.org/x/sys/windows/svc"
)

func main() {
	e := exporter.InitExporter()

	isService, err := svc.IsWindowsService()
	if err != nil {
		level.Error(e.Logger).Log("err", err)
		os.Exit(1)
	}

	stopCh := make(chan bool)
	if isService {
		go func() {
			err = svc.Run("Script Exporter", win.NewWindowsExporterService(stopCh))
			if err != nil {
				level.Error(e.Logger).Log("msg", "Failed to start service", "err", err)
				os.Exit(1)
			}
		}()
	}

	go func() {
		e.Serve()
	}()

	for {
		if <-stopCh {
			level.Info(e.Logger).Log("msg", "Shutting down Script Exporter")
			break
		}
	}

}
