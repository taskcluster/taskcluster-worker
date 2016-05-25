package worker

import (
	"fmt"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/httpbackoff"
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
	runID      int
	taskClaim  *queue.TaskClaimResponse
	definition *queue.TaskDefinitionResponse
}

func reportException(client client.Queue, task *TaskRun, reason runtime.ExceptionReason, log *logrus.Entry) *updateError {
	payload := queue.TaskExceptionRequest{Reason: string(reason)}
	_, err := client.ReportException(task.TaskID, strconv.FormatInt(int64(task.RunID), 10), &payload)
	if err != nil {
		log.WithField("error", err).Warn("Not able to report exception for task")
		return &updateError{err: err.Error()}
	}
	return nil
}

func reportFailed(client client.Queue, task *TaskRun, log *logrus.Entry) *updateError {
	_, err := client.ReportFailed(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))
	if err != nil {
		log.WithField("error", err).Warn("Not able to report failed completion for task.")
		return &updateError{err: err.Error()}
	}
	return nil
}

func reportCompleted(client client.Queue, task *TaskRun, log *logrus.Entry) *updateError {
	_, err := client.ReportCompleted(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))
	if err != nil {
		log.WithField("error", err).Warn("Not able to report successful completion for task.")
		return &updateError{err: err.Error()}
	}
	return nil
}

func claimTask(client client.Queue, taskID string, runID int, workerID string, workerGroup string, log *logrus.Entry) (*taskClaim, error) {
	log.Info("Claiming task")
	payload := queue.TaskClaimRequest{
		WorkerGroup: workerGroup,
		WorkerID:    workerID,
	}

	tcrsp, err := client.ClaimTask(taskID, strconv.FormatInt(int64(runID), 10), &payload)
	// check if an error occurred...
	if err != nil {
		switch err := err.(type) {
		case httpbackoff.BadHttpResponseCode:
			e := &updateError{
				err:        err.Error(),
				statusCode: err.HttpResponseCode,
			}
			var errorMessage string
			switch {
			case err.HttpResponseCode == 401 || err.HttpResponseCode == 403:
				errorMessage = fmt.Sprintf("Not authorized to claim task %s (status code %v).", taskID, err.HttpResponseCode)
			case err.HttpResponseCode >= 500:
				errorMessage = fmt.Sprintf("Server error (status code %v) when attempting to claim task %v.", err.HttpResponseCode, taskID)
			case err.HttpResponseCode != 200:
				errorMessage = fmt.Sprintf("Received http status code %v when claiming task %v.", err.HttpResponseCode, taskID)
			}
			log.WithFields(logrus.Fields{
				"error":      err,
				"statusCode": err.HttpResponseCode,
			}).Error(errorMessage)
			return nil, e

		default:
			log.WithFields(logrus.Fields{
				"error": err,
			}).Error(fmt.Sprintf("Unexpected error occurred when claiming task %s", taskID))
			return nil, err
		}
	}

	return &taskClaim{
		taskID:     taskID,
		runID:      runID,
		taskClaim:  tcrsp,
		definition: &tcrsp.Task,
	}, nil
}

func reclaimTask(client client.Queue, taskID string, runID int, log *logrus.Entry) (*queue.TaskReclaimResponse, *updateError) {
	log.Info("Reclaiming task")
	tcrsp, err := client.ReclaimTask(taskID, strconv.FormatInt(int64(runID), 10))

	// check if an error occurred...
	if err != nil {
		return nil, &updateError{err: err.Error()}
	}

	log.Info("Reclaimed task successfully")
	return tcrsp, nil
}
