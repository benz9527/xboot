package dlock

import (
	randv2 "math/rand/v2"
	"sync/atomic"
	"time"
)

type linearBackoff time.Duration

func (backoff linearBackoff) Next() time.Duration {
	return time.Duration(backoff)
}

func EndlessRetry(backoff time.Duration) RetryStrategy {
	return linearBackoff(backoff)
}

func NoRetry() RetryStrategy {
	return linearBackoff(0)
}

type limitedRetry struct {
	strategy RetryStrategy
	count    int64
	maxCount int64
}

func (retry *limitedRetry) Next() time.Duration {
	if atomic.LoadInt64(&retry.count) >= retry.maxCount {
		return 0
	}
	atomic.AddInt64(&retry.count, 1)
	return retry.strategy.Next()
}

func LimitedRetry(backoff time.Duration, maxCount int64) RetryStrategy {
	if backoff.Milliseconds() <= 0 {
		return NoRetry()
	}
	return &limitedRetry{
		strategy: ExponentialBackoffRetry(
			maxCount,
			backoff,
			0,
			1.0,
			0.1,
		),
		maxCount: maxCount,
	}
}

type exponentialBackoff struct {
	duration time.Duration
	factor   float64
	jitter   float64
	steps    int64
	cap      time.Duration
}

func (backoff *exponentialBackoff) Next() time.Duration {
	if atomic.LoadInt64(&backoff.steps) < 1 {
		return NoRetry().Next()
	}
	atomic.AddInt64(&backoff.steps, -1)
	duration := backoff.duration
	if backoff.factor != 0 {
		backoff.duration = time.Duration(float64(backoff.duration) * backoff.factor)
		if backoff.cap > 0 && backoff.duration > backoff.cap {
			backoff.duration = backoff.cap
			atomic.SwapInt64(&backoff.steps, 0)
		}
	}
	if backoff.jitter > 0 {
		duration = duration + time.Duration(randv2.Float64()*backoff.jitter*float64(duration))
	}
	return duration
}

func ExponentialBackoffRetry(maxSteps int64, initBackoff, maxBackoff time.Duration, backoffFactor, jitter float64) RetryStrategy {
	return &exponentialBackoff{
		cap:      maxBackoff,
		duration: initBackoff,
		factor:   backoffFactor,
		jitter:   jitter,
		steps:    maxSteps,
	}
}

func DefaultExponentialBackoffRetry() RetryStrategy {
	return ExponentialBackoffRetry(
		5,
		10*time.Millisecond,
		0,
		1.0,
		0.1,
	)
}
