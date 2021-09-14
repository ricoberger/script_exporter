//go:build windows
// +build windows

// Script_exporter is a Prometheus exporter to execute programs and
// scripts and collect metrics from their output and their exit
// status.
package main

import (
	"log"

	"github.com/ricoberger/script_exporter/pkg/exporter"
	win "github.com/ricoberger/script_exporter/pkg/windows"
	"golang.org/x/sys/windows/svc"
)

func main() {
	e := exporter.InitExporter()

	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatal(err)
	}

	stopCh := make(chan bool)
	if !isInteractive {
		go func() {
			err = svc.Run("Script Exporter", win.NewWindowsExporterService(stopCh))
			if err != nil {
				log.Fatalf("Failed to start service: %v", err)
			}
		}()
	}

	go func() {
		e.Serve()
	}()

	for {
		if <-stopCh {
			log.Printf("Shutting down %s", "Script Exporter")
			break
		}
	}

}
