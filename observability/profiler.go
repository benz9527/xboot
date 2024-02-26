package observability

// References:
// https://github.com/DataDog/dd-trace-go/blob/main/profiler/profiler.go#L118

type ProfileType int8

const (
	CPUProfile = iota
	MemProfile
)

// TODO: Add profilers
