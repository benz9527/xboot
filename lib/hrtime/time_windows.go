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

	"golang.org/x/sys/windows"
)

// Way 1: Update the Windows time resolution to 1ms.

var (
	windowsHighResolutionEnabled = atomic.Bool{}
)

func SetTimeResolutionTo1ms() {
	if windowsHighResolutionEnabled.Load() {
		return
	}
	if err := windows.TimeBeginPeriod(1); err != nil {
		panic(err)
	}
	windowsHighResolutionEnabled.Store(true)
}

func ResetTimeResolutionFrom1ms() error {
	if !windowsHighResolutionEnabled.Load() {
		return nil
	}
	if err := windows.TimeEndPeriod(1); err != nil {
		return err
	}
	windowsHighResolutionEnabled.Store(false)
	return nil
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

func ClockInit() {
	SetTimeResolutionTo1ms()
	appStartTime = time.Now().In(loadTZLocation(TimeZoneOffset(atomic.LoadInt32(&defaultTimezoneOffset))))

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
	if fallbackLowResolution.Load() {
		return time.Now().In(loadTZLocation(offset))
	}
	return now().In(loadTZLocation(offset))
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

func Since(beginTime time.Time) time.Duration {
	if fallbackLowResolution.Load() {
		return time.Since(beginTime)
	}
	n := NowInDefaultTZ()
	return time.Duration(n.Nanosecond() - beginTime.In(loadTZLocation(TimeZoneOffset(DefaultTimezoneOffset()))).Nanosecond())
}

var (
	SdkClock     = &sdkClockTime{}
	WindowsClock = &windowsClockTime{}
)

type sdkClockTime struct{}

func (s *sdkClockTime) NowIn(offset TimeZoneOffset) time.Time {
	SetFallbackLowResolution(true)
	return NowIn(offset)
}

func (s *sdkClockTime) NowInDefaultTZ() time.Time {
	SetFallbackLowResolution(true)
	return s.NowIn(TimeZoneOffset(atomic.LoadInt32(&defaultTimezoneOffset)))
}

func (s *sdkClockTime) NowInUTC() time.Time {
	SetFallbackLowResolution(true)
	return s.NowIn(TzUtc0Offset)
}

func (s *sdkClockTime) MonotonicElapsed() time.Duration {
	SetFallbackLowResolution(true)
	return MonotonicElapsed()
}

func (s *sdkClockTime) Since(beginTime time.Time) time.Duration {
	SetFallbackLowResolution(true)
	return Since(beginTime)
}

type windowsClockTime struct{}

func (w *windowsClockTime) NowIn(offset TimeZoneOffset) time.Time {
	SetFallbackLowResolution(false)
	return NowIn(offset)
}

func (w *windowsClockTime) NowInDefaultTZ() time.Time {
	SetFallbackLowResolution(false)
	return w.NowIn(TimeZoneOffset(atomic.LoadInt32(&defaultTimezoneOffset)))
}

func (w *windowsClockTime) NowInUTC() time.Time {
	SetFallbackLowResolution(false)
	return w.NowIn(TzUtc0Offset)
}

func (w *windowsClockTime) MonotonicElapsed() time.Duration {
	SetFallbackLowResolution(false)
	return MonotonicElapsed()
}

func (w *windowsClockTime) Since(beginTime time.Time) time.Duration {
	SetFallbackLowResolution(false)
	return Since(beginTime)
}
