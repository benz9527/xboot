package dlock

import "time"

type DLocker interface {
	Lock() error
	Unlock() error
	Renewal(newTTL time.Duration) error
	TTL() (time.Duration, error)
}

type RetryStrategy interface {
	Next() time.Duration
}

type DLockErr string

const (
	ErrDLockAcquireFailed DLockErr = "failed to acquire dlock"
	ErrDLockNoInit        DLockErr = "no init the dlock"
)

var (
	noErr error = nil
)

func (err DLockErr) Error() string {
	return string(err)
}
