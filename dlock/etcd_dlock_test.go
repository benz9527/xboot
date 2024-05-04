//go:build linux
// +build linux

package dlock

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	tintegration "go.etcd.io/etcd/tests/v3/integration"
)

func TestEtcdDLock_DataRace(t *testing.T) {
	tintegration.BeforeTest(t)
	clusterv3 := tintegration.NewClusterV3(t, &tintegration.ClusterConfig{
		Size: 1,
	})
	clients := make([]*clientv3.Client, 0, 2)
	cliConstructor := tintegration.MakeSingleNodeClients(t, clusterv3, &clients)
	cli := cliConstructor()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		lock1, err := EtcdDLockBuilder(context.TODO(), cli).
			TTL(2*time.Second).
			Keys("testKey1", "testKey2").
			Retry(DefaultExponentialBackoffRetry()).
			Build()
		require.NoError(t, err)
		err = lock1.Lock()
		if err != nil {
			t.Logf("lock1.Lock() err: %v", err)
			wg.Done()
			return
		}
		require.NoError(t, err)
		require.Error(t, lock1.Renewal(3*time.Second))
		ttl, err := lock1.TTL()
		require.NoError(t, err)
		t.Log("lock1 ttl", ttl)
		err = lock1.Unlock()
		require.NoError(t, err)
		wg.Done()
	}()
	go func() {
		lock2, err := EtcdDLock(context.TODO(), cli,
			WithEtcdDLockTTL(2*time.Second),
			WithEtcdDLockKeys("testKey1", "testKey2"),
			WithEtcdDLockRetry(DefaultExponentialBackoffRetry()),
		)
		require.NoError(t, err)
		err = lock2.Lock()
		if err != nil {
			t.Logf("lock2.Lock() err: %v", err)
			wg.Done()
			return
		}
		require.NoError(t, err)
		ttl, err := lock2.TTL()
		require.NoError(t, err)
		t.Log("lock2 ttl", ttl)
		err = lock2.Unlock()
		require.NoError(t, err)
		wg.Done()
	}()

	wg.Wait()
	_ = cli.Close()
	clusterv3.Terminate(t)
}
