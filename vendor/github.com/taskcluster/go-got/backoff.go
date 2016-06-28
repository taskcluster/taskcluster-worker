package got

import (
	"math"
	"math/rand"
	"time"
)

// BackOff configuration for retries
type BackOff struct {
	DelayFactor         time.Duration
	RandomizationFactor float64
	MaxDelay            time.Duration
}

// Delay computes the delay for exponential back-off given the number of
// attempts, tried so far. Use attemps = 0 for the first try, attempts = 1
// for the first retry and attempts = 2 for the second retry...
func (b BackOff) Delay(attempts int) time.Duration {
	// Zero attempts means we haven't retried yet, so no delay
	if attempts <= 0 {
		return 0
	}
	// We subtract one to get exponents: 1, 2, 3, 4, 5, ..
	delay := math.Pow(float64(2), float64(attempts-1)) * float64(b.DelayFactor)
	// Apply randomization factor
	delay = delay * (b.RandomizationFactor*(rand.Float64()*2-1) + 1)
	// Always limit with a maximum delay
	return time.Duration(math.Min(delay, float64(b.MaxDelay)))
}

// DefaultBackOff is a simple exponential backoff with delays as follows:
// 1. 400, range: [300; 500]
// 2. 100, range: [75; 125]
// 3. 200, range: [150; 250]
// 4. 800, range: [600; 1000]
var DefaultBackOff = &BackOff{
	DelayFactor:         100 * time.Millisecond,
	RandomizationFactor: 0.25,
	MaxDelay:            30 * time.Second,
}
