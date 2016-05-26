package worker

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
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

type (
	// queueMessagesList is the unmarshalled response from an Azure message queue request.
	queueMessagesList struct {
		XMLName       xml.Name       `xml:"QueueMessagesList"`
		QueueMessages []queueMessage `xml:"QueueMessage"`
	}

	// queueMessage represents part of the message response from an
	// Azure message queue request
	queueMessage struct {
		MessageID    string `xml:"MessageId"`
		PopReceipt   string `xml:"PopReceipt"`
		DequeueCount int    `xml:"DequeueCount"`
		MessageText  string `xml:"MessageText"`
	}

	// messageQueue represents a queue containing a pair of signed
	// delete and polling urls.  A given worker type will have 1 or more messageQueues
	// which should be polled in order for tasks.
	messageQueue struct {
		SignedDeleteURL string `json:"signedDeleteUrl"`
		SignedPollURL   string `json:"signedPollUrl"`
	}

	taskMessage struct {
		TaskID          string `json:"taskId"`
		RunID           int    `json:"runId"`
		signedDeleteURL string
	}

	// QueueService is an interface describing the methods responsible for claiming
	// work from Azure queues.
	QueueService interface {
		Done()
		Start() <-chan *taskClaim
		Stop()
	}

	queueService struct {
		mu               sync.RWMutex
		capacity         int
		interval         int
		tc               chan *taskClaim
		queues           []messageQueue
		expires          tcclient.Time
		expirationOffset int
		client           client.Queue
		provisionerID    string
		workerType       string
		workerID         string
		workerGroup      string
		log              *logrus.Entry
		halt             atomics.Bool
	}
)

// Start will begin the task claiming loop and claim as many tasks as the worker
// capacity allows.  Claimed tasks will be returned on a channel for consumers
// to run.
func (q *queueService) Start() <-chan *taskClaim {
	q.tc = make(chan *taskClaim)

	go func() {
		for !q.halt.Get() {
			q.mu.RLock()
			capacity := q.capacity
			q.mu.RUnlock()
			tasks := q.retrieveTasksFromQueue(capacity)
			q.claimTasks(tasks)
			time.Sleep(time.Duration(q.interval) * time.Second)
		}
		close(q.tc)
	}()
	return q.tc
}

// Stop will set the current capacity to 0 so no tasks are claimed.
func (q *queueService) Stop() {
	q.halt.Set(true)
	return
}

// Done is called each time a task is completed.  Current capacity will be incremented
// each time Done is called.
func (q *queueService) Done() {
	q.mu.Lock()
	q.capacity++
	q.mu.Unlock()
}

func (q *queueService) claimTasks(tasks []*taskMessage) {
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go func(task *taskMessage) {
			defer wg.Done()
			claim, err := q.claimTask(task)
			if err != nil {
				q.log.WithFields(logrus.Fields{
					"taskID": task.TaskID,
					"runID":  task.RunID,
					"error":  err.Error(),
				}).Warn("Could not claim task")
				return
			}
			q.mu.Lock()
			q.capacity--
			q.mu.Unlock()
			q.tc <- claim
		}(task)
	}
	wg.Wait()
	return
}

func (q *queueService) claimTask(task *taskMessage) (*taskClaim, error) {
	claim, err := claimTask(q.client, task.TaskID, task.RunID, q.workerID, q.workerGroup, q.log)
	if err != nil {
		switch err := err.(type) {
		case *updateError:
			if err.statusCode == 401 || err.statusCode == 403 || err.statusCode >= 500 {
				// Do not delete the message if task could not be claimed because of server
				// or authorization failures
				return nil, errors.New("Error when attempting to claim task.  Task was *not* deleted from Azure.")
			}
		}

		_ = q.deleteFromAzure(task.signedDeleteURL)
		return nil, fmt.Errorf("Error when attempting to claim task %v. Error is non-recoverable so task was (hopefully) deleted from Azure. Cause: %v", task.TaskID, err)
	}
	_ = q.deleteFromAzure(task.signedDeleteURL)
	return claim, nil
}

// deleteFromAzure will attempt to delete a task from the Azure queue and
// return an error in case of failure
func (q *queueService) deleteFromAzure(deleteURL string) error {
	// Messages are deleted from the Azure queue with a DELETE request to the
	// SignedDeleteURL from the Azure queue object returned from
	// queue.pollTaskURLs.

	// Also remark that the worker must delete messages if the queue.claimTask
	// operations fails with a 4xx error. A 400 hundred range error implies
	// that the task wasn't created, not scheduled or already claimed, in
	// either case the worker should delete the message as we don't want
	// another worker to receive message later.

	q.log.Info("Deleting task from Azure queue")
	httpCall := func() (*http.Response, error, error) {
		req, err := http.NewRequest("DELETE", deleteURL, nil)
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
		q.log.WithFields(logrus.Fields{
			"error": err,
			"url":   deleteURL,
		}).Warn("Not able to delete task from azure queue")
		return err
	}

	q.log.Info("Successfully deleted task from azure queue")
	return nil
}

// Retrieves the number of tasks requested from the Azure queues.
func (q *queueService) pollTaskURL(messageQueue *messageQueue, ntasks int) ([]*taskMessage, error) {
	taskMessages := []*taskMessage{}
	var r queueMessagesList
	// To poll an Azure Queue the worker must do a `GET` request to the
	// `SignedPollURL` from the object, representing the Azure queue. To
	// receive multiple messages at once the parameter `&numofmessages=N`
	// may be appended to `SignedPollURL`. The parameter `N` is the
	// maximum number of messages desired, `N` can be up to 32.
	n := int(math.Min(32, float64(ntasks)))
	u := fmt.Sprintf("%s%s%d", messageQueue.SignedPollURL, "&numofmessages=", n)
	resp, _, err := httpbackoff.Get(u)
	if err != nil {
		return nil, err
	}
	// When executing a `GET` request to `SignedPollURL` from an Azure queue object,
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
		q.log.WithFields(logrus.Fields{
			"body":  resp.Body,
			"error": err.Error(),
		}).Debugf("Not able to xml decode the response from the Azure queue")
		return nil, fmt.Errorf("Not able to xml decode the response from the Azure queue. Body: %s, Error: %s", resp.Body, err)
	}

	if len(r.QueueMessages) == 0 {
		q.log.Debug("Zero tasks returned in Azure XML queueMessagesList")
		return []*taskMessage{}, nil
	}

	// Utility method for replacing a placeholder within a uri with
	// a string value which first must be uri encoded...
	detokeniseURI := func(URI, placeholder, rawValue string) string {
		return strings.Replace(URI, placeholder, strings.Replace(url.QueryEscape(rawValue), "+", "%20", -1), -1)
	}

	for _, qm := range r.QueueMessages {

		// Before using the SignedDeleteURL the worker must replace the placeholder
		// {{messageId}} with the contents of the <MessageId> tag. It is also
		// necessary to replace the placeholder {{popReceipt}} with the URI encoded
		// contents of the <PopReceipt> tag.  Notice, that the worker must URI
		// encode the contents of <PopReceipt> before substituting into the
		// SignedDeleteURL. Otherwise, the worker will experience intermittent
		// failures.

		signedDeleteURL := detokeniseURI(
			detokeniseURI(
				messageQueue.SignedDeleteURL,
				"{{messageId}}",
				qm.MessageID,
			),
			"{{popReceipt}}",
			qm.PopReceipt,
		)

		// Workers should read the value of the `<DequeueCount>` and log messages
		// that alert the operator if a message has been dequeued a significant
		// number of times, for example 15 or more.
		if qm.DequeueCount >= 15 {
			q.log.Warnf("Queue Message with message id %v has been dequeued %v times!", qm.MessageID, qm.DequeueCount)
			err := q.deleteFromAzure(signedDeleteURL)
			if err != nil {
				q.log.Warnf("Not able to call Azure delete URL %v. %v", signedDeleteURL, err)
			}
		}

		// To find the task referenced in a message the worker must base64
		// decode and JSON parse the contents of the <MessageText> tag. This
		// would return an object on the form: {taskId, runId}.
		m, err := base64.StdEncoding.DecodeString(qm.MessageText)
		if err != nil {
			// try to delete from Azure, if it fails, nothing we can do about it
			// not very serious - another worker will try to delete it
			q.log.WithField("messageText", qm.MessageText).Errorf("Not able to base64 decode the Message Text in Azure message response.")
			q.log.WithField("messageID", qm.MessageID).Info("Deleting from Azure queue as other workers will have the same problem.")
			err := q.deleteFromAzure(signedDeleteURL)
			if err != nil {
				q.log.WithFields(logrus.Fields{
					"messageID": qm.MessageID,
					"url":       signedDeleteURL,
					"error":     err,
				}).Warn("Not able to call Azure delete URL")
			}
			return nil, err
		}

		// initialise fields of TaskRun not contained in json string m
		tm := &taskMessage{
			signedDeleteURL: signedDeleteURL,
		}

		// now populate remaining json fields of TaskMessage from json string m
		err = json.Unmarshal(m, &tm)
		if err != nil {
			q.log.WithFields(logrus.Fields{
				"error":   err,
				"message": m,
			}).Warn("Not able to unmarshal json from base64 decoded MessageText")
			err := q.deleteFromAzure(signedDeleteURL)
			if err != nil {
				q.log.WithFields(logrus.Fields{
					"url":   signedDeleteURL,
					"error": err,
				}).Warn("Not able to call Azure delete URL")
			}
			continue
		}
		taskMessages = append(taskMessages, tm)
	}

	return taskMessages, nil
}

// Refreshes a list of task queue urls.  Each task queue contains a pair of signed urls
// used for polling and deleting messages.
func (q *queueService) refreshMessageQueueURLs() error {
	// Attempt to wait until expiration gets closer before refreshing.  No
	// need to do it more frequently.
	if !q.shouldRefreshQueueUrls() {
		return nil
	}

	q.log.Debug("Refreshing Azure message queue urls")

	signedURLs, err := q.client.PollTaskUrls(q.provisionerID, q.workerType)
	if err != nil {
		q.log.WithField("error", err).Warn("Error retrieving message queue urls.")
		return errors.New("Error retrieving message queue urls.")
	}

	messageQueues := []messageQueue{}
	for _, pair := range signedURLs.Queues {
		messageQueues = append(messageQueues, messageQueue(pair))
	}

	q.mu.Lock()
	q.queues = messageQueues
	q.expires = signedURLs.Expires
	q.mu.Unlock()
	q.log.Debugf("Refreshed %d Azure queue task urls", len(messageQueues))
	return nil
}

func (q *queueService) retrieveTasksFromQueue(ntasks int) []*taskMessage {
	_ = q.refreshMessageQueueURLs()
	tasks := []*taskMessage{}

	for _, queue := range q.queues {
		// Continue polling the Azure queue until either enough messages have been retrieved
		// or the queue has no more messages.
		for {
			// It hopefully would never be greater, but just incase, we would want to return
			// and run what tasks we do have.
			if len(tasks) >= ntasks {
				return tasks
			}
			messages, err := q.pollTaskURL(&queue, ntasks-len(tasks))
			if err != nil {
				q.log.Warnf("Could not retrieve tasks from the Azure queue. %s", err)
				break
			}

			if len(messages) == 0 {
				break
			}

			tasks = append(tasks, messages...)
		}

	}
	return tasks
}

// Evaluate if the current time is getting close to the url expiration as decided
// by the ExpirationOffset.
func (q *queueService) shouldRefreshQueueUrls() bool {
	if len(q.queues) == 0 {
		return true
	}
	// If the duration between Expiration and current time is less than the expiration
	// off set then it's time to refresh the urls
	if int(time.Time(q.expires).Sub(time.Now()).Seconds()) < q.expirationOffset {
		return true
	}
	return false
}
