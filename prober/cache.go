package prober

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ricoberger/script_exporter/config"
)

var cache map[string]cacheEntry
var cacheLock = sync.RWMutex{}

type cacheEntry struct {
	cacheTime time.Time
	result    scriptResult
}

func getCacheKey(script *config.Script, scriptParamValues []string) string {
	return fmt.Sprintf("%s--%s", script.Name, strings.Join(scriptParamValues, "-"))
}

func getCacheResult(script *config.Script, scriptParamValues []string, useExpiredCache bool) *scriptResult {
	cacheLock.RLock()
	defer cacheLock.RUnlock()

	if script.Cache.Duration == nil {
		return nil
	}

	if entry, ok := cache[getCacheKey(script, scriptParamValues)]; ok {
		if entry.cacheTime.Add(time.Duration(*script.Cache.Duration*float64(time.Second))).After(time.Now()) || useExpiredCache {
			return &entry.result
		}
	}

	return nil
}

func setCacheResult(script *config.Script, scriptParamValues []string, result scriptResult) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if script.Cache.Duration == nil {
		return
	}

	if cache == nil {
		cache = make(map[string]cacheEntry)
	}

	cache[getCacheKey(script, scriptParamValues)] = cacheEntry{
		cacheTime: time.Now(),
		result:    result,
	}
}
