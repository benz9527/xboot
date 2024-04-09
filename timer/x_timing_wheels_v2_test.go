package timer

import (
	"context"
	"errors"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"

	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/observability"
)

func testSimpleAfterFuncSdkDefaultTimeV2(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 2100*time.Millisecond, errors.New("timeout"))
	defer cancel()
	idGen, err := id.StandardSnowFlakeID(0, 0, func() time.Time { return time.Now().UTC() })
	require.NoError(t, err)
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelTimeSource(SdkDefaultTime),
		WithTimingWheelsIDGen(idGen),
		withTimingWheelsDebugStatsInit(2),
		WithTimingWheelsStats(),
	)
	defer func() {
		mp, ok := otel.GetMeterProvider().(*metric.MeterProvider)
		if ok && mp != nil {
			_ = mp.Shutdown(ctx)
		}
	}()

	delays := []time.Duration{
		time.Millisecond,
		2 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
		18 * time.Millisecond,
		20 * time.Millisecond,
		21 * time.Millisecond,
		22 * time.Millisecond,
		23 * time.Millisecond,
		50 * time.Millisecond,
		51 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}

	expectedExecCount := int64(len(delays))
	actualExecCounter := atomic.Int64{}
	startTs := time.Now().UTC().UnixMilli()
	for i := 0; i < len(delays); i++ {
		_, err := tw.AfterFunc(delays[i], func(ctx context.Context, md JobMetadata) {
			actualExecCounter.Add(1)
			t.Logf("exec diff: %v; delay: %v\n", time.Now().UTC().UnixMilli()-startTs, delays[i])
		})
		assert.NoError(t, err)
	}
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, expectedExecCount, actualExecCounter.Load())
}

func TestXTimingWheelsV2_SimpleAfterFunc(t *testing.T) {
	loops := 1
	for i := 0; i < loops; i++ {
		t.Logf("loop %d\n", i)
		testSimpleAfterFuncSdkDefaultTimeV2(t)
	}
}

func TestXTimingWheelsV2_AlignmentAndSize(t *testing.T) {
	tw := &xTimingWheelsV2{}
	t.Logf("tw alignment: %d\n", unsafe.Alignof(tw))
	t.Logf("tw ctx alignment: %d\n", unsafe.Alignof(tw.ctx))
	t.Logf("tw tw alignment: %d\n", unsafe.Alignof(tw.tw))
	t.Logf("tw stopC alignment: %d\n", unsafe.Alignof(tw.stopC))
	t.Logf("tw twEventC alignment: %d\n", unsafe.Alignof(tw.twEventDisruptor))
	t.Logf("tw twEventPool alignment: %d\n", unsafe.Alignof(tw.twEventPool))
	t.Logf("tw expiredSlotC alignment: %d\n", unsafe.Alignof(tw.expiredSlotC))
	t.Logf("tw isRunning alignment: %d\n", unsafe.Alignof(tw.isRunning))
	t.Logf("tw dq alignment: %d\n", unsafe.Alignof(tw.dq))
	t.Logf("tw tasksMap alignment: %d\n", unsafe.Alignof(tw.tasksMap))

	t.Logf("tw size: %d\n", unsafe.Sizeof(*tw))
	t.Logf("tw ctx size: %d\n", unsafe.Sizeof(tw.ctx))
	t.Logf("tw tw size: %d\n", unsafe.Sizeof(tw.tw))
	t.Logf("tw stopC size: %d\n", unsafe.Sizeof(tw.stopC))
	t.Logf("tw twEventC size: %d\n", unsafe.Sizeof(tw.twEventDisruptor))
	t.Logf("tw twEventPool size: %d\n", unsafe.Sizeof(tw.twEventPool))
	t.Logf("tw expiredSlotC size: %d\n", unsafe.Sizeof(tw.expiredSlotC))
	t.Logf("tw isRunning size: %d\n", unsafe.Sizeof(tw.isRunning))
	t.Logf("tw dq size: %d\n", unsafe.Sizeof(tw.dq))
	t.Logf("tw tasksMap size: %d\n", unsafe.Sizeof(tw.tasksMap))
}

func TestXTimingWheelsV2_ScheduleFunc_ConcurrentFinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 2100*time.Millisecond, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		withTimingWheelsDebugStatsInit(2),
		WithTimingWheelsStats(),
	)

	delays := []time.Duration{
		time.Millisecond,
		2 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
		18 * time.Millisecond,
		20 * time.Millisecond,
		21 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}
	schedFn := func() Scheduler {
		return NewFiniteScheduler(delays...)
	}
	assert.NotNil(t, schedFn)
	execCounter := &atomic.Int32{}

	go func() {
		task, err := tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {
			execCounter.Add(1)
		})
		assert.NoError(t, err)
		t.Logf("task1: %s\n", task.GetJobID())
	}()
	go func() {
		task, err := tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {
			execCounter.Add(1)
		})
		assert.NoError(t, err)
		t.Logf("task2: %s\n", task.GetJobID())
	}()

	t.Logf("tw tickMs: %d\n", tw.GetTickMs())
	t.Logf("tw startMs: %d\n", tw.GetStartMs())
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
	expectedExecCount := len(delays) * 2
	actualExecCount := execCounter.Load()
	assert.Equal(t, expectedExecCount, int(actualExecCount))
}

func TestXTimingWheelsV2_ScheduleFunc_sdkClock_1MsInfinite(t *testing.T) {
	observability.InitAppStats(context.Background(), "sdk1msInfinite")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		withTimingWheelsDebugStatsInit(5),
		WithTimingWheelsStats(),
	)

	delays := []time.Duration{
		time.Millisecond,
	}
	schedFn := func() Scheduler {
		return NewInfiniteScheduler(delays...)
	}
	assert.NotNil(t, schedFn())
	loop := 20
	tasks := make([]Task, loop)
	for i := range loop {
		var err error
		tasks[i], err = tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {})
		assert.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
}

func TestXTimingWheelsV2_ScheduleFunc_sdkClock_2MsInfinite(t *testing.T) {
	observability.InitAppStats(context.Background(), "sdk2msInfinite")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelsTickMs(2*time.Millisecond),
		WithTimingWheelsSlotSize(20),
		withTimingWheelsDebugStatsInit(5),
		WithTimingWheelsStats(),
	)

	delays := []time.Duration{
		2 * time.Millisecond,
	}
	schedFn := func() Scheduler {
		return NewInfiniteScheduler(delays...)
	}
	assert.NotNil(t, schedFn())
	loop := 20
	tasks := make([]Task, loop)
	for i := range loop {
		var err error
		tasks[i], err = tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {})
		assert.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
}

func TestXTimingWheelsV2_ScheduleFunc_5MsInfinite(t *testing.T) {
	observability.InitAppStats(context.Background(), "sdk5msInfinite")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelsTickMs(5*time.Millisecond),
		WithTimingWheelsSlotSize(20),
		withTimingWheelsDebugStatsInit(5),
		WithTimingWheelsStats(),
	)

	delays := []time.Duration{
		8 * time.Millisecond,
		18 * time.Millisecond,
	}
	schedFn := func() Scheduler {
		return NewInfiniteScheduler(delays...)
	}
	loop := 20
	tasks := make([]Task, loop)
	for i := range loop {
		var err error
		tasks[i], err = tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {})
		assert.NoError(t, err)
		time.Sleep(2 * time.Millisecond)
	}

	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
}

func TestXTimingWheelsV2_AfterFunc_Slots(t *testing.T) {
	_, debugLogDisabled := os.LookupEnv("DISABLE_TEST_DEBUG_LOG")
	ctx, cancel := context.WithTimeoutCause(context.Background(), 500*time.Millisecond, errors.New("timeout"))
	defer cancel()
	ctx = context.WithValue(ctx, disableTimingWheelsScheduleCancelTask, true)
	ctx = context.WithValue(ctx, disableTimingWheelsSchedulePoll, true)

	tw := NewXTimingWheelsV2(
		ctx,
	)

	delays := []time.Duration{
		3 * time.Millisecond,
		4 * time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		15 * time.Millisecond,
		18 * time.Millisecond,
		20 * time.Millisecond,
		21 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}

	for i := 0; i < len(delays); i++ {
		_, err := tw.AfterFunc(delays[i], func(ctx context.Context, md JobMetadata) {})
		assert.NoError(t, err)
	}
	t.Logf("tw tickMs: %d\n", tw.GetTickMs())
	t.Logf("tw startMs: %d\n", tw.GetStartMs())

	<-time.After(100 * time.Millisecond)
	jobIDs := make([]string, 0, len(delays))
	keys := tw.(*xTimingWheelsV2).tasksMap.ListKeys()
	for _, k := range keys {
		jobIDs = append(jobIDs, string(k))
	}
	sort.Strings(jobIDs)
	for _, jobID := range jobIDs {
		v, _ := tw.(*xTimingWheelsV2).tasksMap.Get(JobID(jobID))
		if !debugLogDisabled {
			t.Logf("job ID: %s, slot level: %d, ID %d, %d\n", jobID, v.GetSlot().GetLevel(), v.GetSlot().GetSlotID(), v.GetExpiredMs())
		}
	}
	<-ctx.Done()
}

func BenchmarkNewTimingWheelsV2_AfterFunc(b *testing.B) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, disableTimingWheelsScheduleCancelTask, true)
	ctx = context.WithValue(ctx, disableTimingWheelsSchedulePoll, true)
	tw := NewXTimingWheelsV2(
		ctx,
		WithTimingWheelsTickMs(1*time.Millisecond),
		WithTimingWheelsSlotSize(20),
	)
	defer tw.Shutdown()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tw.AfterFunc(time.Duration(i+1)*time.Millisecond, func(ctx context.Context, md JobMetadata) {
		})
		assert.NoError(b, err)
	}
	b.ReportAllocs()
}
