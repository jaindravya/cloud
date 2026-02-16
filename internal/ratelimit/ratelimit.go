package ratelimit


import (
    "sync"
    "time"
)


// limiter is a fixed-window rate limiter (max n requests per window)
type Limiter struct {
    mu       sync.Mutex
    max      int
    count    int
    windowAt time.Time
    window   time.Duration
}


// new limiter returns a limiter that allows max requests per window
func NewLimiter(maxPerMinute int) *Limiter {
    if maxPerMinute <= 0 {
        maxPerMinute = 1000
    }
    return &Limiter{
        max:    maxPerMinute,
        window: time.Minute,
    }
}


// allow returns true if the request is allowed
func (l *Limiter) Allow() bool {
    l.mu.Lock()
    defer l.mu.Unlock()
    now := time.Now()
    if now.Sub(l.windowAt) >= l.window {
        l.windowAt = now
        l.count = 0
    }
    if l.count >= l.max {
        return false
    }
    l.count++
    return true
}





