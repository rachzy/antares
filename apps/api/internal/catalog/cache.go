package catalog

import (
	"context"
	"sync"
	"time"
)

const maxCached = 1024

// Cached wraps a Source and remembers successful lookups for a fixed time, so reopening a star does
// not query the public services again. Failures are never cached.
type Cached struct {
	Source Source
	TTL    time.Duration
	Clock  func() time.Time

	mu      sync.Mutex
	entries map[Query]cacheEntry
}

type cacheEntry struct {
	star    Star
	expires time.Time
}

// Lookup returns the cached star for q, or asks the wrapped Source.
func (c *Cached) Lookup(ctx context.Context, q Query) (Star, error) {
	now := c.Clock()
	c.mu.Lock()
	if e, ok := c.entries[q]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return e.star, nil
	}
	c.mu.Unlock()

	star, err := c.Source.Lookup(ctx, q)
	if err != nil {
		return Star{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[Query]cacheEntry{}
	}
	if len(c.entries) >= maxCached {
		for k, e := range c.entries {
			if !now.Before(e.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= maxCached {
			clear(c.entries) // bounded memory matters more than keeping every entry
		}
	}
	c.entries[q] = cacheEntry{star: star, expires: now.Add(c.TTL)}
	return star, nil
}
