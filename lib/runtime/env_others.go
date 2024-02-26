//go:build !linux
// +build !linux

package runtime

func IsRunningAtDocker() bool { return false }

func IsRunningAtKubernetes() bool { return false }

func LoadContainerID() string { return "" }
