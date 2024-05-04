//go:build linux
// +build linux

package dlock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	tintegration "go.etcd.io/etcd/tests/v3/integration"
)

func TestEtcdDLock(t *testing.T) {
	tintegration.BeforeTest(t)
	clusterv3 := tintegration.NewClusterV3(t, &tintegration.ClusterConfig{
		Size: 1,
	})
	clients := make([]*clientv3.Client, 0, 2)
	cliConstructor := tintegration.MakeSingleNodeClients(t, clusterv3, &clients)
	cli := cliConstructor()
	defer cli.Close()
	lock1, err := EtcdDLockBuilder(context.TODO(), cli).
		TTL(2*time.Second).
		Keys("testKey1", "testKey2").
		LeaseID(1).
		Build()
	require.NoError(t, err)
	err = lock1.Lock()
	require.NoError(t, err)
}
