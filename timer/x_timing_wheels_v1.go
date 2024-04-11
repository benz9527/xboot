package timer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/panjf2000/ants/v2"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
	"github.com/benz9527/xboot/lib/kv"
	"github.com/benz9527/xboot/lib/queue"
)

var (
	_ TimingWheel  = (*timingWheel)(nil)
	_ TimingWheels = (*xTimingWheels)(nil)
)

type timingWheel struct {
	slots []TimingWheelSlot // In kafka it is buckets
	// ctx is used to shut down the timing wheel and pass
	// value to control debug info.
	ctx              context.Context
	globalDqRef      queue.DelayQueue[TimingWheelSlot]
	overflowWheelRef unsafe.Pointer // same as kafka TimingWheel(*timingWheel)
	tickMs           int64
	startMs          int64 // baseline startup timestamp
	interval         int64
	currentTimeMs    int64
	slotSize         int64 // in kafka it is wheelSize
	globalStats      *xTimingWheelsStats
	clock            hrtime.Clock
}

func (tw *timingWheel) GetTickMs() int64 {
	return atomic.LoadInt64(&tw.tickMs)
}

func (tw *timingWheel) GetStartMs() int64 {
	return atomic.LoadInt64(&tw.startMs)
}

func (tw *timingWheel) GetCurrentTimeMs() int64 {
	return atomic.LoadInt64(&tw.currentTimeMs)
}

func (tw *timingWheel) GetInterval() int64 {
	return atomic.LoadInt64(&tw.interval)
}

func (tw *timingWheel) GetSlotSize() int64 {
	return atomic.LoadInt64(&tw.slotSize)
}

func (tw *timingWheel) getOverflowTimingWheel() TimingWheel {
	return *(*TimingWheel)(atomic.LoadPointer(&tw.overflowWheelRef))
}

func (tw *timingWheel) setOverflowTimingWheel(oftw TimingWheel) {
	atomic.StorePointer(&tw.overflowWheelRef, unsafe.Pointer(&oftw))
}

// Here related to slot level upgrade and downgrade.
func (tw *timingWheel) advanceClock(slotExpiredMs int64) {
	currentTimeMs := tw.GetCurrentTimeMs()
	tickMs := tw.GetTickMs()
	if slotExpiredMs >= currentTimeMs+tickMs {
		currentTimeMs = slotExpiredMs - (slotExpiredMs % tickMs) // truncate the remainder as slot expiredMs left boundary
		atomic.StoreInt64(&tw.currentTimeMs, currentTimeMs)      // update the current time
		oftw := tw.getOverflowTimingWheel()
		if oftw != nil {
			oftw.(*timingWheel).advanceClock(currentTimeMs)
		}
	}
}

func (tw *timingWheel) addTask(task Task, level int64) error {
	if len(task.GetJobID()) <= 0 {
		return ErrTimingWheelTaskEmptyJobID
	}
	if task.GetJob() == nil {
		return ErrTimingWheelEmptyJob
	}
	if task.Cancelled() {
		return ErrTimingWheelTaskCancelled
	}

	taskExpiredMs := task.GetExpiredMs()
	currentTimeMs := tw.clock.NowInDefaultTZ().UnixMilli()
	tickMs := tw.GetTickMs()
	interval := tw.GetInterval()
	slotSize := tw.GetSlotSize()
	diff := taskExpiredMs - currentTimeMs

	if level == 0 && diff <= tickMs {
		task.setSlot(immediateExpiredSlot)
		return ErrTimingWheelTaskIsExpired
	}
	if diff > tickMs && diff < interval {
		virtualID := taskExpiredMs / tickMs
		slotID := virtualID % slotSize
		slot := tw.slots[slotID]
		if slot.GetExpirationMs() == virtualID*tickMs {
			if err := slot.AddTask(task); errors.Is(err, ErrTimingWheelTaskUnableToBeAddedToSlot) {
				task.setSlot(immediateExpiredSlot)
				return ErrTimingWheelTaskIsExpired
			} else if err != nil {
				return err
			}
		} else {
			if slot.setExpirationMs(virtualID * tickMs) {
				slot.setSlotID(slotID)
				slot.setLevel(level)
				if err := slot.AddTask(task); err != nil {
					return err
				}
				tw.globalDqRef.Offer(slot, slot.GetExpirationMs())
			}
		}
		return nil
	}
	// Out of the interval. Put it into the higher interval timing wheel
	if ovf := tw.getOverflowTimingWheel(); ovf == nil {
		tw.setOverflowTimingWheel(newTimingWheel(
			tw.ctx,
			interval,
			slotSize,
			currentTimeMs,
			tw.globalStats,
			tw.globalDqRef,
			tw.clock,
		))
	}
	// Tail recursive call, it will be free the previous stack frame.
	return tw.getOverflowTimingWheel().(*timingWheel).addTask(task, level+1)
}

func newTimingWheel(
	ctx context.Context,
	tickMs int64,
	slotSize int64,
	startMs int64,
	stats *xTimingWheelsStats,
	dq queue.DelayQueue[TimingWheelSlot],
	clock hrtime.Clock,
) TimingWheel {
	tw := &timingWheel{
		ctx:           ctx,
		tickMs:        tickMs,
		startMs:       startMs,
		slotSize:      slotSize,
		globalStats:   stats,
		interval:      tickMs * slotSize,            // Pay attention to the overflow
		currentTimeMs: startMs - (startMs % tickMs), // truncate the remainder as startMs left boundary
		slots:         make([]TimingWheelSlot, slotSize),
		globalDqRef:   dq,
		clock:         clock,
	}
	// Slot initializes by doubly linked list.
	for i := int64(0); i < slotSize; i++ {
		tw.slots[i] = NewXSlot()
	}
	tw.globalStats.RecordSlotCount(slotSize)
	tw.setOverflowTimingWheel(nil)
	return tw
}

const (
	disableTimingWheelsSchedulePoll       = "disableTWSPoll"
	disableTimingWheelsScheduleCancelTask = "disableTWSCancelTask"
)

type TimingWheelTimeSourceEnum int8

type xTimingWheels struct {
	tw           TimingWheel
	ctx          context.Context
	dq           queue.DelayQueue[TimingWheelSlot] // Do not use the timer.Ticker
	tasksMap     kv.ThreadSafeStorer[JobID, Task]
	stopC        chan struct{}
	expiredSlotC infra.ClosableChannel[TimingWheelSlot]
	twEventC     infra.ClosableChannel[*timingWheelEvent]
	twEventPool  *timingWheelEventsPool
	gPool        *ants.Pool
	stats        *xTimingWheelsStats
	isRunning    *atomic.Bool
	clock        hrtime.Clock
	idGenerator  id.Gen
	name         string
}

func (xtw *xTimingWheels) GetTickMs() int64 {
	return xtw.tw.GetTickMs()
}

func (xtw *xTimingWheels) GetStartMs() int64 {
	return xtw.tw.GetStartMs()
}

func (xtw *xTimingWheels) Shutdown() {
	if xtw == nil {
		return
	}
	if old := xtw.isRunning.Swap(false); !old {
		slog.Warn("[x-timing-wheels] timing wheel is already shutdown")
		return
	}
	xtw.isRunning.Store(false)

	close(xtw.stopC)
	_ = xtw.expiredSlotC.Close()
	_ = xtw.twEventC.Close()
	xtw.gPool.Release()

	runtime.SetFinalizer(xtw, func(xtw *xTimingWheels) {
		xtw.dq = nil
		_ = xtw.tasksMap.Purge()
	})
}

func (xtw *xTimingWheels) AddTask(task Task) error {
	if len(task.GetJobID()) <= 0 {
		return ErrTimingWheelTaskEmptyJobID
	}
	if task.GetJob() == nil {
		return ErrTimingWheelEmptyJob
	}
	if !xtw.isRunning.Load() {
		return ErrTimingWheelStopped
	}
	event := xtw.twEventPool.Get()
	event.AddTask(task)
	return xtw.twEventC.Send(event)
}

func (xtw *xTimingWheels) AfterFunc(delayMs time.Duration, fn Job) (Task, error) {
	if delayMs.Milliseconds() < xtw.GetTickMs() {
		return nil, infra.WrapErrorStackWithMessage(ErrTimingWheelTaskTooShortExpiration, "[x-timing-wheels] delay ms "+strconv.FormatInt(delayMs.Milliseconds(), 10)+
			" is less than tick ms "+strconv.FormatInt(xtw.GetTickMs(), 10))
	}
	if fn == nil {
		return nil, infra.WrapErrorStack(ErrTimingWheelEmptyJob)
	}

	var now = xtw.clock.NowInDefaultTZ()
	task := NewOnceTask(
		xtw.ctx,
		JobID(strconv.FormatUint(xtw.idGenerator(), 10)),
		now.Add(delayMs).UnixMilli(),
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

func (xtw *xTimingWheels) ScheduleFunc(schedFn func() Scheduler, fn Job) (Task, error) {
	if schedFn == nil {
		return nil, infra.WrapErrorStack(ErrTimingWheelUnknownScheduler)
	}
	if fn == nil {
		return nil, infra.WrapErrorStack(ErrTimingWheelEmptyJob)
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

func (xtw *xTimingWheels) CancelTask(jobID JobID) error {
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
	return xtw.twEventC.Send(event)
}

func (xtw *xTimingWheels) schedule(ctx context.Context) {
	if ctx == nil {
		return
	}
	// FIXME Block error mainly caused by producer and consumer speed mismatch, lock data race.
	//  Is there any limitation mechanism could gradually  control different interval task‘s execution timeout timestamp?
	//  Tasks piling up in the same slot will cause the timing wheel to be blocked or delayed.
	_ = xtw.gPool.Submit(func() {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("[x-timing-wheels] event schedule panic recover", "error", err, "stack", debug.Stack())
			}
		}()
		cancelDisabled := ctx.Value(disableTimingWheelsScheduleCancelTask)
		if cancelDisabled == nil {
			cancelDisabled = false
		}
		eventC := xtw.twEventC.Wait()
		slotC := xtw.expiredSlotC.Wait()
		for {
			select {
			case <-ctx.Done():
				xtw.Shutdown()
				return
			case <-xtw.stopC:
				return
			default:
				if xtw.twEventC.IsClosed() {
					slog.Warn("[x-timing-wheels] event channel has been closed")
					return
				}
				if xtw.expiredSlotC.IsClosed() {
					slog.Warn("[x-timing-wheels] slot channel has been closed")
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
					slot.setExpirationMs(slotHasBeenFlushedMs)
					slot.Flush(xtw.handleTask)
				}
			case event := <-eventC:
				switch op := event.GetOperation(); op {
				case addTask, reAddTask:
					task, ok := event.GetTask()
					if !ok {
						goto recycle
					}
					if err := xtw.addTask(task); errors.Is(err, ErrTimingWheelTaskIsExpired) {
						// Avoid data race.
						xtw.handleTask(task)
					}
					if op == addTask {
						xtw.stats.RecordJobAliveCount(1)
					}
				case cancelTask:
					jobID, ok := event.GetCancelTaskJobID()
					if !ok || cancelDisabled.(bool) {
						goto recycle
					}
					// Avoid data race
					_ = xtw.cancelTask(jobID)
				case unknown:
					fallthrough
				default:

				}
			recycle:
				xtw.twEventPool.Put(event)
			}
		}
	})
	_ = xtw.gPool.Submit(func() {
		func(disabled any) {
			if disabled != nil && disabled.(bool) {
				slog.Warn("[x-timing-wheels] delay queue poll disabled")
				return
			}
			defer func() {
				if err := recover(); err != nil {
					slog.Error("[x-timing-wheels] poll schedule panic recover", "error", err, "stack", debug.Stack())
				}
				slog.Warn("[x-timing-wheels] delay queue exit")
			}()
			xtw.dq.PollToChan(func() int64 {
				return xtw.clock.NowInDefaultTZ().UnixMilli()
			}, xtw.expiredSlotC)
		}(ctx.Value(disableTimingWheelsSchedulePoll))
	})
	xtw.isRunning.Store(true)
}

// Update all wheels' current time, in order to simulate the time is continuously incremented.
// Here related to slot level upgrade and downgrade.
func (xtw *xTimingWheels) advanceClock(timeoutMs int64) {
	xtw.tw.(*timingWheel).advanceClock(timeoutMs)
}

func (xtw *xTimingWheels) addTask(task Task) error {
	if task == nil || task.Cancelled() || !xtw.isRunning.Load() {
		return ErrTimingWheelStopped
	}
	err := xtw.tw.(*timingWheel).addTask(task, 0)
	if err == nil || errors.Is(err, ErrTimingWheelTaskIsExpired) {
		xtw.tasksMap.AddOrUpdate(task.GetJobID(), task)
	}
	return infra.WrapErrorStack(err)
}

// handleTask all tasks which are called by this method
// will mean that the task must be in a slot ever and related slot
// has been expired.
func (xtw *xTimingWheels) handleTask(t Task) {
	if t == nil || !xtw.isRunning.Load() {
		slog.Warn("[x-timing-wheels] handle task failed",
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
		_ = xtw.twEventC.Send(event)
	case RepeatedJob:
		var sTask Task
		if !runNow {
			sTask = t
		} else {
			if t.GetRestLoopCount() == 0 {
				event := xtw.twEventPool.Get()
				event.CancelTaskJobID(t.GetJobID())
				_ = xtw.twEventC.Send(event)
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
			_ = xtw.twEventC.Send(event)
		}
	}
	return
}

func (xtw *xTimingWheels) cancelTask(jobID JobID) error {
	if !xtw.isRunning.Load() {
		return infra.WrapErrorStack(ErrTimingWheelStopped)
	}

	task, ok := xtw.tasksMap.Get(jobID)
	if !ok {
		return infra.WrapErrorStack(ErrTimingWheelTaskNotFound)
	}

	if task.GetSlot() != nil && !task.GetSlot().RemoveTask(task) {
		return infra.WrapErrorStack(ErrTimingWheelTaskUnableToBeRemoved)
	}

	defer func() {
		xtw.stats.IncreaseJobCancelledCount()
		xtw.stats.RecordJobAliveCount(-1)
	}()

	task.Cancel()

	_, err := xtw.tasksMap.Delete(jobID)
	return infra.WrapErrorStack(err)
}

// NewXTimingWheels creates a new timing wheel.
// The same as the kafka, Time.SYSTEM.hiResClockMs() is used.
func NewXTimingWheels(ctx context.Context, opts ...TimingWheelsOption) TimingWheels {
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

	xtw := &xTimingWheels{
		ctx:          ctx,
		stopC:        make(chan struct{}),
		twEventC:     infra.NewSafeClosableChannel[*timingWheelEvent](xtwOpt.getEventBufferSize()),
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
