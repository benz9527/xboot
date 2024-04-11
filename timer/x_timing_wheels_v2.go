package timer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
	"github.com/benz9527/xboot/lib/ipc"
	"github.com/benz9527/xboot/lib/kv"
	"github.com/benz9527/xboot/lib/queue"
)

type xTimingWheelsV2 struct {
	tw               TimingWheel
	ctx              context.Context
	dq               queue.DelayQueue[TimingWheelSlot] // Do not use the timer.Ticker
	tasksMap         kv.ThreadSafeStorer[JobID, Task]
	stopC            chan struct{}
	expiredSlotC     infra.ClosableChannel[TimingWheelSlot]
	twEventDisruptor ipc.Disruptor[*timingWheelEvent]
	twEventPool      *timingWheelEventsPool
	gPool            *ants.Pool
	stats            *xTimingWheelsStats
	isRunning        *atomic.Bool
	clock            hrtime.Clock
	idGenerator      id.Gen
	name             string
	schedLock        sync.Mutex // Pay attention to the lock granularity
	isStatsEnabled   bool
}

func (xtw *xTimingWheelsV2) GetTickMs() int64 {
	return xtw.tw.GetTickMs()
}

func (xtw *xTimingWheelsV2) GetStartMs() int64 {
	return xtw.tw.GetStartMs()
}

func (xtw *xTimingWheelsV2) Shutdown() {
	if xtw == nil {
		return
	}
	if old := xtw.isRunning.Swap(false); !old {
		slog.Warn("[x-timing-wheels v2] x-timing-wheels has been shutdown!")
		return
	}
	xtw.isRunning.Store(false)

	close(xtw.stopC)
	_ = xtw.expiredSlotC.Close()
	_ = xtw.twEventDisruptor.Stop()
	xtw.gPool.Release()

	runtime.SetFinalizer(xtw, func(xtw *xTimingWheelsV2) {
		xtw.dq = nil
		_ = xtw.tasksMap.Purge()
	})
}

func (xtw *xTimingWheelsV2) AddTask(task Task) error {
	if len(task.GetJobID()) <= 0 {
		return infra.WrapErrorStack(ErrTimingWheelTaskEmptyJobID)
	}
	if task.GetJob() == nil {
		return infra.WrapErrorStack(ErrTimingWheelEmptyJob)
	}
	if !xtw.isRunning.Load() {
		return infra.WrapErrorStack(ErrTimingWheelStopped)
	}
	event := xtw.twEventPool.Get()
	event.AddTask(task)
	_, _, err := xtw.twEventDisruptor.Publish(event)
	return infra.WrapErrorStack(err)
}

func (xtw *xTimingWheelsV2) AfterFunc(delayMs time.Duration, fn Job) (Task, error) {
	if delayMs.Milliseconds() < xtw.GetTickMs() {
		return nil, fmt.Errorf("[x-timing-wheels v2] job's delay ms %d is less than tick ms %d, %w",
			delayMs.Milliseconds(), xtw.GetTickMs(), ErrTimingWheelTaskTooShortExpiration)
	}
	if fn == nil {
		return nil, infra.WrapErrorStack(ErrTimingWheelEmptyJob)
	}

	var now = xtw.clock.NowInDefaultTZ()
	task := NewOnceTask(
		xtw.ctx,
		JobID(fmt.Sprintf("%v", xtw.idGenerator())),
		now.Add(delayMs).UnixMilli(),
		fn,
	)

	if !xtw.isRunning.Load() {
		return nil, ErrTimingWheelStopped
	}
	if err := xtw.AddTask(task); err != nil {
		return nil, err
	}
	return task, nil
}

func (xtw *xTimingWheelsV2) ScheduleFunc(schedFn func() Scheduler, fn Job) (Task, error) {
	if schedFn == nil {
		return nil, ErrTimingWheelUnknownScheduler
	}
	if fn == nil {
		return nil, ErrTimingWheelEmptyJob
	}

	var now = xtw.clock.NowInDefaultTZ()
	task := NewRepeatTask(
		xtw.ctx,
		JobID(fmt.Sprintf("%v", xtw.idGenerator())),
		now.UnixMilli(), schedFn(),
		fn,
	)

	if !xtw.isRunning.Load() {
		return nil, infra.WrapErrorStack(ErrTimingWheelStopped)
	}
	if err := xtw.AddTask(task); err != nil {
		return nil, infra.WrapErrorStack(err)
	}
	return task, nil
}

func (xtw *xTimingWheelsV2) CancelTask(jobID JobID) error {
	if len(jobID) <= 0 {
		return infra.WrapErrorStack(ErrTimingWheelTaskEmptyJobID)
	}

	if xtw.isRunning.Load() {
		return infra.WrapErrorStack(ErrTimingWheelStopped)
	}
	task, ok := xtw.tasksMap.Get(jobID)
	if !ok {
		return infra.WrapErrorStack(ErrTimingWheelTaskNotFound)
	}

	event := xtw.twEventPool.Get()
	event.CancelTaskJobID(task.GetJobID())
	_, _, err := xtw.twEventDisruptor.Publish(event)
	return infra.WrapErrorStack(err)
}

func (xtw *xTimingWheelsV2) schedule(ctx context.Context) {
	if ctx == nil {
		return
	}
	// FIXME Block error mainly caused by producer and consumer speed mismatch, lock data race.
	//  Is there any limitation mechanism could gradually  control different interval task‘s execution timeout timestamp?
	//  Tasks piling up in the same slot will cause the timing wheel to be blocked or delayed.
	_ = xtw.gPool.Submit(func() {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("[x-timing-wheels v2] event schedule panic recover", "error", err, "stack", debug.Stack())
			}
		}()
		cancelDisabled := ctx.Value(disableTimingWheelsScheduleCancelTask)
		if cancelDisabled == nil {
			cancelDisabled = false
		}
		slotC := xtw.expiredSlotC.Wait()
		for {
			select {
			case <-ctx.Done():
				xtw.Shutdown()
				return
			case <-xtw.stopC:
				return
			default:
				if xtw.expiredSlotC.IsClosed() {
					slog.Warn("[x-timing-wheels v2] slot channel has been closed")
					return
				}
			}

			select {
			case slot := <-slotC:
				xtw.advanceClock(slot.GetExpirationMs())
				// Here related to slot level upgrade and downgrade.
				if slot != nil && slot.GetExpirationMs() > slotHasBeenFlushedMs {
					xtw.stats.UpdateSlotActiveCount(xtw.dq.Len())
					// Reset the slot, ready for the next round.
					xtw.schedLock.Lock()
					slot.setExpirationMs(slotHasBeenFlushedMs)
					xtw.schedLock.Unlock()
					_ = xtw.gPool.Submit(func() {
						xtw.schedLock.Lock()
						defer xtw.schedLock.Unlock()
						slot.Flush(xtw.handleTask)
					})
				}
			}
		}
	})
	_ = xtw.gPool.Submit(func() {
		func(disabled any) {
			if disabled != nil && disabled.(bool) {
				slog.Warn("[x-timing-wheels v2] delay queue poll disabled")
				return
			}
			defer func() {
				if err := recover(); err != nil {
					slog.Error("[x-timing-wheels v2] poll schedule panic recover", "error", err, "stack", debug.Stack())
				}
				slog.Warn("[x-timing-wheels v2] delay queue exit")
			}()
			xtw.dq.PollToChan(func() int64 {
				return xtw.clock.NowInDefaultTZ().UnixMilli()
			}, xtw.expiredSlotC)
		}(ctx.Value(disableTimingWheelsSchedulePoll))
	})
	if err := xtw.twEventDisruptor.Start(); err != nil {
		panic(err)
	}
	xtw.isRunning.Store(true)
}

func (xtw *xTimingWheelsV2) handleEvent(event *timingWheelEvent) error {
	switch op := event.GetOperation(); op {
	case addTask, reAddTask:
		task, ok := event.GetTask()
		if !ok {
			goto recycle
		}
		if err := xtw.addTask(task); errors.Is(err, ErrTimingWheelTaskIsExpired) {
			if err = xtw.gPool.Submit(func() {
				xtw.handleTask(task)
			}); err != nil {
				slog.Warn("[x-timing-wheels v2] submit job to pool failed", "op", op.String(),
					"job", task.GetJobID(),
					"execAt", hrtime.MillisToDefaultTzTime(task.GetExpiredMs()),
					"error", err,
				)
			}
		}
		if op == addTask {
			xtw.stats.RecordJobAliveCount(1)
		}
	case cancelTask:
		jobID, ok := event.GetCancelTaskJobID()
		if !ok {
			goto recycle
		}
		if err := xtw.gPool.Submit(func() {
			_ = xtw.cancelTask(jobID)
		}); err != nil {
			slog.Warn("[x-timing-wheels v2] submit job to pool failed", "op", op.String(),
				"job", jobID,
				"error", err,
			)
		}
	default:

	}
recycle:
	xtw.twEventPool.Put(event)
	return nil
}

// Update all wheels' current time, in order to simulate the time is continuously incremented.
// Here related to slot level upgrade and downgrade.
func (xtw *xTimingWheelsV2) advanceClock(timeoutMs int64) {
	xtw.tw.(*timingWheel).advanceClock(timeoutMs)
}

func (xtw *xTimingWheelsV2) addTask(task Task) error {
	if task == nil || task.Cancelled() || !xtw.isRunning.Load() {
		return ErrTimingWheelStopped
	}
	xtw.schedLock.Lock()
	err := xtw.tw.(*timingWheel).addTask(task, 0)
	xtw.schedLock.Unlock()
	if err == nil || errors.Is(err, ErrTimingWheelTaskIsExpired) {
		xtw.tasksMap.AddOrUpdate(task.GetJobID(), task)
	}
	return err
}

// handleTask all tasks which are called by this method
// will mean that the task must be in a slot ever and related slot
// has been expired.
func (xtw *xTimingWheelsV2) handleTask(t Task) {
	if t == nil || !xtw.isRunning.Load() {
		slog.Warn("[x-timing-wheels v2] handle task failed",
			"task is nil", t == nil,
			"timing wheel is running", xtw.isRunning.Load(),
		)
		return
	}

	// [slotExpMs, slotExpMs+interval)
	var (
		prevSlotMetadata = t.GetPreviousSlotMetadata()
		slot             = t.GetSlot()
		taskLevel        int64
		runNow           bool
	)
	if prevSlotMetadata == nil && slot != immediateExpiredSlot {
		// Unknown task
		return
	} else if prevSlotMetadata == nil && slot == immediateExpiredSlot {
		runNow = true
	} else if prevSlotMetadata != nil {
		taskLevel = prevSlotMetadata.GetLevel()
		runNow = prevSlotMetadata.GetExpirationMs() == sentinelSlotExpiredMs
		runNow = runNow || (taskLevel == 0 && t.GetExpiredMs() <= prevSlotMetadata.GetExpirationMs()+xtw.GetTickMs())
	}
	runNow = runNow || t.GetExpiredMs() <= xtw.clock.NowInDefaultTZ().UnixMilli()

	if runNow && !t.Cancelled() {
		job := t.GetJob()
		md := t.GetJobMetadata()
		_ = xtw.gPool.Submit(func() {
			jobStatsWrapper(xtw.stats, job)(xtw.ctx, md)
		})
	} else if t.Cancelled() {
		if slot != nil {
			slot.RemoveTask(t)
		}
		t.setSlot(nil)
		t.setSlotMetadata(nil)
		return
	}

	// Re-addTask loop job to timing wheel.
	// Upgrade and downgrade (move) the t from one slot to another slot.
	// Lock free.
	switch t.GetJobType() {
	case OnceJob:
		event := xtw.twEventPool.Get()
		if runNow {
			event.CancelTaskJobID(t.GetJobID())
		} else {
			event.ReAddTask(t)
		}
		_, _, _ = xtw.twEventDisruptor.Publish(event)
	case RepeatedJob:
		var sTask Task
		if !runNow {
			sTask = t
		} else {
			if t.GetRestLoopCount() == 0 {
				event := xtw.twEventPool.Get()
				event.CancelTaskJobID(t.GetJobID())
				_, _, _ = xtw.twEventDisruptor.Publish(event)
				return
			}
			_sTask, ok := t.(ScheduledTask)
			if !ok {
				return
			}
			_sTask.UpdateNextScheduledMs()
			sTask = _sTask
			if sTask.GetExpiredMs() < 0 {
				return
			}
		}
		if sTask != nil {
			event := xtw.twEventPool.Get()
			event.ReAddTask(sTask)
			_, _, _ = xtw.twEventDisruptor.Publish(event)
		}
	}
	return
}

func (xtw *xTimingWheelsV2) cancelTask(jobID JobID) error {
	if !xtw.isRunning.Load() {
		return infra.WrapErrorStack(ErrTimingWheelStopped)
	}

	task, ok := xtw.tasksMap.Get(jobID)
	if !ok {
		return infra.WrapErrorStack(ErrTimingWheelTaskNotFound)
	}

	xtw.schedLock.Lock()
	if task.GetSlot() != nil && !task.GetSlot().RemoveTask(task) {
		xtw.schedLock.Unlock()
		return ErrTimingWheelTaskUnableToBeRemoved
	}
	xtw.schedLock.Unlock()

	defer func() {
		xtw.stats.IncreaseJobCancelledCount()
		xtw.stats.RecordJobAliveCount(-1)
	}()

	task.Cancel()

	_, err := xtw.tasksMap.Delete(jobID)
	return infra.WrapErrorStack(err)
}

// NewXTimingWheelsV2 creates a new timing wheel.
// The same as the kafka, Time.SYSTEM.hiResClockMs() is used.
func NewXTimingWheelsV2(ctx context.Context, opts ...TimingWheelsOption) TimingWheels {
	if ctx == nil {
		return nil
	}

	xtwOpt := &xTimingWheelsOption{}
	for _, o := range opts {
		if o != nil {
			o(xtwOpt)
		}
	}
	xtwOpt.Validate()

	xtw := &xTimingWheelsV2{
		ctx:          ctx,
		stopC:        make(chan struct{}),
		expiredSlotC: infra.NewSafeClosableChannel[TimingWheelSlot](xtwOpt.getExpiredSlotBufferSize()),
		tasksMap:     kv.NewThreadSafeMap[JobID, Task](),
		isRunning:    &atomic.Bool{},
		clock:        xtwOpt.getClock(),
		idGenerator:  xtwOpt.getIDGenerator(),
		twEventPool:  newTimingWheelEventsPool(),
		stats:        xtwOpt.getStats(),
		name:         xtwOpt.getName(),
	}
	xtw.isRunning.Store(false)
	if p, err := ants.NewPool(xtwOpt.getWorkerPoolSize(), ants.WithPreAlloc(true)); err != nil {
		panic(err)
	} else {
		xtw.gPool = p
	}
	xtw.twEventDisruptor = ipc.NewXDisruptor[*timingWheelEvent](
		uint64(xtwOpt.getEventBufferSize()),
		ipc.NewXGoSchedBlockStrategy(),
		xtw.handleEvent,
	)
	xtw.dq = queue.NewArrayDelayQueue[TimingWheelSlot](ctx, xtwOpt.defaultDelayQueueCapacity())
	xtw.tw = newTimingWheel(
		ctx,
		xtwOpt.getBasicTickMilliseconds(),
		xtwOpt.getSlotIncrementSize(),
		xtwOpt.getClock().NowInDefaultTZ().UnixMilli(),
		xtw.stats,
		xtw.dq,
		xtw.clock,
	)
	xtw.isRunning.Store(true)
	xtw.schedule(ctx)
	return xtw
}
