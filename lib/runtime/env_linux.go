//go:build linux
// +build linux

package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
)

// 1. Loading '/proc/1/status' to check name whether is 'systemd'. If not,
//   the application running in a docker or kubernetes env.
//   It may occur access permission denied.
// 2. Loading '/dev/block' directory, the container env default without block
//   devices.

const (
	dockerEnvPath                = "/.dockerenv"                                             // unstable
	dockerBlockPath              = "/dev/block"                                              // stable
	kubernetesServiceAccountPath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace" // stable
)

func IsRunningAtDocker() bool {
	// Must be contained in kubernetes env
	stat, err := os.Stat(dockerEnvPath)
	if err != nil && os.IsNotExist(err) {
		if _, err = os.Stat(dockerBlockPath); err != nil && os.IsNotExist(err) {
			return true
		}
		return false
	}
	if !stat.IsDir() {
		return true
	}
	return false
}

func IsRunningAtKubernetes() bool {
	// Must be contained in kubernetes env
	stat, err := os.Stat(kubernetesServiceAccountPath)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	if !stat.IsDir() && stat.Size() > 0 {
		return true
	}
	return false
}

const (
	uuidSource      = "[0-9a-f]{8}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{12}|[0-9a-f]{8}(?:-[0-9a-f]{4}){4}$"
	containerSource = "[0-9a-f]{64}"
	taskSource      = "[0-9a-f]{32}-\\d+"
)

var (
	// /proc/self/cgroup line example:
	// 0::/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pode6ac4a8d_1076_453e_9ddb_3976520e3178.slice/cri-containerd-19cd7a809d879d9c855bb93e4d399efe795a769ac856faaa5256cdd8387fe4b1.scope
	procSelfCgroupLineRegex = regexp.MustCompile(`^\d+:[^:]*:(.+)$`)
	containerIDRegex        = regexp.MustCompile(fmt.Sprintf(`(%s|%s|%s)(?:.scope)?$`, uuidSource, containerSource, taskSource))
)

func parseContainerID(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		path := procSelfCgroupLineRegex.FindStringSubmatch(scanner.Text())
		if len(path) != 2 {
			continue
		}
		if parts := containerIDRegex.FindStringSubmatch(path[1]); len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

func LoadContainerID() string {
	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	defer f.Close()
	return parseContainerID(f)
}
