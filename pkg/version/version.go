package version

import (
	"runtime"
)

var (
	// Version contains the version which is defined in the MAKEFILE
	Version string

	// BuildTime is the time when the binaries where build
	BuildTime string

	// BuildUser is the name of the user which build the binaries
	BuildUser string

	// GitCommit is the git commit on which the binaries where build
	GitCommit string

	// GoVersion is the version of go with which the program was build
	GoVersion = runtime.Version()

	// Author hold author information
	Author = "Rico Berger <mail@ricoberger.de>"
)
