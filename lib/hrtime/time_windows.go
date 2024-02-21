//go:build windows
// +build windows

// High-resolution time for Windows.

package hrtime

// References:
// https://github.com/golang/go/issues/31160
// https://github.com/golang/go/issues/31528
// https://go-review.googlesource.com/c/go/+/227499/1/src/testing/time_windows.go
// https://go-review.googlesource.com/c/sys/+/515915
// https://github.com/golang/go/blob/master/src/runtime/os_windows.go#L447
// https://github.com/golang/go/blob/9b6e9f0c8c66355c0f0575d808b32f52c8c6d21c/src/runtime/os_windows.go#L381
// https://learn.microsoft.com/en-us/windows/win32/sysinfo/acquiring-high-resolution-time-stamps

import (
	"errors"
	"sync/atomic"
	"time"
	"unsafe"

	_ "go.uber.org/automaxprocs"
	"golang.org/x/sys/windows"
)

// Way 1: Update the Windows time resolution to 1ms.

func SetTimeResolutionTo1ms() {
	if err := windows.TimeBeginPeriod(1); err != nil {
		panic(err)
	}
}

func ResetTimeResolutionFrom1ms() error {
	return windows.TimeEndPeriod(1)
}

// Way 2: Use the Windows high-resolution performance counter API.

var (
	// Load windows dynamic link library.
	kernel32 = windows.NewLazyDLL("kernel32.dll")
	// Find windows dynamic link library functions.
	procQPF = kernel32.NewProc("QueryPerformanceFrequency")
	procQPC = kernel32.NewProc("QueryPerformanceCounter")
)

// https://learn.microsoft.com/en-us/windows/win32/api/profileapi/nf-profileapi-queryperformancefrequency
// BOOL QueryPerformanceFrequency(
//
//	[out] LARGE_INTEGER *lpFrequency
//
// );
func getFrequency() (int64, bool) {
	var freq int64
	r1, _, err := procQPF.Call(uintptr(unsafe.Pointer(&freq)))
	if err != nil && !errors.Is(err, windows.SEVERITY_SUCCESS) {
		panic(err)
	}
	return freq, r1 == 1
}

// https://learn.microsoft.com/en-us/windows/win32/api/profileapi/nf-profileapi-queryperformancecounter
// BOOL QueryPerformanceCounter(
//
//	[out] LARGE_INTEGER *lpPerformanceCount
//
// );
// In multi-cores CPU, the counter may not be monotonic.
// 1. Hardware differences: The counter is not guaranteed to be consistent across different hardware.
// 2. CPU cache-line consistency: The counter is not guaranteed to be consistent across different CPU cores.
// 3. Interrupt delays: The counter is not guaranteed to be consistent across different CPU cores.
// 4. (Power management) Dynamic Voltage and Frequency Scaling (DVFS): The counter is not guaranteed to be consistent across different CPU cores.
// 5. OS scheduling (CPU Core-to-Core): The counter is not guaranteed to be consistent across different CPU cores.
// 6. Some BIOS issues in multi-cores CPU's: The counter is not guaranteed to be consistent across different CPU cores.
// The way to fetch precious counter-number is running on a single-core CPU.
func getCounter() (int64, bool) {
	var counter int64
	r1, _, err := procQPC.Call(uintptr(unsafe.Pointer(&counter)))
	if err != nil && !errors.Is(err, windows.SEVERITY_SUCCESS) {
		panic(err)
	}
	return counter, r1 == 1
}

var (
	baseProcFreq          int64
	baseProcCounter       int64
	fallbackLowResolution atomic.Bool
)

func SetFallbackLowResolution(flag bool) {
	fallbackLowResolution.Swap(flag)
}

func init() {
	defaultTimezoneOffset = int32(TzUtc0Offset)
	zone := time.FixedZone("CST", int(defaultTimezoneOffset))
	appStartTime = time.Now().In(zone)

	var ok bool
	if baseProcCounter, ok = getCounter(); !ok {
		fallbackLowResolution.Store(true)
	}
	if baseProcFreq, ok = getFrequency(); !fallbackLowResolution.Load() && !ok || baseProcFreq <= 0 {
		fallbackLowResolution.Store(true)
	}
}

func now() time.Time {
	nano := appStartTime.UnixNano() + MonotonicElapsed().Nanoseconds()
	return time.UnixMilli(time.Duration(nano).Milliseconds())
}

func NowIn(offset TimeZoneOffset) time.Time {
	zone := time.FixedZone("CST", int(offset))
	if fallbackLowResolution.Load() {
		return time.Now().In(zone)
	}
	return now().In(zone)
}

func NowInDefaultTZ() time.Time {
	return NowIn(TimeZoneOffset(atomic.LoadInt32(&defaultTimezoneOffset)))
}

func NowInUTC() time.Time {
	return NowIn(TzUtc0Offset)
}

// MonotonicElapsed returns the time elapsed since the program started.
// Note: Not a very precise implementation.
func MonotonicElapsed() time.Duration {
	if fallbackLowResolution.Load() {
		return time.Since(appStartTime)
	}
	currentCounter, _ := getCounter()
	return time.Duration(currentCounter-baseProcCounter) * time.Second / (time.Duration(baseProcFreq) * time.Nanosecond)
}
