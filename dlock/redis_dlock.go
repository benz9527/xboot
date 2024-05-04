package dlock

// References:
// https://github.com/bsm/redislock
// https://github.com/go-redsync/redsync
// https://redis.io/docs/latest/develop/use/patterns/distributed-locks/

import (
	"context"
	_ "embed"
	"errors"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/multierr"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

//go:embed lock.lua
var luaDLockAcquireScript string

//go:embed unlock.lua
var luaDLockReleaseScript string

//go:embed lockrenewal.lua
var luaDLockRenewalTTLScript string

//go:embed lockttl.lua
var luaDLockLoadTTLScript string

var (
	luaDLockAcquire    = redis.NewScript(luaDLockAcquireScript)
	luaDLockRelease    = redis.NewScript(luaDLockReleaseScript)
	luaDLockRenewalTTL = redis.NewScript(luaDLockRenewalTTLScript)
	luaDLockLoadTTL    = redis.NewScript(luaDLockLoadTTLScript)
)

const randomTokenLength = 16

var nano, _ = id.ClassicNanoID(randomTokenLength)

var _ DLocker = (*redisDLock)(nil)

// TODO: Enable watchdog to renewal lock automatically
type redisDLock struct {
	*redisDLockOptions
	locked atomic.Bool
}

func (dl *redisDLock) Lock() error {
	retry := dl.strategy
	var (
		ticker *time.Ticker
		merr   error
	)
	for {
		if _, err := luaDLockAcquire.Eval(
			*dl.ctx.Load(), // TODO pay attention to the context has been cancelled.
			dl.scripterLoader(),
			dl.keys,
			dl.token, len(dl.token), dl.ttl.Milliseconds(),
		).Result(); err != nil && !errors.Is(err, redis.Nil) {
			merr = multierr.Append(merr, err)
		} else if err == nil || errors.Is(err, redis.Nil) {
			dl.locked.Store(true)
			return noErr
		}

		backoff := retry.Next()
		if backoff.Milliseconds() < 1 {
			if ticker != nil {
				return infra.WrapErrorStackWithMessage(multierr.Combine(merr, ErrDLockAcquireFailed), "redis dlock lock retry reach to max")
			}
			// No retry strategy.
			return noErr
		}

		if ticker == nil {
			ticker = time.NewTicker(backoff)
			defer ticker.Stop() // Avoid ticker leak.
		} else {
			ticker.Reset(backoff)
		}

		select {
		case <-(*dl.ctx.Load()).Done():
			return infra.WrapErrorStack((*dl.ctx.Load()).Err())
		case <-ticker.C:
			// continue
		}
	}
}

func (dl *redisDLock) Renewal(newTTL time.Duration) error {
	if newTTL.Milliseconds() <= 0 {
		return infra.NewErrorStack("renewal dlock with zero ms TTL")
	}
	if !dl.locked.Load() {
		return infra.WrapErrorStackWithMessage(ErrDLockNoInit, "renewal dlock with no lock")
	}
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	// TODO pay attention to the context has been cancelled.
	ctx, cancel = context.WithTimeout(*dl.ctx.Load(), newTTL)
	if ctx != nil && cancel != nil {
		if _, err := luaDLockRenewalTTL.Eval(
			ctx,
			dl.scripterLoader(),
			dl.keys,
			dl.token, newTTL.Milliseconds(),
		).Result(); err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
		dl.ctx.Store(&ctx)
		dl.ctxCancel.Store(&cancel)
		return noErr
	}
	return infra.NewErrorStack("refresh dlock ttl with nil context or nil context cancel function")
}

func (dl *redisDLock) TTL() (time.Duration, error) {
	if !dl.locked.Load() {
		return 0, infra.WrapErrorStackWithMessage(ErrDLockNoInit, "fetch dlock ttl failed")
	}
	res, err := luaDLockLoadTTL.Eval(
		*dl.ctx.Load(), // TODO pay attention to the context has been cancelled.
		dl.scripterLoader(),
		dl.keys,
		dl.token,
	).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}
	if res == nil {
		return 0, infra.NewErrorStack("no error but nil dlock ttl value")
	}
	if num := res.(int64); num > 0 {
		return time.Duration(num) * time.Millisecond, noErr
	}
	return 0, noErr
}

func (dl *redisDLock) Unlock() error {
	if !dl.locked.Load() {
		return infra.WrapErrorStackWithMessage(ErrDLockNoInit, "attempt to unlock a no init dlock")
	}
	if _, err := luaDLockRelease.Eval(
		*dl.ctx.Load(), // TODO pay attention to the context has been cancelled.
		dl.scripterLoader(),
		dl.keys,
		dl.token,
	).Result(); err != nil && !errors.Is(err, redis.Nil) {
		return err
	}
	if cancel := *dl.ctxCancel.Load(); cancel != nil {
		cancel()
	}
	return noErr
}

type redisDLockOptions struct {
	ctx            atomic.Pointer[context.Context]
	ctxCancel      atomic.Pointer[context.CancelFunc]
	scripterLoader func() redis.Scripter
	strategy       RetryStrategy
	keys           []string
	token          string
	ttl            time.Duration
}

func RedisDLockBuilder(ctx context.Context, scripter func() redis.Scripter) *redisDLockOptions {
	if ctx == nil {
		ctx = context.Background()
	}
	opts := &redisDLockOptions{scripterLoader: scripter}
	opts.ctx.Store(&ctx)
	return opts
}

func (opt *redisDLockOptions) TTL(ttl time.Duration) *redisDLockOptions {
	opt.ttl = ttl
	return opt
}

func (opt *redisDLockOptions) Token(token string) *redisDLockOptions {
	opt.token = token + "&" + nano()
	return opt
}

func (opt *redisDLockOptions) Keys(keys ...string) *redisDLockOptions {
	opt.keys = make([]string, len(keys))
	for i, key := range keys {
		opt.keys[i] = key
	}
	return opt
}

func (opt *redisDLockOptions) Retry(strategy RetryStrategy) *redisDLockOptions {
	opt.strategy = strategy
	return opt
}

func (opt *redisDLockOptions) Build() (DLocker, error) {
	if opt.scripterLoader == nil {
		return nil, infra.NewErrorStack("redis dlock scripter loader is nil")
	}
	if opt.ttl.Milliseconds() <= 0 {
		return nil, infra.NewErrorStack("redis dlock with zero ms TTL")
	}
	if len(opt.keys) <= 0 {
		return nil, infra.NewErrorStack("redis dlock with zero keys")
	}
	if opt.strategy == nil {
		opt.strategy = NoRetry()
	}
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	ctx, cancel = context.WithTimeout(*opt.ctx.Load(), opt.ttl)
	if ctx == nil || cancel == nil {
		return nil, infra.NewErrorStack("redis dlock build with nil context or nil context cancel function")
	}
	opt.ctx.Store(&ctx)
	opt.ctxCancel.Store(&cancel)
	return &redisDLock{redisDLockOptions: opt}, nil
}

type RedisDLockOption func(opt *redisDLockOptions)

func WithRedisDLockTTL(ttl time.Duration) RedisDLockOption {
	return func(opt *redisDLockOptions) {
		opt.TTL(ttl)
	}
}

func WithRedisDLockKeys(keys ...string) RedisDLockOption {
	return func(opt *redisDLockOptions) {
		opt.Keys(keys...)
	}
}

func WithRedisDLockToken(token string) RedisDLockOption {
	return func(opt *redisDLockOptions) {
		opt.Token(token)
	}
}

func WithRedisDLockRetry(strategy RetryStrategy) RedisDLockOption {
	return func(opt *redisDLockOptions) {
		opt.Retry(strategy)
	}
}

func RedisDLock(ctx context.Context, scripter func() redis.Scripter, opts ...RedisDLockOption) (DLocker, error) {
	builderOpts := RedisDLockBuilder(ctx, scripter)
	for _, o := range opts {
		o(builderOpts)
	}
	return builderOpts.Build()
}
