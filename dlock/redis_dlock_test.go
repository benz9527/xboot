package dlock

import (
	"context"
	"testing"
	"time"

	mredisv2 "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisDLock(t *testing.T) {
	const addr = "127.0.0.1:6379"
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

	_, err := rclient.PExpire(context.TODO(), "test", 100*time.Millisecond).Result()
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
		Keys("testKey1", "testKey2").
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
