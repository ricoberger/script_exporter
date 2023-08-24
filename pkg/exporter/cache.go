package exporter

import (
	"time"
)

var cache map[string]cacheEntry

type cacheEntry struct {
	cacheTime       time.Time
	formattedOutput string
	exitCode        int
	successStatus   int
}

func getCacheResult(scriptName string, cacheDuration time.Duration) (*string, *int, *int) {
	if entry, ok := cache[scriptName]; ok {
		if entry.cacheTime.Add(cacheDuration).After(time.Now()) {
			return &entry.formattedOutput, &entry.successStatus, &entry.exitCode
		}
	}

	return nil, nil, nil
}

func setCacheResult(scriptName, formattedOutput string, successStatus, exitCode int) {
	if cache == nil {
		cache = make(map[string]cacheEntry)
	}

	cache[scriptName] = cacheEntry{
		cacheTime:       time.Now(),
		formattedOutput: formattedOutput,
		exitCode:        exitCode,
		successStatus:   successStatus,
	}
}
