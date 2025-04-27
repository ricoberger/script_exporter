//go:build darwin || linux
// +build darwin linux

package main

import (
	"os"
)

func main() {
	stopCh := make(chan bool)
	os.Exit(run(stopCh))
}
