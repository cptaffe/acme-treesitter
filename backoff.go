package treesitter

import (
	"math/rand"
	"time"
)

// backoff implements truncated binary exponential backoff with jitter.
// The zero value is ready to use with sensible defaults (min=0, max=0 means
// callers should set the fields before calling next).
type backoff struct {
	min     time.Duration
	max     time.Duration
	current time.Duration
}

// next advances the backoff and returns the duration to wait.
// On the first call it returns min; subsequent calls double the interval up
// to max.  Up to 25 % random jitter is added to spread retries from multiple
// goroutines.
func (b *backoff) next() time.Duration {
	if b.current < b.min {
		b.current = b.min
	} else {
		b.current *= 2
		if b.current > b.max {
			b.current = b.max
		}
	}
	// Add up to 25 % jitter so goroutines don't all wake simultaneously.
	jitter := time.Duration(rand.Int63n(int64(b.current)/4 + 1))
	return b.current + jitter
}

// reset restores the backoff to its initial state.  Call after a successful
// attempt so the next failure starts from min again.
func (b *backoff) reset() {
	b.current = 0
}
