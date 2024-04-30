package dlock

import (
	"context"
	_ "embed"
	"errors"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
)

//go:embedded lock.lua
var luaDLockAcquireScript string

//go:embedded unlock.lua
var luaDLockReleaseScript string

//go:embedded lockrenewal.lua
var luaDLockRenewalTTLScript string

//go:embedded lockttl.lua
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
}

func (dl *redisDLock) Close() error {
	if cancel := *dl.ctxCancel.Load(); cancel != nil {
		cancel()
	}
	return nil
}

func (dl *redisDLock) Lock() error {
	retry := dl.strategy
	var ticker *time.Ticker
	for {
		if _, err := luaDLockAcquire.EvalSha(
			*dl.ctx.Load(),
			dl.scripterLoader(),
			dl.keys,
			dl.token, dl.ttl.Milliseconds(),
		).Result(); err != nil {
			if errors.Is(err, redis.Nil) {
				return nil
			}
			return infra.WrapErrorStackWithMessage(err, "acquire redis lock failed")
		}

		backoff := retry.Next()
		if backoff.Milliseconds() < 1 { // No retry strategy.
			return infra.WrapErrorStack(ErrDLockAcquireFailed)
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
		return infra.NewErrorStack("[redis-dlock] renewal lock with zero ms TTL")
	}
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	ctx, cancel = context.WithTimeout(*dl.ctx.Load(), newTTL)
	if ctx != nil && cancel != nil {
		if _, err := luaDLockRenewalTTL.EvalSha(
			ctx,
			dl.scripterLoader(),
			dl.keys,
			dl.token, newTTL.Milliseconds(),
		).Result(); err != nil {
			return err
		}
		dl.ctx.Store(&ctx)
		dl.ctxCancel.Store(&cancel)
	}
	return infra.NewErrorStack("[redis-dlock] renewal with nil context or nil context cancel function")
}

func (dl *redisDLock) TTL() (time.Duration, error) {
	res, err := luaDLockLoadTTL.EvalSha(
		*dl.ctx.Load(),
		dl.scripterLoader(),
		dl.keys,
		dl.token,
	).Result()
	if err != nil {
		return 0, err
	}
	if num := res.(int64); num > 0 {
		return time.Duration(num) * time.Millisecond, nil
	}
	return 0, nil
}

func (dl *redisDLock) Unlock() error {
	if _, err := luaDLockRelease.EvalSha(
		*dl.ctx.Load(),
		dl.scripterLoader(),
		dl.keys,
		dl.token,
	).Result(); err != nil {
		return err
	}
	return dl.Close()
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

func RedisLockBuilder(ctx context.Context, scripter func() redis.Scripter) *redisDLockOptions {
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
	opt.keys = keys
	return opt
}

func (opt *redisDLockOptions) Retry(strategy RetryStrategy) *redisDLockOptions {
	opt.strategy = strategy
	return opt
}

func (opt *redisDLockOptions) Build() (DLocker, error) {
	if opt.scripterLoader == nil {
		return nil, infra.NewErrorStack("[redis-dlock] scripter loader is nil")
	}
	if opt.ttl.Milliseconds() <= 0 {
		return nil, infra.NewErrorStack("[redis-dlock] lock with zero ms TTL")
	}
	if len(opt.keys) <= 0 {
		return nil, infra.NewErrorStack("[redis-dlock] lock with zero keys")
	}
	if _, ok := (*opt.ctx.Load()).Deadline(); !ok || opt.ctxCancel.Load() == nil {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)
		ctx, cancel = context.WithTimeout(*opt.ctx.Load(), opt.ttl)
		if ctx == nil || cancel == nil {
			return nil, infra.NewErrorStack("[redis-dlock] build with nil context or nil context cancel function")
		}
		opt.ctx.Store(&ctx)
		opt.ctxCancel.Store(&cancel)
	}
	if opt.strategy == nil {
		opt.strategy = NoRetry()
	}
	return &redisDLock{redisDLockOptions: opt}, nil
}
