package api

import (
	"sync"
	"time"
)

type idemEntry struct {
	jobID     string
	expiresAt time.Time
}

// idempotency stores idempotency key -> job id with ttl
type idempotency struct {
	mu   sync.RWMutex
	keys map[string]idemEntry
	ttl  time.Duration
}

func newIdempotency(ttl time.Duration) *idempotency {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &idempotency{keys: make(map[string]idemEntry), ttl: ttl}
}

func (i *idempotency) get(key string) (jobID string, ok bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	e, ok := i.keys[key]
	if !ok || time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.jobID, true
}

func (i *idempotency) set(key, jobID string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.keys[key] = idemEntry{jobID: jobID, expiresAt: time.Now().Add(i.ttl)}
}
