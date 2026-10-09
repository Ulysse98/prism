package main

import (
	"sync"
	"time"
)

const (
	quantumAuditRateLimit  = 6
	quantumAuditRateWindow = time.Minute
)

// quantumAuditRateLimiter keeps a bounded rolling window of
// authenticated audit requests. Its zero value is ready to use.
type quantumAuditRateLimiter struct {
	mu       sync.Mutex
	arrivals []time.Time
}

func (limiter *quantumAuditRateLimiter) allow(
	now time.Time,
) (bool, time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	cutoff := now.Add(-quantumAuditRateWindow)

	kept := limiter.arrivals[:0]

	for _, arrival := range limiter.arrivals {
		if arrival.After(cutoff) {
			kept = append(kept, arrival)
		}
	}

	limiter.arrivals = kept

	if len(limiter.arrivals) >= quantumAuditRateLimit {
		retry := limiter.arrivals[0].
			Add(quantumAuditRateWindow).
			Sub(now)

		if retry <= 0 {
			retry = time.Second
		}

		return false, retry
	}

	limiter.arrivals = append(limiter.arrivals, now)

	return true, 0
}
