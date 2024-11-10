package exporter

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

var cache map[string]cacheEntry
var cacheLock = sync.RWMutex{}

type cacheEntry struct {
	cacheTime       time.Time
	formattedOutput string
	exitCode        int
	successStatus   int
}

func getCacheResult(scriptName string, paramValues []string, cacheDuration time.Duration, expCacheOnTimeout bool) (*string, *int, *int) {
	cacheLock.RLock()
	defer cacheLock.RUnlock()

	if entry, ok := cache[fmt.Sprintf("%s--%s", scriptName, strings.Join(paramValues, "-"))]; ok {
		if entry.cacheTime.Add(cacheDuration).After(time.Now()) || expCacheOnTimeout {
			return &entry.formattedOutput, &entry.successStatus, &entry.exitCode
		}
	}

	return nil, nil, nil
}

func setCacheResult(scriptName string, paramValues []string, formattedOutput string, successStatus, exitCode int) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if cache == nil {
		cache = make(map[string]cacheEntry)
	}

	cache[fmt.Sprintf("%s--%s", scriptName, strings.Join(paramValues, "-"))] = cacheEntry{
		cacheTime:       time.Now(),
		formattedOutput: formattedOutput,
		exitCode:        exitCode,
		successStatus:   successStatus,
	}
}
