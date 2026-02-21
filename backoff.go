package treesitter

import (
	"math/rand"
	"time"
)

// Backoff implements "full jitter" truncated exponential backoff:
//
//	sleep = random_between(0, min(cap, base * 2^attempt))
//
// This gives good spread even when a large cohort of goroutines all start
// together: every goroutine independently picks a uniform random value in
// [0, ceiling], so they naturally scatter across the whole window instead
// of clustering near the ceiling like additive jitter does.
//
// The zero value is ready to use; set Base and Cap before the first call
// to Next().
type Backoff struct {
	Base    time.Duration
	Cap     time.Duration
	attempt int
}

// Next advances the attempt counter and returns a random duration in
// [0, min(cap, base*2^attempt)].
func (b *Backoff) Next() time.Duration {
	// Compute ceiling = base * 2^attempt, capped to avoid overflow.
	// We cap the exponent at 62 so the shift never overflows int64.
	exp := b.attempt
	if exp > 62 {
		exp = 62
	}
	ceiling := b.Base << uint(exp)
	if ceiling <= 0 || ceiling > b.Cap { // overflow or over cap
		ceiling = b.Cap
	}
	b.attempt++
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(ceiling) + 1))
}

// Reset restores the Backoff to its initial state.  Call after a successful
// attempt so the next failure starts from the beginning again.
func (b *Backoff) Reset() {
	b.attempt = 0
}
