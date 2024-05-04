package dlock

// References:
// https://etcd.io/docs/v3.5/dev-guide/api_reference_v3/
// https://github.com/etcd-io/etcd

import (
	"context"
	"sync/atomic"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	concv3 "go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/multierr"

	"github.com/benz9527/xboot/lib/infra"
)

var _ DLocker = (*etcdDLock)(nil)

type etcdDLock struct {
	*etcdDLockOptions
	session   *concv3.Session
	mutexes   []*concv3.Mutex
	startTime time.Time
}

func (dl *etcdDLock) Lock() error {
	if len(dl.mutexes) < 1 {
		return infra.NewErrorStack("etcd dlock is not initialized")
	}

	var (
		merr                error
		fallbackUnlockIndex int
		retry               = dl.strategy
		ticker              *time.Ticker
	)
	// TODO pay attention to the context has been cancelled.
	for {
		for i, mu := range dl.mutexes {
			if err := /* reentrant */ mu.TryLock(*dl.ctx.Load()); err != nil {
				merr = multierr.Append(merr, err)
				fallbackUnlockIndex = i
				break
			}
		}

		if merr == nil {
			return noErr
		} else {
			for i := fallbackUnlockIndex - 1; i >= 0; i-- {
				if err := dl.mutexes[i].Unlock(*dl.ctx.Load()); err != nil {
					return err
				}
			}
		}
		// Retry.
		backoff := retry.Next()
		if backoff.Milliseconds() < 1 {
			if ticker != nil {
				return infra.WrapErrorStackWithMessage(multierr.Combine(merr, ErrDLockAcquireFailed), "etcd dlock lock retry reach to max")
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

func (dl *etcdDLock) Renewal(newTTL time.Duration) error {
	return infra.NewErrorStack("etcd dlock not support to refresh ttl")
}

func (dl *etcdDLock) TTL() (time.Duration, error) {
	if dl.startTime.IsZero() {
		return 0, infra.NewErrorStack("etcd dlock is not initialized or has been  closed")
	}
	diff := time.Now().Sub(dl.startTime)
	return dl.ttl - diff, nil
}

func (dl *etcdDLock) Unlock() error {
	if len(dl.mutexes) < 1 {
		return infra.NewErrorStack("etcd dlock is not initialized")
	}
	var merr error
	// TODO pay attention to the context has been cancelled.
	for _, mu := range dl.mutexes {
		if err := mu.Unlock(*dl.ctx.Load()); err != nil {
			merr = multierr.Append(merr, err)
		}
	}
	if merr == nil && dl.session != nil {
		merr = multierr.Append(merr, dl.session.Close())
		if cancelPtr := dl.ctxCancel.Load(); cancelPtr != nil {
			(*cancelPtr)()
		}
		dl.startTime = time.Time{} // Zero time
	}
	return merr
}

type etcdDLockOptions struct {
	client    *clientv3.Client
	ctx       atomic.Pointer[context.Context]
	ctxCancel atomic.Pointer[context.CancelFunc]
	strategy  RetryStrategy
	keys      []string
	ttl       time.Duration
	leaseID   *clientv3.LeaseID // Used as token (must be unique)
}

func EtcdDLockBuilder(ctx context.Context, client *clientv3.Client) *etcdDLockOptions {
	if ctx == nil {
		ctx = context.Background()
	}
	opts := &etcdDLockOptions{client: client}
	opts.ctx.Store(&ctx)
	return opts
}

func (opt *etcdDLockOptions) TTL(ttl time.Duration) *etcdDLockOptions {
	opt.ttl = ttl
	return opt
}

func (opt *etcdDLockOptions) Keys(keys ...string) *etcdDLockOptions {
	opt.keys = make([]string, len(keys))
	for i, key := range keys {
		opt.keys[i] = key
	}
	return opt
}

func (opt *etcdDLockOptions) LeaseID(id int64) *etcdDLockOptions {
	leaseID := clientv3.LeaseID(id)
	opt.leaseID = &leaseID
	return opt
}

func (opt *etcdDLockOptions) Retry(strategy RetryStrategy) *etcdDLockOptions {
	opt.strategy = strategy
	return opt
}

func (opt *etcdDLockOptions) Build() (DLocker, error) {
	if opt.client == nil {
		return nil, infra.NewErrorStack("etcd dlock client is nil")
	}
	if opt.ttl.Seconds() < 1 {
		return nil, infra.NewErrorStack("etcd dlock with zero second TTL")
	}
	if len(opt.keys) <= 0 {
		return nil, infra.NewErrorStack("etcd dlock with zero keys")
	}
	if opt.strategy == nil {
		opt.strategy = NoRetry()
	}
	if opt.leaseID == nil {
		noLease := clientv3.NoLease
		opt.leaseID = &noLease // Grant new lease ID by new session.
	}

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	ctx, cancel = context.WithTimeout(*opt.ctx.Load(), opt.ttl)
	if ctx == nil || cancel == nil {
		return nil, infra.NewErrorStack("etcd dlock build with nil context or nil context cancel function")
	}
	startTime := time.Now()
	opt.ctx.Store(&ctx)
	opt.ctxCancel.Store(&cancel)

	session, err := concv3.NewSession(opt.client,
		concv3.WithTTL(int(opt.ttl.Seconds())),
		concv3.WithLease(*opt.leaseID),
	)
	if err != nil {
		return nil, infra.WrapErrorStack(err)
	}
	var mutexes = make([]*concv3.Mutex, 0, len(opt.keys))
	for _, prefix := range opt.keys {
		mutexes = append(mutexes, concv3.NewMutex(session, prefix))
	}
	return &etcdDLock{
		etcdDLockOptions: opt,
		session:          session,
		startTime:        startTime,
		mutexes:          mutexes,
	}, nil
}

type EtcdDLockOption func(opt *etcdDLockOptions)

func WithEtcdDLockTTL(ttl time.Duration) EtcdDLockOption {
	return func(opt *etcdDLockOptions) {
		opt.TTL(ttl)
	}
}

func WithEtcdDLockKeys(keys ...string) EtcdDLockOption {
	return func(opt *etcdDLockOptions) {
		opt.Keys(keys...)
	}
}

func WithEtcdDLockLeaseID(id int64) EtcdDLockOption {
	return func(opt *etcdDLockOptions) {
		opt.LeaseID(id)
	}
}

func WithEtcdDLockRetry(strategy RetryStrategy) EtcdDLockOption {
	return func(opt *etcdDLockOptions) {
		opt.Retry(strategy)
	}
}

func EtcdDLock(ctx context.Context, client *clientv3.Client, opts ...EtcdDLockOption) (DLocker, error) {
	builderOpts := EtcdDLockBuilder(ctx, client)
	for _, o := range opts {
		o(builderOpts)
	}
	return builderOpts.Build()
}
