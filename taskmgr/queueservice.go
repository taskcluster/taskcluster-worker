package taskmgr

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	logrus "github.com/Sirupsen/logrus"
	"github.com/taskcluster/httpbackoff"
	tcqueue "github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
)

type (
	// Used for modelling the xml we get back from Azure
	queueMessagesList struct {
		XMLName       xml.Name       `xml:"QueueMessagesList"`
		QueueMessages []queueMessage `xml:"QueueMessage"`
	}

	// Used for modelling the xml we get back from Azure
	queueMessage struct {
		MessageId    string `xml:"MessageId"`
		PopReceipt   string `xml:"PopReceipt"`
		DequeueCount int    `xml:"DequeueCount"`
		MessageText  string `xml:"MessageText"`
	}

	// TaskId and RunId are taken from the json encoding of
	// queueMessage.MessageId that we get back from Azure
	TaskRun struct {
		TaskId              string                         `json:"taskId"`
		RunId               uint                           `json:"runId"`
		SignedDeleteUrl     string                         `json:"-"`
		TaskClaimResponse   tcqueue.TaskClaimResponse      `json:"-"`
		TaskReclaimResponse tcqueue.TaskReclaimResponse    `json:"-"`
		Definition          tcqueue.TaskDefinitionResponse `json:"-"`
		reclaimTimer        *time.Timer
	}

	taskQueue struct {
		SignedDeleteUrl string `json:"signedDeleteUrl"`
		SignedPollUrl   string `json:"signedPollUrl"`
	}

	taskRuns []*TaskRun

	// TaskclusterQueue is an interface to the Queue client provided by the
	// taskcluster-client-go package.  Passing around an interface allows the
	// queue client to be mocked
	TaskclusterQueue interface {
		ReportCompleted(string, string) (*tcqueue.TaskStatusResponse, *tcclient.CallSummary, error)
		ReportException(string, string, *tcqueue.TaskExceptionRequest) (*tcqueue.TaskStatusResponse, *tcclient.CallSummary, error)
		ReportFailed(string, string) (*tcqueue.TaskStatusResponse, *tcclient.CallSummary, error)
		ClaimTask(string, string, *tcqueue.TaskClaimRequest) (*tcqueue.TaskClaimResponse, *tcclient.CallSummary, error)
		ReclaimTask(string, string) (*tcqueue.TaskReclaimResponse, *tcclient.CallSummary, error)
		PollTaskUrls(string, string) (*tcqueue.PollTaskUrlsResponse, *tcclient.CallSummary, error)
		CancelTask(string) (*tcqueue.TaskStatusResponse, *tcclient.CallSummary, error)
	}

	queueService struct {
		mu               sync.Mutex
		queues           []taskQueue
		Expires          tcclient.Time
		ExpirationOffset int
		client           TaskclusterQueue
		ProvisionerId    string
		WorkerType       string
		WorkerId         string
		WorkerGroup      string
		Log              *logrus.Entry
	}
)

// Given a number of tasks needed, the Azure task queues will be polled in order
// of priority until either there are no more tasks to claim, or the given number of
// tasks has been fulfilled.
func (q queueService) ClaimWork(ntasks int) []*TaskRun {
	q.Log.Debugf("Attempting to claim %d tasks.", ntasks)
	tasks := []*TaskRun{}
	taskRuns, err := q.retrieveTasksFromQueue(ntasks)
	if err != nil {
		// Log the error but just return an empty set of Task Runs.
		q.Log.WithField("error", err).Error("Error retrieving tasks to execute.")
		return []*TaskRun{}
	}

	tasks = q.claimTasks(taskRuns)
	return tasks
}

func (q queueService) claimTasks(tasks []*TaskRun) []*TaskRun {
	var wg sync.WaitGroup
	claims := []*TaskRun{}
	claimed := make(chan *TaskRun)
	wg.Add(len(tasks))
	for _, task := range tasks {
		go func(task *TaskRun) {
			defer wg.Done()
			success := q.claimTask(task)
			if success {
				claimed <- task
			}
		}(task)
	}
	wg.Wait()

	for claim := range claimed {
		claims = append(claims, claim)
	}

	return claims
}

// TODO (garndt): Move to some methods used by task manager as well.
func (q queueService) claimTask(task *TaskRun) bool {
	update := TaskStatusUpdate{
		Task:          task,
		Status:        Claimed,
		WorkerId:      q.WorkerId,
		ProvisionerId: q.ProvisionerId,
		WorkerGroup:   q.WorkerGroup,
	}

	err := <-UpdateTaskStatus(update, q.client, q.Log)
	if err != nil {
		if err.StatusCode != 401 || err.StatusCode != 403 || err.StatusCode < 500 {
			_ = q.deleteFromAzure(task.TaskId, task.SignedDeleteUrl)
		}
		return false
	}
	_ = q.deleteFromAzure(task.TaskId, task.SignedDeleteUrl)
	return true
}

// deleteFromAzure will attempt to delete a task from the Azure queue and
// return an error in case of failure
func (q queueService) deleteFromAzure(taskId string, deleteUrl string) error {
	// Messages are deleted from the Azure queue with a DELETE request to the
	// signedDeleteUrl from the Azure queue object returned from
	// queue.pollTaskUrls.

	// Also remark that the worker must delete messages if the queue.claimTask
	// operations fails with a 4xx error. A 400 hundred range error implies
	// that the task wasn't created, not scheduled or already claimed, in
	// either case the worker should delete the message as we don't want
	// another worker to receive message later.

	q.Log.Info("Deleting task from Azure queue")
	httpCall := func() (*http.Response, error, error) {
		req, err := http.NewRequest("DELETE", deleteUrl, nil)
		if err != nil {
			return nil, nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		return resp, err, nil
	}

	_, _, err := httpbackoff.Retry(httpCall)

	// Notice, that failure to delete messages from Azure queue is serious, as
	// it wouldn't manifest itself in an immediate bug. Instead if messages
	// repeatedly fails to be deleted, it would result in a lot of unnecessary
	// calls to the queue and the Azure queue. The worker will likely continue
	// to work, as the messages eventually disappears when their deadline is
	// reached. However, the provisioner would over-provision aggressively as
	// it would be unable to tell the number of pending tasks. And the worker
	// would spend a lot of time attempting to claim faulty messages. For these
	// reasons outlined above it's strongly advised that workers logs failures
	// to delete messages from Azure queues.
	if err != nil {
		q.Log.WithFields(logrus.Fields{
			"error": err,
			"url":   deleteUrl,
		}).Warn("Not able to delete task from azure queue")
		return err
	} else {
		q.Log.Info("Successfully deleted task from azure queue")
	}
	return nil
}

// Retrieves the number of tasks requested from the Azure queues.
func (q *queueService) pollTaskUrl(taskQueue *taskQueue, ntasks int) ([]*TaskRun, error) {
	taskRuns := []*TaskRun{}
	var r queueMessagesList
	// To poll an Azure Queue the worker must do a `GET` request to the
	// `signedPollUrl` from the object, representing the Azure queue. To
	// receive multiple messages at once the parameter `&numofmessages=N`
	// may be appended to `signedPollUrl`. The parameter `N` is the
	// maximum number of messages desired, `N` can be up to 32.
	// Since we can only process one task at a time, grab only one.
	resp, _, err := httpbackoff.Get(fmt.Sprintf("%s%s%d", taskQueue.SignedPollUrl, "&numofmessages=", int(math.Min(32, float64(ntasks)))))
	if err != nil {
		return nil, err
	}
	// When executing a `GET` request to `signedPollUrl` from an Azure queue object,
	// the request will return an XML document on the form:
	//
	// ```xml
	// <QueueMessagesList>
	//     <QueueMessage>
	//       <MessageId>...</MessageId>
	//       <InsertionTime>...</InsertionTime>
	//       <ExpirationTime>...</ExpirationTime>
	//       <PopReceipt>...</PopReceipt>
	//       <TimeNextVisible>...</TimeNextVisible>
	//       <DequeueCount>...</DequeueCount>
	//       <MessageText>...</MessageText>
	//     </QueueMessage>
	//     ...
	// </QueueMessagesList>
	// ```
	// We unmarshal the response into go objects, using the go xml decoder.
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err := xml.Unmarshal(data, &r); err != nil {
		//if err := xml.NewDecoder(resp.Body).Decode(&queueMessagesList); err != nil {
		q.Log.WithFields(logrus.Fields{
			"body":  resp.Body,
			"error": err.Error(),
		}).Debugf("Not able to xml decode the response from the Azure queue")
		return nil, errors.New(
			fmt.Sprintf("Not able to xml decode the response from the Azure queue. Body: %s, Error: %s", resp.Body, err),
		)
	}

	if len(r.QueueMessages) == 0 {
		q.Log.Debug("Zero tasks returned in Azure XML queueMessagesList")
		return []*TaskRun{}, nil
	}

	// Utility method for replacing a placeholder within a uri with
	// a string value which first must be uri encoded...
	detokeniseUri := func(uri, placeholder, rawValue string) string {
		return strings.Replace(uri, placeholder, strings.Replace(url.QueryEscape(rawValue), "+", "%20", -1), -1)
	}

	for _, qm := range r.QueueMessages {

		// Before using the signedDeleteUrl the worker must replace the placeholder
		// {{messageId}} with the contents of the <MessageId> tag. It is also
		// necessary to replace the placeholder {{popReceipt}} with the URI encoded
		// contents of the <PopReceipt> tag.  Notice, that the worker must URI
		// encode the contents of <PopReceipt> before substituting into the
		// signedDeleteUrl. Otherwise, the worker will experience intermittent
		// failures.

		signedDeleteUrl := detokeniseUri(
			detokeniseUri(
				taskQueue.SignedDeleteUrl,
				"{{messageId}}",
				qm.MessageId,
			),
			"{{popReceipt}}",
			qm.PopReceipt,
		)

		// Workers should read the value of the `<DequeueCount>` and log messages
		// that alert the operator if a message has been dequeued a significant
		// number of times, for example 15 or more.
		if qm.DequeueCount >= 15 {
			q.Log.Warnf("Queue Message with message id %v has been dequeued %v times!", qm.MessageId, qm.DequeueCount)
			err := q.deleteFromAzure("", signedDeleteUrl)
			if err != nil {
				q.Log.Warnf("Not able to call Azure delete URL %v. %v", signedDeleteUrl, err)
			}
		}

		// To find the task referenced in a message the worker must base64
		// decode and JSON parse the contents of the <MessageText> tag. This
		// would return an object on the form: {taskId, runId}.
		m, err := base64.StdEncoding.DecodeString(qm.MessageText)
		if err != nil {
			// try to delete from Azure, if it fails, nothing we can do about it
			// not very serious - another worker will try to delete it
			q.Log.WithField("messageText", qm.MessageText).Errorf("Not able to base64 decode the Message Text in Azure message response.")
			q.Log.WithField("messageId", qm.MessageId).Info("Deleting from Azure queue as other workers will have the same problem.")
			err := q.deleteFromAzure("", signedDeleteUrl)
			if err != nil {
				q.Log.WithFields(logrus.Fields{
					"messageId": qm.MessageId,
					"url":       signedDeleteUrl,
					"error":     err,
				}).Warn("Not able to call Azure delete URL")
			}
			return nil, err
		}

		// initialise fields of TaskRun not contained in json string m
		taskRun := &TaskRun{
			SignedDeleteUrl: signedDeleteUrl,
		}

		// now populate remaining json fields of TaskRun from json string m
		err = json.Unmarshal(m, &taskRun)
		if err != nil {
			q.Log.WithFields(logrus.Fields{
				"error":   err,
				"message": m,
			}).Warn("Not able to unmarshal json from base64 decoded MessageText")
			err := q.deleteFromAzure("", signedDeleteUrl)
			if err != nil {
				q.Log.WithFields(logrus.Fields{
					"url":   signedDeleteUrl,
					"error": err,
				}).Warn("Not able to call Azure delete URL")
			}
			continue
		}
		taskRuns = append(taskRuns, taskRun)
	}

	return taskRuns, nil
}

// Refreshes a list of task queue urls.  Each task queue contains a pair of signed urls
// used for polling and deleting messages.
func (q *queueService) refreshTaskQueueUrls() error {
	// Attempt to wait until expiration gets closer before refreshing.  No
	// need to do it more frequently.
	if !q.shouldRefreshQueueUrls() {
		return nil
	}

	q.Log.Debug("Refreshing Azure queue task urls")
	signedURLs, _, err := q.client.PollTaskUrls(q.ProvisionerId, q.WorkerType)
	if err != nil {
		q.Log.WithField("error", err).Warn("Error retrieving task urls.")
		return errors.New("Error retrieving task urls.")
	}

	taskQueues := []taskQueue{}
	for _, pair := range signedURLs.Queues {
		taskQueues = append(taskQueues, taskQueue(pair))
	}

	q.mu.Lock()
	q.queues = taskQueues
	q.Expires = signedURLs.Expires
	q.mu.Unlock()
	q.Log.Debugf("Refreshed %d Azure queue task urls", len(taskQueues))
	return nil
}

func (q queueService) retrieveTasksFromQueue(ntasks int) ([]*TaskRun, error) {
	err := q.refreshTaskQueueUrls()
	if err != nil {
		return nil, err
	}

	tasks := []*TaskRun{}
	for _, queue := range q.queues {
		// It hopefully would never be greater, but just incase, we would want to return
		// and run what tasks we do have.
		if len(tasks) >= ntasks {
			return tasks, nil
		}
		taskRuns, err := q.pollTaskUrl(&queue, ntasks-len(tasks))
		if err != nil {
			q.Log.Warnf("Could not retrieve tasks from the Azure queue. %s", err)
			continue
		}
		tasks = append(tasks, taskRuns...)

	}
	return tasks, nil
}

// Evaluate if the current time is getting close to the url expiration as decided
// by the ExpirationOffset.
func (q queueService) shouldRefreshQueueUrls() bool {
	if len(q.queues) == 0 {
		return true
	}
	// If the duration between Expiration and current time is less than the expiration
	// off set then it's time to refresh the urls
	if int(time.Time(q.Expires).Sub(time.Now()).Seconds()) < q.ExpirationOffset {
		return true
	}
	return false
}
