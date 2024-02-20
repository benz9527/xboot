package timer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/panjf2000/ants/v2"

	"github.com/benz9527/xboot/lib/ipc"
	"github.com/benz9527/xboot/lib/queue"
)

var (
	_ TimingWheel  = (*timingWheel)(nil)
	_ TimingWheels = (*xTimingWheels)(nil)
)

type timingWheel struct {
	slots []TimingWheelSlot // alignment 8, size 24; in kafka it is buckets
	// ctx is used to shut down the timing wheel and pass
	// value to control debug info.
	ctx                  context.Context
	globalDqRef          queue.DelayQueue[TimingWheelSlot]
	overflowWheelRef     unsafe.Pointer // same as kafka TimingWheel(*timingWheel)
	tickMs               int64
	startMs              int64 // baseline startup timestamp
	interval             int64
	currentTimeMs        int64
	slotSize             int64 // in kafka it is wheelSize
	globalSlotCounterRef *atomic.Int64
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
		return fmt.Errorf("[timing wheel] task %s is cancelled, %w",
			task.GetJobID(), ErrTimingWheelTaskCancelled)
	}

	taskExpiredMs := task.GetExpiredMs()
	currentTimeMs := time.Now().UTC().UnixMilli()
	tickMs := tw.GetTickMs()
	interval := tw.GetInterval()
	slotSize := tw.GetSlotSize()
	diff := taskExpiredMs - currentTimeMs

	if diff < tickMs {
		task.setSlot(immediateExpiredSlot)
		tw.advanceClock(currentTimeMs)
		tw.globalSlotCounterRef.Add(1)
		return fmt.Errorf("[timing wheel] task task expired ms  %d is before %d, %w",
			taskExpiredMs, currentTimeMs+tickMs, ErrTimingWheelTaskIsExpired)
	} else if diff >= tickMs && diff < interval {
		virtualID := taskExpiredMs / tickMs
		slotID := virtualID % slotSize
		slot := tw.slots[slotID]
		slotMs := slot.GetExpirationMs()
		if slot.GetExpirationMs() != (virtualID*tickMs) && !slot.setExpirationMs(virtualID*tickMs) { // FIXME data race
			err := fmt.Errorf("[timing wheel] slot (level:%d) (old:%d<->new:%d) unable update the expiration, %w",
				level, slotMs, virtualID*tickMs, ErrTimingWheelTaskUnableToBeAddedToSlot)
			slog.Error("[timing wheel] add task error", "error", err)
			return err
		}

		slot.setSlotID(slotID)
		slot.setLevel(level)
		slot.AddTask(task)
		tw.globalDqRef.Offer(slot, slot.GetExpirationMs())
		return nil
	}
	// Out of the interval. Put it into the higher interval timing wheel
	oftw := tw.getOverflowTimingWheel()
	if oftw == nil {
		tw.setOverflowTimingWheel(newTimingWheel(
			tw.ctx,
			interval,
			slotSize,
			currentTimeMs,
			tw.globalSlotCounterRef,
			tw.globalDqRef,
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
	slotCounter *atomic.Int64,
	dq queue.DelayQueue[TimingWheelSlot],
) TimingWheel {
	tw := &timingWheel{
		ctx:                  ctx,
		tickMs:               tickMs,
		startMs:              startMs,
		slotSize:             slotSize,
		globalSlotCounterRef: slotCounter,
		interval:             tickMs * slotSize,
		currentTimeMs:        startMs - (startMs % tickMs), // truncate the remainder as startMs left boundary
		slots:                make([]TimingWheelSlot, slotSize),
		globalDqRef:          dq,
	}
	// Slot initialize by doubly linked list.
	for i := int64(0); i < slotSize; i++ {
		tw.slots[i] = NewXSlot()
	}
	tw.globalSlotCounterRef.Add(slotSize)
	tw.setOverflowTimingWheel(nil)
	return tw
}

const (
	disableTimingWheelsSchedulePoll        = "disableTWSPoll"
	disableTimingWheelsScheduleCancelTask  = "disableTWSCancelTask"
	disableTimingWheelsScheduleExpiredSlot = "disableTWSExpSlot"
)

type xTimingWheels struct {
	tw             TimingWheel
	ctx            context.Context
	dq             queue.DelayQueue[TimingWheelSlot] // Do not use the timer.Ticker
	tasksMap       map[JobID]Task
	stopC          chan struct{}
	expiredSlotC   ipc.ClosableChannel[TimingWheelSlot]
	twEventC       ipc.ClosableChannel[*timingWheelEvent]
	twEventPool    *timingWheelEventsPool
	gPool          *ants.Pool
	stats          *timingWheelStats
	taskCounter    *atomic.Int64
	slotCounter    *atomic.Int64
	isRunning      *atomic.Bool
	name           string
	isStatsEnabled bool
}

func (xtw *xTimingWheels) GetTickMs() int64 {
	return xtw.tw.GetTickMs()
}

func (xtw *xTimingWheels) GetStartMs() int64 {
	return xtw.tw.GetStartMs()
}

func (xtw *xTimingWheels) GetTaskCounter() int64 {
	return xtw.taskCounter.Load()
}

func (xtw *xTimingWheels) GetSlotSize() int64 {
	return xtw.slotCounter.Load()
}

func (xtw *xTimingWheels) Shutdown() {
	if old := xtw.isRunning.Swap(false); !old {
		slog.Warn("[timing wheel] timing wheel is already shutdown")
		return
	}
	xtw.dq = nil
	xtw.isRunning.Store(false)

	// FIXME close on channel is no empty and will cause panic.
	close(xtw.stopC)
	_ = xtw.expiredSlotC.Close()
	_ = xtw.twEventC.Close()
	xtw.gPool.Release()

	// FIXME map clear data race
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
		return nil, fmt.Errorf("[timing wheel] delay ms %d is less than tick ms %d, %w",
			delayMs.Milliseconds(), xtw.GetTickMs(), ErrTimingWheelTaskTooShortExpiration)
	}
	if fn == nil {
		return nil, ErrTimingWheelEmptyJob
	}

	now := time.Now().UTC()
	task := NewOnceTask(
		xtw.ctx,
		JobID(fmt.Sprintf("%d", now.UnixNano())), // FIXME UUID
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

func (xtw *xTimingWheels) ScheduleFunc(schedFn func() Scheduler, fn Job) (Task, error) {
	if schedFn == nil {
		return nil, ErrTimingWheelUnknownScheduler
	}
	if fn == nil {
		return nil, ErrTimingWheelEmptyJob
	}

	now := time.Now()
	task := NewRepeatTask(
		xtw.ctx,
		JobID(fmt.Sprintf("%d", now.UnixNano())), // FIXME UUID
		now.UnixMilli(), schedFn(),
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

func (xtw *xTimingWheels) CancelTask(jobID JobID) error {
	if len(jobID) <= 0 {
		return ErrTimingWheelTaskEmptyJobID
	}

	if xtw.isRunning.Load() {
		return ErrTimingWheelStopped
	}
	task, ok := xtw.tasksMap[jobID]
	if !ok {
		return ErrTimingWheelTaskNotFound
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
				slog.Error("[timing wheel] event schedule panic recover", "error", err, "stack", debug.Stack())
			}
		}()
		cancelDisabled := ctx.Value(disableTimingWheelsScheduleCancelTask)
		if cancelDisabled == nil {
			cancelDisabled = false
		}
		eventC := xtw.twEventC.Wait()
		for {
			select {
			case <-ctx.Done():
				xtw.Shutdown()
				return
			case <-xtw.stopC:
				return
			default:
				if xtw.twEventC.IsClosed() {
					slog.Warn("[timing wheel] event channel has been closed")
					return
				}
			}

			select {
			case event := <-eventC:
				switch op := event.GetOperation(); op {
				case addTask, reAddTask:
					task, ok := event.GetTask()
					if !ok {
						goto recycle
					}
					if err := xtw.addTask(task); errors.Is(err, ErrTimingWheelTaskIsExpired) {
						xtw.handleTask(task)
					}
					if op == addTask {
						xtw.taskCounter.Add(1)
					}
				case cancelTask:
					jobID, ok := event.GetCancelTaskJobID()
					if !ok || cancelDisabled.(bool) {
						goto recycle
					}
					if err := xtw.cancelTask(jobID); err == nil {
						xtw.taskCounter.Add(-1)
					}
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
				slog.Warn("[timing wheel] delay queue expired slot channel disabled")
				return
			}
			defer func() {
				if err := recover(); err != nil {
					slog.Error("[timing wheel] expired slot schedule panic recover", "error", err, "stack", debug.Stack())
				}
			}()
			slotC := xtw.expiredSlotC.Wait()
			for {
				select {
				default:
					if xtw.expiredSlotC.IsClosed() {
						return
					}
				case <-ctx.Done():
					xtw.Shutdown()
					return
				case <-xtw.stopC:
					return
				case slot := <-slotC:
					xtw.advanceClock(slot.GetExpirationMs())
					// Here related to slot level upgrade and downgrade.
					slot.Flush(xtw.handleTask)
				}
			}
		}(ctx.Value(disableTimingWheelsScheduleExpiredSlot))
	})
	_ = xtw.gPool.Submit(func() {
		func(disabled any) {
			if disabled != nil && disabled.(bool) {
				slog.Warn("[timing wheel] delay queue poll disabled")
				return
			}
			defer func() {
				if err := recover(); err != nil {
					slog.Error("[timing wheel] poll schedule panic recover", "error", err, "stack", debug.Stack())
				}
				slog.Warn("[timing wheel] delay queue exit")
			}()
			xtw.dq.PollToChan(func() int64 {
				return time.Now().UTC().UnixMilli()
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
	// FIXME Recursive function to addTask a task, need to measure the performance.
	err := xtw.tw.(*timingWheel).addTask(task, 0)
	if err == nil || errors.Is(err, ErrTimingWheelTaskIsExpired) {
		// FIXME map data race
		xtw.tasksMap[task.GetJobID()] = task
	}
	return err
}

// handleTask all tasks which are called by this method
// will mean that the task must be in a slot ever and related slot
// has been expired.
func (xtw *xTimingWheels) handleTask(t Task) {
	if t == nil || !xtw.isRunning.Load() {
		slog.Info("[timing wheel] task is nil or timing wheel is stopped")
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
		return // Unknown task
	} else if prevSlotMetadata != nil {
		taskLevel = prevSlotMetadata.GetLevel()
		runNow = prevSlotMetadata.GetExpirationMs() == sentinelSlotExpiredMs
		runNow = runNow || taskLevel == 0 && t.GetExpiredMs() <= prevSlotMetadata.GetExpirationMs()+xtw.GetTickMs()
	}
	runNow = runNow || t.GetExpiredMs() <= time.Now().UTC().UnixMilli()

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
			_ = xtw.twEventC.Send(event)
		} else {
			event.ReAddTask(t)
			_ = xtw.twEventC.Send(event)
		}
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
		return ErrTimingWheelStopped
	}

	task, ok := xtw.tasksMap[jobID]
	if !ok {
		return ErrTimingWheelTaskNotFound
	}

	if task.GetSlot() != nil && !task.GetSlot().RemoveTask(task) {
		return ErrTimingWheelTaskUnableToBeRemoved
	}

	defer func() {
		xtw.stats.IncreaseJobCancelledCount()
	}()

	task.Cancel()

	delete(xtw.tasksMap, jobID)
	return nil
}

// NewTimingWheels creates a new timing wheel.
// @param startMs the start time in milliseconds, example value time.Now().UnixMilli().
//
//	Same as the kafka, Time.SYSTEM.hiResClockMs() is used.
func NewTimingWheels(ctx context.Context, startMs int64, opts ...TimingWheelsOption) TimingWheels {
	if ctx == nil {
		return nil
	}

	xtw := &xTimingWheels{
		ctx:          ctx,
		taskCounter:  &atomic.Int64{},
		slotCounter:  &atomic.Int64{},
		stopC:        make(chan struct{}),
		twEventC:     ipc.NewSafeClosableChannel[*timingWheelEvent](1024),
		expiredSlotC: ipc.NewSafeClosableChannel[TimingWheelSlot](128),
		tasksMap:     make(map[JobID]Task),
		isRunning:    &atomic.Bool{},
		twEventPool:  newTimingWheelEventsPool(),
		tw:           &timingWheel{},
	}
	xtw.isRunning.Store(false)
	for _, o := range opts {
		if o != nil {
			o(xtw)
		}
	}
	// Temporarily store the configuration
	tw := xtw.tw.(*timingWheel)
	tw.startMs = startMs

	if tw.tickMs <= 0 {
		tw.tickMs = time.Millisecond.Milliseconds()
	}
	if tw.slotSize <= 0 {
		tw.slotSize = 20
	}
	xtw.dq = queue.NewArrayDelayQueue[TimingWheelSlot](ctx, 128)
	xtw.tw = newTimingWheel(
		ctx,
		tw.tickMs,
		tw.slotSize,
		tw.startMs,
		xtw.slotCounter,
		xtw.dq,
	)
	if p, err := ants.NewPool(128, ants.WithPreAlloc(true)); err != nil {
		panic(err)
	} else {
		xtw.gPool = p
	}
	if xtw.name == "" {
		// FIXME UUID
		xtw.name = "default-" + strconv.FormatInt(xtw.GetStartMs(), 10)
	}
	if xtw.isStatsEnabled {
		xtw.stats = newTimingWheelStats(xtw)
	}
	xtw.schedule(ctx)
	return xtw
}

type TimingWheelsOption func(*xTimingWheels)

func WithTimingWheelTickMs(basicTickMs time.Duration) TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		tw := xtw.tw.(*timingWheel)
		if tw == nil {
			return
		}
		tw.tickMs = basicTickMs.Milliseconds()
	}
}

func WithTimingWheelSlotSize(slotSize int64) TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		tw := xtw.tw.(*timingWheel)
		if tw == nil {
			return
		}
		tw.slotSize = slotSize
	}
}

func WithTimingWheelName(name string) TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		xtw.name = name
	}
}
