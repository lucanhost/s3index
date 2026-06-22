package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/TwiN/gocache/v2"
)

type APICache struct {
	cache *gocache.Cache
	ttl   time.Duration
}

var apiCache *APICache

func initCache() {
	// Parse Max Memory Limit (bytes) or fallback to 50MB
	maxMemory := parseMemoryLimit(globalConfig.ApiCacheMaxMemory)

	log.Printf("Initializing API LRU Cache: max_entries=%d, ttl=%s, max_memory=%s (%d bytes)", globalConfig.ApiCacheSize, globalConfig.ApiCacheTTL, formatBytes(maxMemory), maxMemory)

	// Initialize gocache v2
	c := gocache.NewCache().
		WithMaxSize(globalConfig.ApiCacheSize).
		WithMaxMemoryUsage(int(maxMemory)).
		WithEvictionPolicy(gocache.LeastRecentlyUsed)

	// Start the Janitor for active background expiration cleaning
	c.StartJanitor()

	apiCache = &APICache{
		cache: c,
		ttl:   globalConfig.ApiCacheTTL,
	}
}

func (c *APICache) Get(key string) (any, bool) {
	if c == nil || c.cache == nil {
		return nil, false
	}
	return c.cache.Get(key)
}

func (c *APICache) Set(key string, value any) {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.SetWithTTL(key, value, c.ttl)
}

func (c *APICache) Delete(key string) {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Delete(key)
}

func parseMemoryLimit(val string) int64 {
	val = strings.ToUpper(strings.TrimSpace(val))
	if val == "" {
		return 50 * 1024 * 1024 // default 50 MB
	}

	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(val, "GB") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(val, "GB")
	} else if strings.HasSuffix(val, "MB") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(val, "MB")
	} else if strings.HasSuffix(val, "KB") {
		multiplier = 1024
		numStr = strings.TrimSuffix(val, "KB")
	} else if strings.HasSuffix(val, "B") {
		numStr = strings.TrimSuffix(val, "B")
	} else {
		numStr = val
	}

	numStr = strings.TrimSpace(numStr)
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil || num <= 0 {
		return 50 * 1024 * 1024 // fallback to 50 MB
	}

	return num * multiplier
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := ""
	switch exp {
	case 0:
		suffix = "KB"
	case 1:
		suffix = "MB"
	case 2:
		suffix = "GB"
	default:
		suffix = "TB"
	}
	return fmt.Sprintf("%.2f %s", float64(bytes)/float64(div), suffix)
}
