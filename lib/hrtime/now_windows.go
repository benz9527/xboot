//go:build windows
// +build windows

package hrtime

// References:
// https://github.com/azul3d-legacy/clock
// https://github1s.com/loov/hrtime

import (
	"errors"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazyDLL("kernel32.dll")
	procQPF  = kernel32.NewProc("QueryPerformanceFrequency")
	procQPC  = kernel32.NewProc("QueryPerformanceCounter")
)

// https://learn.microsoft.com/en-us/windows/win32/api/profileapi/nf-profileapi-queryperformancefrequency
// BOOL QueryPerformanceFrequency(
//
//	[out] LARGE_INTEGER *lpFrequency
//
// );
func getFrequency() int64 {
	var freq int64
	r1, _, err := procQPF.Call(uintptr(unsafe.Pointer(&freq)))
	if err != nil && !errors.Is(err, windows.SEVERITY_SUCCESS) || r1 != 1 {
		panic(err)
	}
	return freq
}

// https://learn.microsoft.com/en-us/windows/win32/api/profileapi/nf-profileapi-queryperformancecounter
// BOOL QueryPerformanceCounter(
//
//	[out] LARGE_INTEGER *lpPerformanceCount
//
// );
func getCounter() int64 {
	var counter int64
	r1, _, err := procQPC.Call(uintptr(unsafe.Pointer(&counter)))
	if err != nil && !errors.Is(err, windows.SEVERITY_SUCCESS) || r1 != 1 {
		panic(err)
	}
	return counter
}

var (
	baseProcFreq    = getFrequency()
	baseProcCounter = getCounter()
)

func Now() time.Duration {
	return time.Duration(getCounter()-baseProcCounter) * time.Second / (time.Duration(baseProcFreq) * time.Nanosecond)
}

func TimeResolution() float64 {
	return float64(time.Second) / (float64(baseProcFreq) * float64(time.Nanosecond))
}
