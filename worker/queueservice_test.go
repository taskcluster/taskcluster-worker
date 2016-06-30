package worker

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/taskcluster/httpbackoff"
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

const ProvisionerID = "dummy-provisioner"

var WorkerType = (fmt.Sprintf("dummy-type-%s", slugid.Nice()))[0:22]
var WorkerID = fmt.Sprintf("dummy-worker-%s", slugid.Nice())

func TestRetrievePollTaskUrls(t *testing.T) {
	logger, _ := runtime.CreateLogger("")
	mockedQueue := &client.MockQueue{}
	service := queueService{
		client:           mockedQueue,
		provisionerID:    ProvisionerID,
		workerType:       WorkerType,
		log:              logger.WithField("component", "Queue Service"),
		expirationOffset: 300,
	}
	mockedQueue.On(
		"PollTaskUrls",
		ProvisionerID,
		WorkerType,
	).Return(&queue.PollTaskUrlsResponse{
		Expires: tcclient.Time(time.Now().Add(time.Minute * 10)),
		Queues: []struct {
			SignedDeleteURL string `json:"signedDeleteUrl"`
			SignedPollURL   string `json:"signedPollUrl"`
		}{{
			// Urls are arbitrary and unique so they can be checked later on.
			// Polling should return at least 2 queues in production because of
			// high/low priority queues
			SignedDeleteURL: "abc",
			SignedPollURL:   "123",
		}, {
			SignedDeleteURL: "def",
			SignedPollURL:   "456",
		}},
	}, nil)
	service.refreshMessageQueueURLs()
	assert.Equal(t,
		len(service.queues),
		2,
		fmt.Sprintf("Queue Service should contain two sets of url pairs but got %d", len(service.queues)),
	)

	assert.Equal(t, "abc", service.queues[0].SignedDeleteURL)
	assert.Equal(t, "123", service.queues[0].SignedPollURL)
	assert.Equal(t, "def", service.queues[1].SignedDeleteURL)
	assert.Equal(t, "456", service.queues[1].SignedPollURL)
}

func TestRetrievePollTaskUrlsErrorCaught(t *testing.T) {
	logger, _ := runtime.CreateLogger("")
	mockedQueue := &client.MockQueue{}
	service := queueService{
		client:           mockedQueue,
		provisionerID:    ProvisionerID,
		workerType:       WorkerType,
		log:              logger.WithField("component", "Queue Service"),
		expirationOffset: 300,
	}

	mockedQueue.On(
		"PollTaskUrls",
		ProvisionerID,
		WorkerType,
	// Error value does not matter, just as long as we create an error to return
	).Return(&queue.PollTaskUrlsResponse{}, errors.New("bad error"))

	err := service.refreshMessageQueueURLs()
	if err == nil {
		t.Fatal("Error should have been returned when polling failed")
	}

	assert.Equal(t, "Error retrieving message queue urls.", err.Error())
}

func TestShouldRefreshQueueUrls(t *testing.T) {
	service := queueService{
		expirationOffset: 300,
	}

	// Should refresh since there are no queues currently stored
	assert.Equal(t, true, service.shouldRefreshQueueUrls())

	// When expiration is still within limits, and the queue slice has already been
	// populated, the queue service should not need to refresh
	service.expires = tcclient.Time(time.Now().Add(time.Minute * 6))
	service.queues = []messageQueue{messageQueue{}, messageQueue{}}
	assert.Equal(t, false, service.shouldRefreshQueueUrls())

	// Expiration is coming close, need to refresh
	service.expires = tcclient.Time(time.Now().Add(time.Minute * 2))
	assert.Equal(t, true, service.shouldRefreshQueueUrls())
}

func TestShouldNotRefreshMessageQueueURLs(t *testing.T) {
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		expirationOffset: 300,
		expires:          tcclient.Time(time.Now().Add(time.Minute * 10)),
		queues:           []messageQueue{messageQueue{}, messageQueue{}},
		log:              logger.WithField("component", "Queue Service"),
	}

	// Because the expiration is not close, adn the service already has queues,
	// there should be no reason to refresh.  Because the service was not created
	// with a taskcluster queue client, if it attempts to refresh, there will be
	// a panic
	err := service.refreshMessageQueueURLs()
	assert.Nil(t, err, "No error should be returned because the urls should not have been refreshed")
}

func TestPollTaskUrlInvalidXMLResponse(t *testing.T) {
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, "Invalid XML")
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log: logger.WithField("component", "Queue Service"),
		queues: []messageQueue{{
			SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollURL:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	_, err := service.pollTaskURL(&service.queues[0], 3)
	assert.NotNil(t, err, "Error should have been returned when invalid xml was parsed")
	assert.Contains(t, err.Error(), "Not able to xml decode the response from the Azure queue")
}

func TestPollTaskURLEmptyMessageList(t *testing.T) {
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, "<QueueMessagesList></QueueMessagesList>")
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log: logger.WithField("component", "Queue Service"),
		queues: []messageQueue{{
			SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollURL:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	tasks, err := service.pollTaskURL(&service.queues[0], 3)
	assert.Nil(t, err, "Error should not have been returned when empty message list provided.")
	assert.Equal(t, 0, len(tasks))
}

func TestPollTaskURLNonEmptyMessageList(t *testing.T) {
	// Messages below are arbitrary messages to ensure that they can be
	// decoded.
	messages := `<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiYWJjIiwgInJ1bklkIjogMH0=</MessageText>
	  </QueueMessage>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca2</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiZGVmIiwgInJ1bklkIjogMX0=</MessageText>
	  </QueueMessage>
	</QueueMessagesList>`
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(messages))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log: logger.WithField("component", "Queue Service"),
		queues: []messageQueue{{
			SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollURL:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	tasks, err := service.pollTaskURL(&service.queues[0], 3)
	assert.Nil(t, err, "Error should not have been returned when empty message list provided.")
	assert.Equal(t, 2, len(tasks))
	// quick sanity check to make sure the messages are different
	assert.NotEqual(t, tasks[0].TaskID, tasks[1].TaskID)
	assert.NotEqual(t, tasks[0].signedDeleteURL, tasks[1].signedDeleteURL)
}

func TestPollTaskURLInvalidMessageTextContents(t *testing.T) {
	// MessageText is {"abc",0} which is an invalid format when
	// unmarshalling.
	messages := `<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJhYmMiLDB9</MessageText>
	  </QueueMessage>

	</QueueMessagesList>`
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(messages))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log: logger.WithField("component", "Queue Service"),
		queues: []messageQueue{{
			SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollURL:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	tasks, err := service.pollTaskURL(&service.queues[0], 3)
	assert.Nil(t, err, "Error should not have been raised when unmarshalling invalid MessageText")
	assert.Equal(t, 0, len(tasks))
}

func TestPollTaskURLInvalidMessageTextEncoding(t *testing.T) {
	// MessageText is not a valid base64 encoded string
	messages := `<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>invalid MessageText, not base64 encoded!</MessageText>
	  </QueueMessage>

	</QueueMessagesList>`

	// When there is an error decoding, message should be deleted from Azure
	deleteCalled := false

	var handler = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delete/5974b586-0df3-4e2d-ad0c-18e3892bfca3/YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw" {
			deleteCalled = true
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(messages))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log: logger.WithField("component", "Queue Service"),
		queues: []messageQueue{{
			SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollURL:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	tasks, err := service.pollTaskURL(&service.queues[0], 3)
	assert.Nil(t, err, "Error should not have been raised when unmarshalling invalid MessageText")
	assert.Equal(t, 0, len(tasks))
	assert.True(t, deleteCalled, "Delete URL not called after attempting to decode messageText")
}

func TestSuccessfullyDeleteFromAzureQueue(t *testing.T) {
	// The method for deleting from the azure queue just makes sure that when
	// calling a given URL that a 200 status reponse is received.
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log: logger.WithField("component", "Queue Service"),
	}
	err := service.deleteFromAzure(fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL))
	assert.Nil(t, err)
}

func TestErrorCaughtDeleteFromAzureQueueL(t *testing.T) {
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log: logger.WithField("component", "Queue Service"),
	}
	err := service.deleteFromAzure(fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "(Permanent) HTTP response code 403")
}
func TestRetrieveTasksFromQueue(t *testing.T) {
	// Tasks should be retrieved from multiple priority queues until either there are
	// no more tasks to retrieve or the number of requested tasks are fulfilled.
	messages := []string{
		// MessageText is {"taskId": "abc", "RunID": 1}
		`<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiYWJjIiwgInJ1bklkIjogMX0=</MessageText>
	  </QueueMessage>
	</QueueMessagesList>`,
		// MessageText[0] {"taskId": "def", "RunID": 0}
		// MessageText[1] {"taskId": "ghi", "RunID": 2}
		`<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiZGVmIiwgInJ1bklkIjogMH0NCg==</MessageText>
	  </QueueMessage>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiZ2hpIiwgInJ1bklkIjogMH0NCg==</MessageText>
	  </QueueMessage>
	</QueueMessagesList>`,
	}

	var handler = func(w http.ResponseWriter, r *http.Request) {
		var message string
		if r.URL.Path == "/tasks/1234" {
			message = messages[0]
			messages[0] = ""
		} else {
			message = messages[1]
			messages[1] = ""
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(message))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log:              logger.WithField("component", "Queue Service"),
		expirationOffset: 300,
		expires:          tcclient.Time(time.Now().Add(time.Minute * 10)),
		queues: []messageQueue{
			{
				SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
				SignedPollURL:   fmt.Sprintf("%s/tasks/1234?messages=true", s.URL),
			},
			{
				SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
				SignedPollURL:   fmt.Sprintf("%s/tasks/456?messages=true", s.URL),
			},
		},
	}

	tasks := service.retrieveTasksFromQueue(3)

	assert.Equal(t, 3, len(tasks))
	taskIds := []string{"abc", "def", "ghi"}
	for i, v := range taskIds {
		assert.Equal(t, v, tasks[i].TaskID)
	}
}

func TestRetrieveTasksFromQueueDoesNotQueryLowPriority(t *testing.T) {
	// When enough tasks have been retrieved from the higher (first) priority queue,
	// the lower (second) priority queue should not be polled.
	messages := []string{
		// MessageText is {"taskId": "abc", "RunID": 1}
		`<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiYWJjIiwgInJ1bklkIjogMX0=</MessageText>
	  </QueueMessage>
	</QueueMessagesList>`,
		// MessageText[0] {"taskId": "def", "RunID": 0}
		// MessageText[1] {"taskId": "ghi", "RunID": 2}
		`<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiZGVmIiwgInJ1bklkIjogMH0NCg==</MessageText>
	  </QueueMessage>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>1</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiZ2hpIiwgInJ1bklkIjogMH0NCg==</MessageText>
	  </QueueMessage>
	</QueueMessagesList>`,
	}

	var handler = func(w http.ResponseWriter, r *http.Request) {
		var message string
		if r.URL.Path == "/tasks/1234" {
			message = messages[1]
		} else {
			message = messages[0]
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(message))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log:              logger.WithField("component", "Queue Service"),
		expirationOffset: 300,
		expires:          tcclient.Time(time.Now().Add(time.Minute * 10)),
		queues: []messageQueue{
			{
				SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
				SignedPollURL:   fmt.Sprintf("%s/tasks/1234?messages=true", s.URL),
			},
			{
				SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
				SignedPollURL:   fmt.Sprintf("%s/tasks/456?messages=true", s.URL),
			},
		},
	}

	tasks := service.retrieveTasksFromQueue(2)

	// Only two tasks should have been retrieving leaving the third task in the lower
	// priority queue not being retrieved
	assert.Equal(t, 2, len(tasks))
	taskIds := []string{"def", "ghi"}
	for i, v := range taskIds {
		assert.Equal(t, v, tasks[i].TaskID)
	}
}
func TestRetrieveTasksFromQueueDequeueChecked(t *testing.T) {
	// When the dequeue count is above the reshold of 15, the message should be deleted
	// regardless if it's been claimed yet or not.
	message := `<?xml version="1.0" encoding="utf-8"?>
	<QueueMessagesList>
	  <QueueMessage>
		<MessageId>5974b586-0df3-4e2d-ad0c-18e3892bfca3</MessageId>
		<InsertionTime>Fri, 09 Oct 2009 21:04:30 GMT</InsertionTime>
		<ExpirationTime>Fri, 16 Oct 2009 21:04:30 GMT</ExpirationTime>
		<PopReceipt>YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw</PopReceipt>
		<TimeNextVisible>Fri, 09 Oct 2009 23:29:20 GMT</TimeNextVisible>
		<DequeueCount>16</DequeueCount>
		<MessageText>eyJ0YXNrSWQiOiAiYWJjIiwgInJ1bklkIjogMX0=</MessageText>
	  </QueueMessage>
	</QueueMessagesList>`

	// Delete URL should be called when dequeue count above threshold (15)
	deleteCalled := false

	var handler = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delete/5974b586-0df3-4e2d-ad0c-18e3892bfca3/YzQ4Yzg1MDItYTc0Ny00OWNjLTkxYTUtZGM0MDFiZDAwYzEw" {
			deleteCalled = true
			message = ""
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(message))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		log:              logger.WithField("component", "Queue Service"),
		expirationOffset: 300,
		expires:          tcclient.Time(time.Now().Add(time.Minute * 10)),
		queues: []messageQueue{
			{
				SignedDeleteURL: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
				SignedPollURL:   fmt.Sprintf("%s/tasks/1234?messages=true", s.URL),
			},
		},
	}

	tasks := service.retrieveTasksFromQueue(2)

	assert.Equal(t, 1, len(tasks))
	assert.True(t, deleteCalled, "Delete should have been called when dequeue count above threshold")

}

func TestClaimTask(t *testing.T) {
	// Verifies that when claimTask is called in the queue service for a
	// particular task run object, that the task is claimed and deleted from
	// the azure queue.
	deleteCalled := false
	var handler = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delete" {
			deleteCalled = true
		}
		return
	}
	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	mockedQueue := &client.MockQueue{}
	mockedQueue.On(
		"ClaimTask",
		"abc",
		"0",
		&queue.TaskClaimRequest{
			WorkerGroup: WorkerType,
			WorkerID:    WorkerID,
		},
	).Return(&queue.TaskClaimResponse{
		Credentials: struct {
			AccessToken string `json:"accessToken"`
			Certificate string `json:"certificate"`
			ClientID    string `json:"clientId"`
		}{
			AccessToken: "1040824383284384",
			Certificate: "{}",
			ClientID:    "ajafdsfkj23",
		},
		RunID:       0,
		Status:      queue.TaskStatusStructure{},
		TakenUntil:  tcclient.Time{},
		Task:        queue.TaskDefinitionResponse{},
		WorkerGroup: WorkerType,
		WorkerID:    WorkerID,
	}, nil)

	task := &taskMessage{
		TaskID:          "abc",
		RunID:           0,
		signedDeleteURL: fmt.Sprintf("%s/delete", s.URL),
	}

	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		client:        mockedQueue,
		log:           logger.WithField("component", "Queue Service"),
		workerID:      WorkerID,
		workerGroup:   WorkerType,
		provisionerID: ProvisionerID,
	}

	claim, err := service.claimTask(task)

	assert.Nil(t, err)
	assert.True(t, deleteCalled)
	// Do a quick sanity check to make sure the response was correctly stored in
	// the task run object
	assert.Equal(t, "1040824383284384", claim.taskClaim.Credentials.AccessToken)
}

func TestClaimTaskError(t *testing.T) {
	// When a task cannot be claimed because of a 401 authorization error, the message
	// should not be deleted from the queue.

	// Delete should be called if the claim errored because of authorization or ISE
	// issues
	deleteCalled := false
	var handler = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/delete" {
			deleteCalled = true
		}
		return
	}
	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	mockedQueue := &client.MockQueue{}
	mockedQueue.On(
		"ClaimTask",
		"abc",
		"0",
		&queue.TaskClaimRequest{
			WorkerGroup: WorkerType,
			WorkerID:    WorkerID,
		},
	).Return(&queue.TaskClaimResponse{},
		httpbackoff.BadHttpResponseCode{
			HttpResponseCode: 401,
		},
	)
	task := &taskMessage{
		TaskID:          "abc",
		RunID:           0,
		signedDeleteURL: fmt.Sprintf("%s/delete", s.URL),
	}

	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		client:        mockedQueue,
		log:           logger.WithField("component", "Queue Service"),
		workerID:      WorkerID,
		workerGroup:   WorkerType,
		provisionerID: ProvisionerID,
	}

	_, err := service.claimTask(task)

	assert.NotNil(t, err, "Task should not have been claimed")
	// Delete should not have been called because it was an authorization issue
	assert.False(t, deleteCalled, "Message should not have been deleted from the queue.")
}

func TestClaimTasks(t *testing.T) {
	// Given a slice of task objects, claimTasks should claim each of them successfully
	// and return a list of the claimed task runs.
	var handler = func(w http.ResponseWriter, r *http.Request) {
		return
	}
	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()

	mockedQueue := &client.MockQueue{}
	mockedQueue.On(
		"ClaimTask",
		"abc",
		"0",
		&queue.TaskClaimRequest{
			WorkerGroup: WorkerType,
			WorkerID:    WorkerID,
		},
	).Return(&queue.TaskClaimResponse{
		Credentials: struct {
			AccessToken string `json:"accessToken"`
			Certificate string `json:"certificate"`
			ClientID    string `json:"clientId"`
		}{
			AccessToken: "1040824383284384",
			Certificate: "{}",
			ClientID:    "ajafdsfkj23",
		},
		RunID:       0,
		Status:      queue.TaskStatusStructure{},
		TakenUntil:  tcclient.Time{},
		Task:        queue.TaskDefinitionResponse{},
		WorkerGroup: WorkerType,
		WorkerID:    WorkerID,
	}, nil)
	mockedQueue.On(
		"ClaimTask",
		"def",
		"1",
		&queue.TaskClaimRequest{
			WorkerGroup: WorkerType,
			WorkerID:    WorkerID,
		},
	).Return(&queue.TaskClaimResponse{
		Credentials: struct {
			AccessToken string `json:"accessToken"`
			Certificate string `json:"certificate"`
			ClientID    string `json:"clientId"`
		}{
			AccessToken: "234aajsgfaj340",
			Certificate: "{}",
			ClientID:    "asfg089asgf08",
		},
		RunID:       1,
		Status:      queue.TaskStatusStructure{},
		TakenUntil:  tcclient.Time{},
		Task:        queue.TaskDefinitionResponse{},
		WorkerGroup: WorkerType,
		WorkerID:    WorkerID,
	}, nil)
	tasks := []*taskMessage{{
		TaskID:          "abc",
		RunID:           0,
		signedDeleteURL: fmt.Sprintf("%s/delete", s.URL),
	}, {
		TaskID:          "def",
		RunID:           1,
		signedDeleteURL: fmt.Sprintf("%s/delete", s.URL),
	}}

	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		client:        mockedQueue,
		capacity:      2,
		tc:            make(chan *taskClaim, 2),
		log:           logger.WithField("component", "Queue Service"),
		workerID:      WorkerID,
		workerGroup:   WorkerType,
		provisionerID: ProvisionerID,
	}

	claims := []*taskClaim{}
	service.claimTasks(tasks)
loop:
	for len(claims) != 2 {
		select {
		case t := <-service.tc:
			claims = append(claims, t)
		// Tasks should be claimed in less than 1 second, but set up a timer to timeout
		// so tests don't run forever waiting.
		case <-time.NewTimer(1 * time.Second).C:
			break loop
		}
	}

	assert.Equal(t, 2, len(claims), "Not enough task claims received after calling claimTasks")
}
