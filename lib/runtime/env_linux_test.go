//go:build linux
// +build linux

package runtime

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsRunningAtDocker(t *testing.T) {
	t.Logf("running in docker: %v\n", IsRunningAtDocker())
}

func TestIsRunningAtKubernetes(t *testing.T) {
	t.Logf("running in kubernetes: %v\n", IsRunningAtKubernetes())
}

func TestContainerIDParse(t *testing.T) {
	example := `0::/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pode6ac4a8d_1076_453e_9ddb_3976520e3178.slice/cri-containerd-19cd7a809d879d9c855bb93e4d399efe795a769ac856faaa5256cdd8387fe4b1.scope`
	path := procSelfCgroupLineRegex.FindStringSubmatch(example)
	if len(path) != 2 {
	}
	assert.Equal(t, 2, len(path))
	parts := containerIDRegex.FindStringSubmatch(path[1])
	assert.Equal(t, 2, len(parts))
	expected := `19cd7a809d879d9c855bb93e4d399efe795a769ac856faaa5256cdd8387fe4b1`
	assert.Equal(t, expected, parts[1])
}
