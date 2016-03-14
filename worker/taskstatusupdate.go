package worker

import (
	"strconv"

	"github.com/Sirupsen/logrus"
	tcqueue "github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type updateError struct {
	statusCode int
	err        string
}

func (e updateError) Error() string {
	return e.err
}

type taskClaim struct {
	taskID      string
	runID       uint
	taskClaim   tcqueue.TaskClaimResponse
	definition  tcqueue.TaskDefinitionResponse
	queueClient queueClient
}

func reportException(client queueClient, task *TaskRun, reason runtime.ExceptionReason, log *logrus.Entry) *updateError {
	payload := tcqueue.TaskExceptionRequest{Reason: string(reason)}
	_, _, err := client.ReportException(task.TaskID, strconv.FormatInt(int64(task.RunID), 10), &payload)
	if err != nil {
		log.WithField("error", err).Warn("Not able to report exception for task")
		return &updateError{err: err.Error()}
	}
	return nil
}

func reportFailed(client queueClient, task *TaskRun, log *logrus.Entry) *updateError {
	_, _, err := client.ReportFailed(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))
	if err != nil {
		log.WithField("error", err).Warn("Not able to report failed completion for task.")
		return &updateError{err: err.Error()}
	}
	return nil
}

func reportCompleted(client queueClient, task *TaskRun, log *logrus.Entry) *updateError {
	_, _, err := client.ReportCompleted(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))
	if err != nil {
		log.WithField("error", err).Warn("Not able to report successful completion for task.")
		return &updateError{err: err.Error()}
	}
	return nil
}

func claimTask(client queueClient, taskID string, runID uint, workerID string, workerGroup string, log *logrus.Entry) (*taskClaim, *updateError) {
	log.Info("Claiming task")
	payload := tcqueue.TaskClaimRequest{
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
		taskID: taskID,
		runID:  runID,
		// TODO (garndt): replace with client retrieved from the task context.
		queueClient: tcqueue.New(
			&tcclient.Credentials{
				ClientId:    tcrsp.Credentials.ClientID,
				AccessToken: tcrsp.Credentials.AccessToken,
				Certificate: tcrsp.Credentials.Certificate,
			},
		),
		taskClaim:  *tcrsp,
		definition: tcrsp.Task,
	}, nil
}

func reclaimTask(client queueClient, task *TaskRun, log *logrus.Entry) *updateError {
	log.Info("Reclaiming task")
	tcrsp, _, err := client.ReclaimTask(task.TaskID, strconv.FormatInt(int64(task.RunID), 10))

	// check if an error occurred...
	if err != nil {
		return &updateError{err: err.Error()}
	}

	task.taskReclaim = *tcrsp
	log.Info("Reclaimed task successfully")
	return nil
}
