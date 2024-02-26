//go:build linux
// +build linux

package runtime

import "os"

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
