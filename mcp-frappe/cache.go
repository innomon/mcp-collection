package main

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data      *FrappeDocType
	expiresAt time.Time
}

func (e *cacheEntry) expired(now time.Time) bool {
	return now.After(e.expiresAt)
}

// DocTypeCache is an in-memory TTL cache for FrappeDocType metadata.
type DocTypeCache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	store map[string]*cacheEntry
	now   func() time.Time // injectable for testing
}

// NewDocTypeCache returns a cache with the given TTL.
func NewDocTypeCache(ttl time.Duration) *DocTypeCache {
	return &DocTypeCache{
		ttl:   ttl,
		store: make(map[string]*cacheEntry),
		now:   time.Now,
	}
}

// Get retrieves a cached FrappeDocType by name.
func (c *DocTypeCache) Get(name string) (*FrappeDocType, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.store[name]
	if !ok || entry.expired(c.now()) {
		return nil, false
	}
	return entry.data, true
}

// Set stores a FrappeDocType in the cache with the configured TTL.
func (c *DocTypeCache) Set(name string, dt *FrappeDocType) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[name] = &cacheEntry{
		data:      dt,
		expiresAt: c.now().Add(c.ttl),
	}
}
