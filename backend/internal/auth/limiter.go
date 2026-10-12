package auth

import (
	"sync"
	"time"
)

type attempts struct {
	count int
	until time.Time
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]attempts
}

// Bounded, process-local limiter. No forwarded header is trusted for client IP.
func (l *limiter) allow(ip, login string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, entry := range l.entries {
		if !now.Before(entry.until) {
			delete(l.entries, key)
		}
	}
	keys := []string{"ip:" + ip, "login:" + login}
	limits := []int{20, 5}
	missing := 0
	for i, key := range keys {
		if l.entries[key].count >= limits[i] {
			return false
		}
		if _, ok := l.entries[key]; !ok {
			missing++
		}
	}
	if len(l.entries)+missing > 10000 {
		return false
	}
	for _, key := range keys {
		entry := l.entries[key]
		if entry.count == 0 {
			entry.until = now.Add(15 * time.Minute)
		}
		entry.count++
		l.entries[key] = entry
	}
	return true
}
