//go:build darwin || linux || freebsd

package main

import (
	"os"
)

func main() {
	stopCh := make(chan bool)
	os.Exit(run(stopCh))
}
