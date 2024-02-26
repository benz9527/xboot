package timer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/panjf2000/ants/v2"

	"github.com/benz9527/xboot/lib/hrtime"
	"github.com/benz9527/xboot/lib/id"
	"github.com/benz9527/xboot/lib/infra"
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
	ctx              context.Context
	globalDqRef      queue.DelayQueue[TimingWheelSlot]
	overflowWheelRef unsafe.Pointer // same as kafka TimingWheel(*timingWheel)
	tickMs           int64
	startMs          int64 // baseline startup timestamp
	interval         int64
	currentTimeMs    int64
	slotSize         int64 // in kafka it is wheelSize
	globalStats      *timingWheelStats
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
		return fmt.Errorf("[timing wheel] task %s is cancelled, %w",
			task.GetJobID(), ErrTimingWheelTaskCancelled)
	}

	taskExpiredMs := task.GetExpiredMs()
	currentTimeMs := tw.clock.NowInDefaultTZ().UnixMilli()
	tickMs := tw.GetTickMs()
	interval := tw.GetInterval()
	slotSize := tw.GetSlotSize()
	diff := taskExpiredMs - currentTimeMs

	if level == 0 && diff <= tickMs {
		task.setSlot(immediateExpiredSlot)
		return fmt.Errorf("[timing wheel] task task expired ms  %d is before %d, %w",
			taskExpiredMs, currentTimeMs+tickMs, ErrTimingWheelTaskIsExpired)
	}
	if diff > tickMs && diff < interval {
		virtualID := taskExpiredMs / tickMs
		slotID := virtualID % slotSize
		slot := tw.slots[slotID]
		if slot.GetExpirationMs() == virtualID*tickMs {
			if err := slot.AddTask(task); err != nil {
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
	stats *timingWheelStats,
	dq queue.DelayQueue[TimingWheelSlot],
	clock hrtime.Clock,
) TimingWheel {
	tw := &timingWheel{
		ctx:           ctx,
		tickMs:        tickMs,
		startMs:       startMs,
		slotSize:      slotSize,
		globalStats:   stats,
		interval:      tickMs * slotSize,
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
	tw             TimingWheel
	ctx            context.Context
	dq             queue.DelayQueue[TimingWheelSlot] // Do not use the timer.Ticker
	tasksMap       map[JobID]Task
	stopC          chan struct{}
	expiredSlotC   infra.ClosableChannel[TimingWheelSlot]
	twEventC       infra.ClosableChannel[*timingWheelEvent]
	twEventPool    *timingWheelEventsPool
	gPool          *ants.Pool
	stats          *timingWheelStats
	isRunning      *atomic.Bool
	clock          hrtime.Clock
	idGenerator    id.Gen
	name           string
	isStatsEnabled bool
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
		slog.Warn("[timing wheel] timing wheel is already shutdown")
		return
	}
	xtw.isRunning.Store(false)

	close(xtw.stopC)
	_ = xtw.expiredSlotC.Close()
	_ = xtw.twEventC.Close()
	xtw.gPool.Release()

	runtime.SetFinalizer(xtw, func(xtw *xTimingWheels) {
		xtw.dq = nil
		clear(xtw.tasksMap)
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
		return nil, fmt.Errorf("[timing wheel] delay ms %d is less than tick ms %d, %w",
			delayMs.Milliseconds(), xtw.GetTickMs(), ErrTimingWheelTaskTooShortExpiration)
	}
	if fn == nil {
		return nil, ErrTimingWheelEmptyJob
	}

	var now = xtw.clock.NowInDefaultTZ()
	task := NewOnceTask(
		xtw.ctx,
		JobID(fmt.Sprintf("%d", xtw.idGenerator())),
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

	var now = xtw.clock.NowInDefaultTZ()
	task := NewRepeatTask(
		xtw.ctx,
		JobID(fmt.Sprintf("%d", xtw.idGenerator())),
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
					slog.Warn("[timing wheel] event channel has been closed")
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
					_ = xtw.gPool.Submit(func() {
						slot.Flush(xtw.handleTask)
					})
				}
			case event := <-eventC:
				switch op := event.GetOperation(); op {
				case addTask, reAddTask:
					task, ok := event.GetTask()
					if !ok {
						goto recycle
					}
					if err := xtw.addTask(task); errors.Is(err, ErrTimingWheelTaskIsExpired) {
						xtw.handleTask(task)
					} else if errors.Is(err, ErrTimingWheelTaskUnableToBeAddedToSlot) {
						slog.Error("unable add a task to slot", "ms", task.GetExpiredMs())
					}
					if op == addTask {
						xtw.stats.RecordJobAliveCount(1)
					}
				case cancelTask:
					jobID, ok := event.GetCancelTaskJobID()
					if !ok || cancelDisabled.(bool) {
						goto recycle
					}
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
		xtw.stats.RecordJobAliveCount(-1)
	}()

	task.Cancel()

	delete(xtw.tasksMap, jobID)
	return nil
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

func WithTimingWheelSnowflakeID(datacenterID, machineID int64) TimingWheelsOption {
	return func(xtw *xTimingWheels) {
		idGenerator, err := id.StandardSnowFlakeID(datacenterID, machineID, func() time.Time {
			return xtw.clock.NowInDefaultTZ()
		})
		if err != nil {
			panic(err)
		}
		xtw.idGenerator = idGenerator
	}
}
