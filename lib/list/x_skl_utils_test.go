package list

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOptimisticLock(t *testing.T) {
	lock := new(spinMutex)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		startTime := time.Now()
		lock.lock(1)
		defer wg.Done()
		defer lock.unlock(1)
		defer func() {
			t.Logf("1 elapsed: %d\n", time.Since(startTime).Milliseconds())
		}()
		ms := cryptoRandUint32() % 11
		t.Logf("id: 1, obtain spin lock, sleep ms : %d\n", ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}()
	go func() {
		startTime := time.Now()
		lock.lock(1)
		defer wg.Done()
		defer lock.unlock(1)
		defer func() {
			t.Logf("2 elapsed: %d\n", time.Since(startTime).Milliseconds())
		}()
		ms := cryptoRandUint32() % 11
		t.Logf("id: 2, obtain spin lock, sleep ms : %d\n", ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}()
	wg.Wait()
}

func TestFlagBitSetBitsAs(t *testing.T) {
	type testcase struct {
		name        string
		targetBits  uint32
		value       uint32
		fb          flagBits
		expectValue uint32
	}
	testcases := []testcase{
		{
			name:        "0x0000 set 0x0001 to 0x00FF as 0x0001",
			targetBits:  0x00FF,
			value:       0x0001,
			fb:          flagBits{},
			expectValue: 0x0001,
		}, {
			name:        "0x0000 set 0x00FF to 0x00FF as 0x00FF",
			targetBits:  0x00FF,
			value:       0x00FF,
			fb:          flagBits{},
			expectValue: 0x00FF,
		}, {
			name:        "0x01FF set 0x00FF to 0x00FF as 0x01FF",
			targetBits:  0x00FF,
			value:       0x00FF,
			fb:          flagBits{bits: 0x01FF},
			expectValue: 0x01FF,
		}, {
			name:        "0x01FF set 0x00FF to 0x0000 as 0x0100",
			targetBits:  0x00FF,
			value:       0x0000,
			fb:          flagBits{bits: 0x01FF},
			expectValue: 0x0100,
		}, {
			name:        "0x03FF set 0x0300 to 0x0001 as 0x01FF",
			targetBits:  0x0300,
			value:       0x0001,
			fb:          flagBits{bits: 0x03FF},
			expectValue: 0x01FF,
		}, {
			name:        "0x00FF set 0x0300 to 0x0003 as 0x03FF",
			targetBits:  0x0300,
			value:       0x0003,
			fb:          flagBits{bits: 0x00FF},
			expectValue: 0x03FF,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			tc.fb.setBitsAs(tc.targetBits, tc.value)
			require.Equal(tt, tc.expectValue, tc.fb.bits)
		})
	}
}

func TestFlagBitBitsAreEqualTo(t *testing.T) {
	type testcase struct {
		name        string
		targetBits  uint32
		value       uint32
		fb          flagBits
		expectValue bool
	}
	testcases := []testcase{
		{
			name:        "0x0001 get 0x00FF are equal to 0x0001",
			targetBits:  0x00FF,
			value:       0x0001,
			fb:          flagBits{bits: 0x0001},
			expectValue: true,
		}, {
			name:        "0x0301 get 0x0700 are equal to 0x0003",
			targetBits:  0x0700,
			value:       0x0003,
			fb:          flagBits{bits: 0x0301},
			expectValue: true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			res := tc.fb.areEqual(tc.targetBits, tc.value)
			require.True(tt, res)
		})
	}
}
