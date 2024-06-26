package timer

// Ref:
// https://dl.acm.org/doi/10.1145/41457.37504
// https://ieeexplore.ieee.org/document/650142
// github ref:
// https://github.com/apache/kafka/tree/trunk/server-common/src/main/java/org/apache/kafka/server/util/timer
// https://github.com/RussellLuo/timingwheel (Not a good implementation)

import (
	"context"
	"time"
)

type twError string

func (e twError) Error() string { return string(e) }

const (
	ErrTimingWheelStopped                   = twError("[timing-wheels] stopped")
	ErrTimingWheelTaskNotFound              = twError("[timing-wheels] task not found")
	ErrTimingWheelTaskEmptyJobID            = twError("[timing-wheels] empty job id in task")
	ErrTimingWheelEmptyJob                  = twError("[timing-wheels] empty job in task")
	ErrTimingWheelTaskIsExpired             = twError("[timing-wheels] task is expired")
	ErrTimingWheelTaskUnableToBeAddedToSlot = twError("[timing-wheels] task unable to be added to a flushed slot")
	ErrTimingWheelTaskUnableToBeRemoved     = twError("[timing-wheels] task unable to be removed")
	ErrTimingWheelTaskTooShortExpiration    = twError("[timing-wheels] task expiration is too short")
	ErrTimingWheelUnknownScheduler          = twError("[timing-wheels] unknown schedule")
	ErrTimingWheelTaskCancelled             = twError("[timing-wheels] task cancelled")
)

type TimingWheelCommonMetadata interface {
	// GetTickMs returns the baseline tick ms (interval) of the timing-wheel.
	GetTickMs() int64
	// GetStartMs returns the start ms of the timing-wheel.
	GetStartMs() int64
}

type TimingWheel interface {
	TimingWheelCommonMetadata
	GetInterval() int64
	GetCurrentTimeMs() int64
}

type Scheduler interface {
	// next returns the next expiredMs.
	next(beginMs int64) (nextExpiredMs int64)
	// GetRestLoopCount returns the rest loop count.
	// If the rest loop count is -1, it means that the task will run forever unless cancel manually.
	GetRestLoopCount() int64
}

type TimingWheels interface {
	TimingWheelCommonMetadata
	// AddTask adds a task to the timing wheels.
	AddTask(task Task) error
	// CancelTask cancels a task by jobID.
	CancelTask(jobID JobID) error
	// Shutdown stops the timing wheels
	Shutdown()
	// AfterFunc schedules a function to run after the duration delayMs.
	AfterFunc(delayMs time.Duration, fn Job) (Task, error)
	// ScheduleFunc schedules a function to run at a certain time generated by the schedule.
	ScheduleFunc(schedFn func() Scheduler, fn Job) (Task, error)
}

// JobID is the unique identifier of a job
type JobID string

// Job is the function that will be executed by the timing wheel
type Job func(ctx context.Context, metadata JobMetadata)

type JobType uint8

const (
	OnceJob JobType = iota
	RepeatedJob
)

func (t JobType) String() string {
	switch t {
	case OnceJob:
		return "once"
	case RepeatedJob:
		return "repeat"
	default:
	}
	return "unknown"
}

// All metadata interfaces are designed for debugging and monitoring friendly.

// JobMetadata describes the metadata of a job
// Each slot in the timing wheel is a linked list of jobs
type JobMetadata interface {
	// GetJobID returns the jobID of the job, unique identifier.
	GetJobID() JobID
	// GetExpiredMs returns the expirationMs of the job.
	GetExpiredMs() int64
	// GetRestLoopCount returns the rest loop count.
	GetRestLoopCount() int64
	// GetJobType returns the job type.
	GetJobType() JobType
}

// Task is the interface that wraps the Job
type Task interface {
	JobMetadata
	GetJobMetadata() JobMetadata
	// GetJob returns the job function.
	GetJob() Job
	// GetSlot returns the slot of the job.
	GetSlot() TimingWheelSlot
	// setSlot sets the slot of the job, it is a private method.
	setSlot(slot TimingWheelSlot)
	// GetPreviousSlotMetadata returns the previous slot metadata of the job.
	GetPreviousSlotMetadata() TimingWheelSlotMetadata
	// setPreviousSlotMetadata sets the current slot metadata of the job.
	setSlotMetadata(slotMetadata TimingWheelSlotMetadata)
	Cancel() bool
	Cancelled() bool
}

// ScheduledTask is the interface that wraps the repeat Job
type ScheduledTask interface {
	Task
	UpdateNextScheduledMs()
}

// TaskHandler is a function that reinserts a task into the timing wheel.
// It means that the task should be executed periodically or repeatedly for a certain times.
// Reinsert will add current task to next slot, higher level slot (overflow wheel) or
// the same level slot (current wheel) depending on the expirationMs of the task.
// When the task is reinserted, the expirationMs of the task should be updated.
//  1. Check if the task is cancelled. If so, stop reinserting.
//  2. Check if the task's loop count is greater than 0. If so, decrease the loop count and reinsert.
//  3. Check if the task's loop count is -1 (run forever unless cancel manually).
//     If so, reinsert and update the expirationMs.
type TaskHandler func(Task) // Core function

type TimingWheelSlotMetadata interface {
	// GetExpirationMs returns the expirationMs of the slot.
	GetExpirationMs() int64
	// setExpirationMs sets the expirationMs of the slot.
	setExpirationMs(expirationMs int64) bool
	// GetSlotID returns the slotID of the slot, easy for debugging.
	GetSlotID() int64
	// setSlotID sets the slotID of the slot, easy for debugging.
	setSlotID(slotID int64)
	// GetLevel returns the level of the slot, easy for debugging.
	GetLevel() int64
	// setLevel sets the level of the slot, easy for debugging.
	setLevel(level int64)
}

// TimingWheelSlot is the interface that wraps the slot, in kafka, it is called bucket.
type TimingWheelSlot interface {
	TimingWheelSlotMetadata
	// GetMetadata returns the metadata of the slot.
	GetMetadata() TimingWheelSlotMetadata
	// AddTask adds a task to the slot.
	AddTask(Task) error
	// RemoveTask removes a task from the slot.
	RemoveTask(Task) bool
	// Flush flushes all tasks in the slot generally,
	// but it should be called in a loop.
	Flush(TaskHandler)
}
