//go:build linux
// +build linux

package runtime

import (
	"testing"
)

func TestIsRunningAtDocker(t *testing.T) {
	t.Logf("running in docker: %v\n", IsRunningAtDocker())
}

func TestIsRunningAtKubernetes(t *testing.T) {
	t.Logf("running in kubernetes: %v\n", IsRunningAtKubernetes())
}
