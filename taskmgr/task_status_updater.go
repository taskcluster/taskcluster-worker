package taskmgr

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	tcclient "github.com/taskcluster/taskcluster-client-go/queue"
)

// NOTE (garndt): This is still up in the air as I"m not sure if I like calling the
// update methods like this.  It is nice to just pass in an update object and the method
// takes care of calling it in a goroutine.

type TaskStatus string
type TaskStatusUpdate struct {
	Task          *TaskRun
	Status        TaskStatus
	IfStatusIn    map[TaskStatus]bool
	Reason        string
	WorkerId      string
	ProvisionerId string
	WorkerGroup   string
}

// Enumerate task status to aid life-cycle decision making
// Use strings for benefit of simple logging/reporting
const (
	Aborted   TaskStatus = "Aborted"
	Cancelled TaskStatus = "Cancelled"
	Succeeded TaskStatus = "Succeeded"
	Failed    TaskStatus = "Failed"
	Errored   TaskStatus = "Errored"
	Claimed   TaskStatus = "Claimed"
	Reclaimed TaskStatus = "Reclaimed"
)

type updateError struct {
	StatusCode int
	err        string
}

func (e updateError) Error() string {
	return e.err
}

func UpdateTaskStatus(ts TaskStatusUpdate, queue *tcclient.Queue, log *logrus.Entry) (err <-chan *updateError) {
	e := make(chan *updateError)

	logger := log.WithFields(logrus.Fields{
		"taskId": ts.Task.TaskId,
		"runId":  ts.Task.RunId,
	})

	// we'll make all these functions internal to TaskStatusHandler so that
	// they can only be called inside here, so that reading/writing to the
	// appropriate channels is the only way to trigger them, to ensure
	// proper concurrency handling

	reportException := func(task *TaskRun, reason string, log *logrus.Entry) *updateError {
		ter := tcclient.TaskExceptionRequest{Reason: reason}
		tsr, _, err := queue.ReportException(task.TaskId, strconv.FormatInt(int64(task.RunId), 10), &ter)
		if err != nil {
			log.WithField("error", err).Warn("Not able to report exception for task")
			return &updateError{err: err.Error()}
		}
		task.TaskClaimResponse.Status = tsr.Status
		return nil
	}

	reportFailed := func(task *TaskRun, log *logrus.Entry) *updateError {
		tsr, _, err := queue.ReportFailed(task.TaskId, strconv.FormatInt(int64(task.RunId), 10))
		if err != nil {
			log.WithField("error", err).Warn("Not able to report failed completion for task.")
			return &updateError{err: err.Error()}
		}
		task.TaskClaimResponse.Status = tsr.Status
		return nil
	}

	reportCompleted := func(task *TaskRun, log *logrus.Entry) *updateError {
		tsr, _, err := queue.ReportCompleted(task.TaskId, strconv.FormatInt(int64(task.RunId), 10))
		if err != nil {
			log.WithField("error", err).Warn("Not able to report successful completion for task.")
			return &updateError{err: err.Error()}
		}
		task.TaskClaimResponse.Status = tsr.Status
		return nil
	}

	claim := func(task *TaskRun, log *logrus.Entry) *updateError {
		log.Info("Claiming task")
		cr := tcclient.TaskClaimRequest{
			WorkerGroup: ts.WorkerGroup,
			WorkerId:    ts.WorkerId,
		}
		// Using the taskId and runId from the <MessageText> tag, the worker
		// must call queue.claimTask().
		tcrsp, callSummary, err := queue.ClaimTask(task.TaskId, strconv.FormatInt(int64(task.RunId), 10), &cr)
		// check if an error occurred...
		if err != nil {
			e := &updateError{err: err.Error()}
			// If the queue.claimTask() operation fails with a 4xx error, the
			// worker must delete the messages from the Azure queue (except 401).
			var errorMessage string
			switch {
			case callSummary.HttpResponse.StatusCode == 401 || callSummary.HttpResponse.StatusCode == 403:
				errorMessage = "Not authorized to claim task, *not* deleting it from Azure queue!"
			case callSummary.HttpResponse.StatusCode >= 500:
				errorMessage = "Server error when attempting to claim task."
			default:
				errorMessage = "Received an error with a status code other than 401/403/500.  Deleting message from the queue"
				// attempt to delete, but if it fails, log and continue
				// nothing we can do, and better to return the first 4xx error
				e.StatusCode = callSummary.HttpResponse.StatusCode
				if err != nil {
					errorMessage = "Not able to delete task from queue after receiving an unexpected status code."
				}
			}
			log.WithFields(logrus.Fields{
				"error":      err,
				"statusCode": callSummary.HttpResponse.StatusCode,
			}).Error(errorMessage)
			return e
		}
		task.TaskClaimResponse = *tcrsp
		return nil
	}

	reclaim := func(task *TaskRun, log *logrus.Entry) *updateError {
		log.Info("Reclaiming task")
		tcrsp, _, err := queue.ReclaimTask(task.TaskId, fmt.Sprintf("%d", task.RunId))

		// check if an error occurred...
		if err != nil {
			return &updateError{err: err.Error()}
		}

		task.TaskReclaimResponse = *tcrsp
		log.Info("Reclaimed task successfully")
		return nil
	}

	go func() {
		// only update if either IfStatusIn is nil
		// or it is non-nil but it has "true" value
		// for key of current status
		if ts.IfStatusIn == nil || ts.IfStatusIn[ts.Status] {
			task := ts.Task
			switch ts.Status {
			// Aborting is when you stop running a job you already claimed
			case Succeeded:
				e <- reportCompleted(task, logger)
			case Failed:
				e <- reportFailed(task, logger)
			case Errored:
				e <- reportException(task, ts.Reason, logger)
			case Claimed:
				e <- claim(task, logger)
			case Reclaimed:
				e <- reclaim(task, logger)
			default:
				err := &updateError{err: fmt.Sprintf("Internal error: unknown task status: %v", ts.Status)}
				logger.Error(err)
				e <- err
			}
		} else {
			// current status is such that we shouldn't update to new
			// status, so just report that no error occurred...
			e <- nil
		}
	}()
	return e
}
