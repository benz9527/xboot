package dlock

import "time"

type DLocker interface {
	Lock() error
	Unlock() error
	Renewal(newTTL time.Duration) error
	TTL() (time.Duration, error)
	Close() error
}

type RetryStrategy interface {
	Next() time.Duration
}

type DLockErr string

const (
	ErrDLockAcquireFailed DLockErr = "failed to acquire lock"
)

func (err DLockErr) Error() string {
	return string(err)
}
