package dlock

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	mredisv2 "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisDLock_MiniRedis(t *testing.T) {
	require.NotEmpty(t, luaDLockAcquireScript)
	require.NotEmpty(t, luaDLockReleaseScript)
	require.NotEmpty(t, luaDLockRenewalTTLScript)
	require.NotEmpty(t, luaDLockLoadTTLScript)

	const addr = "127.0.0.1:6480"
	mredis := mredisv2.NewMiniRedis()
	defer func() { mredis.Close() }()
	go func() {
		err := mredis.StartAddr(addr)
		require.NoError(t, err)
	}()

	rclient := redisv9.NewClient(&redisv9.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
	defer func() { _ = rclient.Close() }()

	_, err := rclient.Set(context.TODO(), "test", "test", 0).Result()
	require.NoError(t, err)
	_, err = rclient.PExpire(context.TODO(), "test", 100*time.Millisecond).Result()
	require.NoError(t, err)
	res, err := rclient.PTTL(context.TODO(), "test").Result()
	require.NoError(t, err)
	t.Log(res)
	require.GreaterOrEqual(t, 100*time.Millisecond, res)

	lock, err := RedisDLockBuilder(context.TODO(),
		func() redis.Scripter {
			return rclient.Conn()
		},
	).Retry(DefaultExponentialBackoffRetry()).
		TTL(200*time.Millisecond).
		Keys("testKey1_1", "testKey2_1").
		Token("test1").
		Build()
	require.NoError(t, err)
	err = lock.Lock()
	require.NoError(t, err)
	ttl, err := lock.TTL()
	require.NoError(t, err)
	t.Log("ttl1", ttl)
	err = lock.Renewal(300 * time.Millisecond)
	require.NoError(t, err)
	ttl, err = lock.TTL()
	require.NoError(t, err)
	t.Log("ttl2", ttl)
	err = lock.Unlock()
	require.NoError(t, err)
}

func TestRedisDLock_MiniRedis_DataRace(t *testing.T) {
	require.NotEmpty(t, luaDLockAcquireScript)
	require.NotEmpty(t, luaDLockReleaseScript)
	require.NotEmpty(t, luaDLockRenewalTTLScript)
	require.NotEmpty(t, luaDLockLoadTTLScript)

	const addr = "127.0.0.1:6481"
	mredis := mredisv2.NewMiniRedis()
	defer func() { mredis.Close() }()
	go func() {
		err := mredis.StartAddr(addr)
		require.NoError(t, err)
	}()

	rclient := redisv9.NewClient(&redisv9.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
	defer func() { _ = rclient.Close() }()

	_, err := rclient.Set(context.TODO(), "test", "test", 0).Result()
	require.NoError(t, err)

	lock1, err := RedisDLockBuilder(context.TODO(),
		func() redis.Scripter {
			return rclient.Conn()
		},
	).Retry(DefaultExponentialBackoffRetry()).
		TTL(200*time.Millisecond).
		Keys("testKey1_2", "testKey2_2").
		Token("test1").
		Build()
	require.NoError(t, err)

	lock2, err := RedisDLockBuilder(context.TODO(),
		func() redis.Scripter {
			return rclient.Conn()
		},
	).Retry(DefaultExponentialBackoffRetry()).
		TTL(200*time.Millisecond).
		Keys("testKey1_2", "testKey2_2").
		Token("test2").
		Build()
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_err := lock1.Lock()
		if _err != nil {
			t.Logf("lock1 acquire dlock filed")
			require.True(t, errors.Is(_err, ErrDLockAcquireFailed))
			wg.Done()
			return
		}
		_err = lock1.Unlock()
		require.NoError(t, _err)
		wg.Done()
	}()

	go func() {
		_err := lock2.Lock()
		if _err != nil {
			t.Logf("lock2 acquire dlock filed")
			require.True(t, errors.Is(_err, ErrDLockAcquireFailed))
			wg.Done()
			return
		}
		_err = lock2.Unlock()
		require.NoError(t, _err)
		wg.Done()
	}()
	wg.Wait()
}

func TestRedisDLock_SingleRedis(t *testing.T) {
	require.NotEmpty(t, luaDLockAcquireScript)
	require.NotEmpty(t, luaDLockReleaseScript)
	require.NotEmpty(t, luaDLockRenewalTTLScript)
	require.NotEmpty(t, luaDLockLoadTTLScript)

	addr := os.Getenv("REDIS_DLOCK_ADDR")
	if addr == "" {
		t.Skip("REDIS_DLOCK_ADDR is empty")
	}
	pwd := os.Getenv("REDIS_DLOCK_PWD")

	rclient := redisv9.NewClient(&redisv9.Options{
		Addr:     addr,
		Password: pwd,
		DB:       0,
	})
	defer func() { _ = rclient.Close() }()

	_, err := rclient.Set(context.TODO(), "test", "test", 0).Result()
	require.NoError(t, err)
	_, err = rclient.PExpire(context.TODO(), "test", 300*time.Millisecond).Result()
	require.NoError(t, err)
	res, err := rclient.PTTL(context.TODO(), "test").Result()
	require.NoError(t, err)
	t.Log(res)
	require.GreaterOrEqual(t, 300*time.Millisecond, res)

	lock, err := RedisDLockBuilder(context.TODO(),
		func() redis.Scripter {
			return rclient.Conn()
		},
	).Retry(DefaultExponentialBackoffRetry()).
		TTL(200*time.Millisecond).
		Keys("testKey1_3", "testKey2_4").
		Token("test1").
		Build()
	require.NoError(t, err)
	err = lock.Lock()
	require.NoError(t, err)
	ttl, err := lock.TTL()
	require.NoError(t, err)
	t.Log("ttl1", ttl)
	err = lock.Renewal(300 * time.Millisecond)
	require.NoError(t, err)
	ttl, err = lock.TTL()
	require.NoError(t, err)
	t.Log("ttl2", ttl)
	err = lock.Unlock()
	require.NoError(t, err)
}

func TestRedisDLock_SingleRedis_DataRace(t *testing.T) {
	require.NotEmpty(t, luaDLockAcquireScript)
	require.NotEmpty(t, luaDLockReleaseScript)
	require.NotEmpty(t, luaDLockRenewalTTLScript)
	require.NotEmpty(t, luaDLockLoadTTLScript)

	addr := os.Getenv("REDIS_DLOCK_ADDR")
	if addr == "" {
		t.Skip("REDIS_DLOCK_ADDR is empty")
	}
	pwd := os.Getenv("REDIS_DLOCK_PWD")

	rclient := redisv9.NewClient(&redisv9.Options{
		Addr:     addr,
		Password: pwd,
		DB:       0,
	})
	defer func() { _ = rclient.Close() }()

	_, err := rclient.Set(context.TODO(), "test", "test", 0).Result()
	require.NoError(t, err)

	lock1, err := RedisDLockBuilder(context.TODO(),
		func() redis.Scripter {
			return rclient.Conn()
		},
	).Retry(DefaultExponentialBackoffRetry()).
		TTL(200*time.Millisecond).
		Keys("testKey1_4", "testKey2_4").
		Token("test1").
		Build()
	require.NoError(t, err)

	lock2, err := RedisDLockBuilder(context.TODO(),
		func() redis.Scripter {
			return rclient.Conn()
		},
	).Retry(DefaultExponentialBackoffRetry()).
		TTL(200*time.Millisecond).
		Keys("testKey1_4", "testKey2_4").
		Token("test2").
		Build()
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_err := lock1.Lock()
		if _err != nil {
			t.Logf("lock1 acquire dlock filed")
			require.True(t, errors.Is(_err, ErrDLockAcquireFailed))
			wg.Done()
			return
		}
		_err = lock1.Unlock()
		require.NoError(t, _err)
		wg.Done()
	}()

	go func() {
		_err := lock2.Lock()
		if _err != nil {
			t.Logf("lock2 acquire dlock filed")
			require.True(t, errors.Is(_err, ErrDLockAcquireFailed))
			wg.Done()
			return
		}
		_err = lock2.Unlock()
		require.NoError(t, _err)
		wg.Done()
	}()
	wg.Wait()
}
