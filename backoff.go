package treesitter

import (
	"math/rand"
	"time"
)

// backoff implements "full jitter" truncated exponential backoff:
//
//	sleep = random_between(0, min(cap, base * 2^attempt))
//
// This gives good spread even when a large cohort of goroutines all start
// together: every goroutine independently picks a uniform random value in
// [0, ceiling], so they naturally scatter across the whole window instead
// of clustering near the ceiling like additive jitter does.
//
// The zero value is ready to use; set base and cap before the first call
// to next().
type backoff struct {
	base    time.Duration
	cap     time.Duration
	attempt int
}

// next advances the attempt counter and returns a random duration in
// [0, min(cap, base*2^attempt)].
func (b *backoff) next() time.Duration {
	// Compute ceiling = base * 2^attempt, capped to avoid overflow.
	// We cap the exponent at 62 so the shift never overflows int64.
	exp := b.attempt
	if exp > 62 {
		exp = 62
	}
	ceiling := b.base << uint(exp)
	if ceiling <= 0 || ceiling > b.cap { // overflow or over cap
		ceiling = b.cap
	}
	b.attempt++
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(ceiling) + 1))
}

// reset restores the backoff to its initial state.  Call after a successful
// attempt so the next failure starts from the beginning again.
func (b *backoff) reset() {
	b.attempt = 0
}
