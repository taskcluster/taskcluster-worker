package worker

import (
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

type updateError struct {
	statusCode int
	err        string
}

func (e updateError) Error() string {
	return e.err
}

type taskClaim struct {
	taskID     string
	runID      uint
	taskClaim  *queue.TaskClaimResponse
	definition *queue.TaskDefinitionResponse
}

func reportException(client client.Queue, task *TaskRun, reason runtime.ExceptionReason, log *logrus.Entry) *updateError {
	payload := queue.TaskExceptionRequest{Reason: string(reason)}
	_, _, err := client.ReportException(task.TaskID, strconv.FormatInt(int64(task.RunID), 10), &payload)
	if err != nil {
		log.WithField("error", err).Warn("Not able to report exception for task")
		return &updateError{err: err.Error()}
	}
	return nil
}

func reportFailed(client client.Queue, task *TaskRun, log *logrus.Entry) *updateError {
	_, _, err := client.ReportFailed(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))
	if err != nil {
		log.WithField("error", err).Warn("Not able to report failed completion for task.")
		return &updateError{err: err.Error()}
	}
	return nil
}

func reportCompleted(client client.Queue, task *TaskRun, log *logrus.Entry) *updateError {
	_, _, err := client.ReportCompleted(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))
	if err != nil {
		log.WithField("error", err).Warn("Not able to report successful completion for task.")
		return &updateError{err: err.Error()}
	}
	return nil
}

func claimTask(client client.Queue, taskID string, runID uint, workerID string, workerGroup string, log *logrus.Entry) (*taskClaim, *updateError) {
	log.Info("Claiming task")
	payload := queue.TaskClaimRequest{
		WorkerGroup: workerGroup,
		WorkerID:    workerID,
	}

	tcrsp, callSummary, err := client.ClaimTask(taskID, strconv.FormatInt(int64(runID), 10), &payload)
	// check if an error occurred...
	if err != nil {
		e := &updateError{
			err:        err.Error(),
			statusCode: callSummary.HttpResponse.StatusCode,
		}
		var errorMessage string
		switch {
		case callSummary.HttpResponse.StatusCode == 401 || callSummary.HttpResponse.StatusCode == 403:
			errorMessage = "Not authorized to claim task."
		case callSummary.HttpResponse.StatusCode >= 500:
			errorMessage = "Server error when attempting to claim task."
		default:
			errorMessage = "Received an error with a status code other than 401/403/500."
		}
		log.WithFields(logrus.Fields{
			"error":      err,
			"statusCode": callSummary.HttpResponse.StatusCode,
		}).Error(errorMessage)
		return nil, e
	}

	return &taskClaim{
		taskID:     taskID,
		runID:      runID,
		taskClaim:  tcrsp,
		definition: &tcrsp.Task,
	}, nil
}

func reclaimTask(client client.Queue, task *TaskRun, log *logrus.Entry) (*queue.TaskReclaimResponse, *updateError) {
	log.Info("Reclaiming task")
	tcrsp, _, err := client.ReclaimTask(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))

	// check if an error occurred...
	if err != nil {
		return nil, &updateError{err: err.Error()}
	}

	log.Info("Reclaimed task successfully")
	return tcrsp, nil
}
