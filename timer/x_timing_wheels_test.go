package timer

import (
	"context"
	"errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"log/slog"
	"os"
	"sort"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

func withTimingWheelStatsInit(interval int64) TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		exp, err := stdoutmetric.New(
			//stdoutmetric.WithPrettyPrint(),
			stdoutmetric.WithWriter(os.Stdout),
		)
		if err != nil {
			panic(err)
		}
		mp := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(exp, metric.WithInterval(time.Duration(interval)*time.Second))))
		otel.SetMeterProvider(mp)
	}
}

func TestTimingWheel_AlignmentAndSize(t *testing.T) {
	tw := &timingWheel{}
	t.Logf("tw alignment: %d\n", unsafe.Alignof(tw))
	t.Logf("tw ctx alignment: %d\n", unsafe.Alignof(tw.ctx))
	t.Logf("tw slot alignment: %d\n", unsafe.Alignof(tw.slots))
	t.Logf("tw tickMs alignment: %d\n", unsafe.Alignof(tw.tickMs))
	t.Logf("tw startMs alignment: %d\n", unsafe.Alignof(tw.startMs))
	t.Logf("tw slotSize alignment: %d\n", unsafe.Alignof(tw.slotSize))
	t.Logf("tw globalSlotCounterRef alignment: %d\n", unsafe.Alignof(tw.globalSlotCounterRef))
	t.Logf("tw overflowWheelRef alignment: %d\n", unsafe.Alignof(tw.overflowWheelRef))
	t.Logf("tw globalDqRef alignment: %d\n", unsafe.Alignof(tw.globalDqRef))

	t.Logf("tw size: %d\n", unsafe.Sizeof(*tw))
	t.Logf("tw ctx size: %d\n", unsafe.Sizeof(tw.ctx))
	t.Logf("tw slot size: %d\n", unsafe.Sizeof(tw.slots))
	t.Logf("tw tickMs size: %d\n", unsafe.Sizeof(tw.tickMs))
	t.Logf("tw startMs size: %d\n", unsafe.Sizeof(tw.startMs))
	t.Logf("tw slotSize size: %d\n", unsafe.Sizeof(tw.slotSize))
	t.Logf("tw globalSlotCounterRef size: %d\n", unsafe.Sizeof(tw.globalSlotCounterRef))
	t.Logf("tw overflowWheelRef size: %d\n", unsafe.Sizeof(tw.overflowWheelRef))
	t.Logf("tw globalDqRef size: %d\n", unsafe.Sizeof(tw.globalDqRef))
}

func TestXTimingWheels_AlignmentAndSize(t *testing.T) {
	tw := &xTimingWheels{}
	t.Logf("tw alignment: %d\n", unsafe.Alignof(tw))
	t.Logf("tw ctx alignment: %d\n", unsafe.Alignof(tw.ctx))
	t.Logf("tw tw alignment: %d\n", unsafe.Alignof(tw.tw))
	t.Logf("tw stopC alignment: %d\n", unsafe.Alignof(tw.stopC))
	t.Logf("tw twEventC alignment: %d\n", unsafe.Alignof(tw.twEventC))
	t.Logf("tw twEventPool alignment: %d\n", unsafe.Alignof(tw.twEventPool))
	t.Logf("tw expiredSlotC alignment: %d\n", unsafe.Alignof(tw.expiredSlotC))
	t.Logf("tw taskCounter alignment: %d\n", unsafe.Alignof(tw.taskCounter))
	t.Logf("tw slotCounter alignment: %d\n", unsafe.Alignof(tw.slotCounter))
	t.Logf("tw isRunning alignment: %d\n", unsafe.Alignof(tw.isRunning))
	t.Logf("tw dq alignment: %d\n", unsafe.Alignof(tw.dq))
	t.Logf("tw tasksMap alignment: %d\n", unsafe.Alignof(tw.tasksMap))

	t.Logf("tw size: %d\n", unsafe.Sizeof(*tw))
	t.Logf("tw ctx size: %d\n", unsafe.Sizeof(tw.ctx))
	t.Logf("tw tw size: %d\n", unsafe.Sizeof(tw.tw))
	t.Logf("tw stopC size: %d\n", unsafe.Sizeof(tw.stopC))
	t.Logf("tw twEventC size: %d\n", unsafe.Sizeof(tw.twEventC))
	t.Logf("tw twEventPool size: %d\n", unsafe.Sizeof(tw.twEventPool))
	t.Logf("tw expiredSlotC size: %d\n", unsafe.Sizeof(tw.expiredSlotC))
	t.Logf("tw taskCounter size: %d\n", unsafe.Sizeof(tw.taskCounter))
	t.Logf("tw slotCounter size: %d\n", unsafe.Sizeof(tw.slotCounter))
	t.Logf("tw isRunning size: %d\n", unsafe.Sizeof(tw.isRunning))
	t.Logf("tw dq size: %d\n", unsafe.Sizeof(tw.dq))
	t.Logf("tw tasksMap size: %d\n", unsafe.Sizeof(tw.tasksMap))
}

func TestNewTimingWheels(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.TODO(), time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		time.Now().UTC().UnixMilli(),
		WithTimingWheelTickMs(100),
		WithTimingWheelSlotSize(32),
	)
	t.Logf("tw tickMs: %d\n", tw.GetTickMs())
	t.Logf("tw startMs: %d\n", tw.GetStartMs())
	t.Logf("tw slotSize: %d\n", tw.GetSlotSize())
	t.Logf("tw globalTaskCounterRef: %d\n", tw.GetTaskCounter())
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
}

func testAfterFunc(t *testing.T) (percent float64) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 2100*time.Millisecond, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		time.Now().UTC().UnixMilli(),
		withTimingWheelStatsInit(2),
		WithTimingWheelStats(),
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
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}

	expectedExecCount := int64(len(delays))
	actualExecCounter := atomic.Int64{}
	bigDiffCounter := atomic.Int64{}
	for i := 0; i < len(delays); i++ {
		_, err := tw.AfterFunc(delays[i], func(ctx context.Context, md JobMetadata) {
			actualExecCounter.Add(1)
			diff := time.Now().UTC().UnixMilli() - md.GetExpiredMs()
			if diff > 1 {
				bigDiffCounter.Add(1)
			}
		})
		assert.NoError(t, err)
	}
	<-ctx.Done()
	time.Sleep(50 * time.Millisecond)
	percent = float64(actualExecCounter.Load()-bigDiffCounter.Load()) / float64(expectedExecCount)
	t.Logf("[-1,1] percent: %f\n", percent)
	assert.Equal(t, expectedExecCount, actualExecCounter.Load())
	return percent
}

func TestXTimingWheels_AfterFunc(t *testing.T) {
	loops := 100
	percents := 0.0
	for i := 0; i < loops; i++ {
		t.Logf("loop %d\n", i)
		percents += testAfterFunc(t)
	}
	t.Logf("average percent: %f\n", percents/float64(loops))
}

func TestXTimingWheels_ScheduleFunc_ConcurrentFinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 2100*time.Millisecond, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		time.Now().UTC().UnixMilli(),
		withTimingWheelStatsInit(2),
		WithTimingWheelStats(),
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
	t.Logf("tw slotSize: %d\n", tw.GetSlotSize())
	t.Logf("tw tasks: %d\n", tw.GetTaskCounter())
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
	t.Logf("final tw tasks: %d\n", tw.GetTaskCounter())
	expectedExecCount := len(delays) * 2
	actualExecCount := execCounter.Load()
	assert.Equal(t, expectedExecCount, int(actualExecCount))
	assert.Equal(t, int64(0), tw.GetTaskCounter())
}

func TestXTimingWheels_ScheduleFunc_1MsInfinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 5*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		time.Now().UTC().UnixMilli(),
		withTimingWheelStatsInit(5),
		WithTimingWheelStats(),
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

func TestXTimingWheels_ScheduleFunc_32MsInfinite(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 3*time.Second, errors.New("timeout"))
	defer cancel()
	tw := NewTimingWheels(
		ctx,
		time.Now().UTC().UnixMilli(),
	)

	delays := []time.Duration{
		18 * time.Millisecond,
	}
	schedFn := func() Scheduler {
		return NewInfiniteScheduler(delays...)
	}
	assert.NotNil(t, schedFn())
	task, err := tw.ScheduleFunc(schedFn, func(ctx context.Context, md JobMetadata) {
		execAt := time.Now().UTC().UnixMilli()
		slog.Info("infinite sched32 after func", "expired ms", md.GetExpiredMs(), "exec at", execAt, "diff",
			execAt-md.GetExpiredMs())
	})
	assert.NoError(t, err)
	t.Logf("task1: %s\n", task.GetJobID())

	t.Logf("tw tickMs: %d\n", tw.GetTickMs())
	t.Logf("tw startMs: %d\n", tw.GetStartMs())
	t.Logf("tw slotSize: %d\n", tw.GetSlotSize())
	t.Logf("tw tasks: %d\n", tw.GetTaskCounter())
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)
	t.Logf("final tw tasks: %d\n", tw.GetTaskCounter())
}

func TestXTimingWheels_AfterFunc_Slots(t *testing.T) {
	ctx, cancel := context.WithTimeoutCause(context.Background(), 500*time.Millisecond, errors.New("timeout"))
	defer cancel()
	ctx = context.WithValue(ctx, disableTimingWheelsScheduleCancelTask, true)
	ctx = context.WithValue(ctx, disableTimingWheelsScheduleExpiredSlot, true)
	ctx = context.WithValue(ctx, disableTimingWheelsSchedulePoll, true)

	tw := NewTimingWheels(
		ctx,
		time.Now().UTC().UnixMilli(),
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
	t.Logf("tw slotSize: %d\n", tw.GetSlotSize())
	t.Logf("tw tasks: %d\n", tw.GetTaskCounter())

	<-time.After(100 * time.Millisecond)
	taskIDs := make([]string, 0, len(delays))
	for k := range tw.(*xTimingWheels).tasksMap {
		taskIDs = append(taskIDs, string(k))
	}
	sort.Strings(taskIDs)
	for _, taskID := range taskIDs {
		v := tw.(*xTimingWheels).tasksMap[JobID(taskID)]
		t.Logf("slot level: %d, ID %d, %d\n", v.GetSlot().GetLevel(), v.GetSlot().GetSlotID(), v.GetExpiredMs())
	}
	<-ctx.Done()
}

func BenchmarkNewTimingWheels_AfterFunc(b *testing.B) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, disableTimingWheelsScheduleCancelTask, true)
	ctx = context.WithValue(ctx, disableTimingWheelsScheduleExpiredSlot, true)
	ctx = context.WithValue(ctx, disableTimingWheelsSchedulePoll, true)
	tw := NewTimingWheels(
		ctx,
		time.Now().UTC().UnixMilli(),
		WithTimingWheelTickMs(1),
		WithTimingWheelSlotSize(20),
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
