package dlock

import (
	"testing"
	"time"
)

func TestBackoffRetry(t *testing.T) {
	for i := range 5 {
		t.Log("default ex backoff", i, DefaultExponentialBackoffRetry().Next())
	}
	for i := range 5 {
		t.Log("endless linear backoff", i, EndlessRetry(20*time.Millisecond).Next())
	}
	exBackoff := ExponentialBackoffRetry(5, 10*time.Millisecond, 100*time.Millisecond, 0.0, 0.5)
	for i := range 6 {
		t.Log("ex backoff", i, exBackoff.Next())
	}
	exBackoff = ExponentialBackoffRetry(5, 10*time.Millisecond, 100*time.Millisecond, 2.0, 0.5)
	for i := range 6 {
		t.Log("ex backoff2", i, exBackoff.Next())
	}
	limitedRetry := LimitedRetry(20*time.Millisecond, 5)
	for i := range 6 {
		t.Log("limited backoff", i, limitedRetry.Next())
	}
	limitedRetry = LimitedRetry(0, 5)
	for i := range 6 {
		t.Log("limited backoff 2", i, limitedRetry.Next())
	}
}
