// Script_exporter is a Prometheus exporter to execute programs and
// scripts and collect metrics from their output and their exit
// status.
package main

import (
	"github.com/ricoberger/script_exporter/pkg/exporter"
)

func main() {
	e := exporter.InitExporter()

	e.Serve()
}
