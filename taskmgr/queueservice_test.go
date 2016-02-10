package taskmgr

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
	"github.com/taskcluster/slugid-go/slugid"
	"github.com/taskcluster/taskcluster-client-go/queue"
	"github.com/taskcluster/taskcluster-client-go/tcclient"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

const PROVISIONER_ID = "dummy-provisioner"

var WORKER_TYPE = (fmt.Sprintf("dummy-type-%s", slugid.Nice()))[0:22]

func TestRetrievePollTaskUrls(t *testing.T) {
	logger, _ := runtime.CreateLogger("")
	mockedQueue := &MockQueue{}
	service := queueService{
		client:           mockedQueue,
		ProvisionerId:    PROVISIONER_ID,
		WorkerType:       WORKER_TYPE,
		Log:              logger.WithField("component", "Queue Service"),
		ExpirationOffset: 300,
	}
	mockedQueue.On(
		"PollTaskUrls",
		PROVISIONER_ID,
		WORKER_TYPE,
	).Return(&queue.PollTaskUrlsResponse{
		Expires: tcclient.Time(time.Now().Add(time.Minute * 10)),
		Queues: []struct {
			SignedDeleteUrl string `json:"signedDeleteUrl"`
			SignedPollUrl   string `json:"signedPollUrl"`
		}{{
			// Urls are arbitrary and unique so they can be checked later on.
			// Polling should return at least 2 queues in production because of
			// high/low priority queues
			SignedDeleteUrl: "abc",
			SignedPollUrl:   "123",
		}, {
			SignedDeleteUrl: "def",
			SignedPollUrl:   "456",
		}},
	}, &tcclient.CallSummary{}, nil)
	service.refreshTaskQueueUrls()
	assert.Equal(t,
		len(service.queues),
		2,
		fmt.Sprintf("Queue Service should contain two sets of url pairs but got %d", len(service.queues)),
	)

	assert.Equal(t, "abc", service.queues[0].SignedDeleteUrl)
	assert.Equal(t, "123", service.queues[0].SignedPollUrl)
	assert.Equal(t, "def", service.queues[1].SignedDeleteUrl)
	assert.Equal(t, "456", service.queues[1].SignedPollUrl)
}

func TestRetrievePollTaskUrlsErrorCaught(t *testing.T) {
	logger, _ := runtime.CreateLogger("")
	mockedQueue := &MockQueue{}
	service := queueService{
		client:           mockedQueue,
		ProvisionerId:    PROVISIONER_ID,
		WorkerType:       WORKER_TYPE,
		Log:              logger.WithField("component", "Queue Service"),
		ExpirationOffset: 300,
	}

	mockedQueue.On(
		"PollTaskUrls",
		PROVISIONER_ID,
		WORKER_TYPE,
	// Error value does not matter, just as long as we create an error to return
	).Return(&queue.PollTaskUrlsResponse{}, &tcclient.CallSummary{}, errors.New("bad error"))

	err := service.refreshTaskQueueUrls()
	if err == nil {
		t.Fatal("Error should have been returned when polling failed")
	}

	assert.Equal(t, "Error retrieving task urls.", err.Error())
}

func TestShouldRefreshQueueUrls(t *testing.T) {
	service := queueService{
		ExpirationOffset: 300,
	}

	// Should refresh since there are no queues currently stored
	assert.Equal(t, true, service.shouldRefreshQueueUrls())

	// When expiration is still within limits, and the queue slice has already been
	// populated, the queue service should not need to refresh
	service.Expires = tcclient.Time(time.Now().Add(time.Minute * 6))
	service.queues = []taskQueue{taskQueue{}, taskQueue{}}
	assert.Equal(t, false, service.shouldRefreshQueueUrls())

	// Expiration is coming close, need to refresh
	service.Expires = tcclient.Time(time.Now().Add(time.Minute * 2))
	assert.Equal(t, true, service.shouldRefreshQueueUrls())
}

func TestShouldNotRefreshTaskQueueUrls(t *testing.T) {
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		ExpirationOffset: 300,
		Expires:          tcclient.Time(time.Now().Add(time.Minute * 10)),
		queues:           []taskQueue{taskQueue{}, taskQueue{}},
		Log:              logger.WithField("component", "Queue Service"),
	}

	// Because the expiration is not close, adn the service already has queues,
	// there should be no reason to refresh.  Because the service was not created
	// with a taskcluster queue client, if it attempts to refresh, there will be
	// a panic
	err := service.refreshTaskQueueUrls()
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
		Log: logger.WithField("component", "Queue Service"),
		queues: []taskQueue{{
			SignedDeleteUrl: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollUrl:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	_, err := service.pollTaskUrl(&service.queues[0], 3)
	assert.NotNil(t, err, "Error should have been returned when invalid xml was parsed")
	assert.Contains(t, err.Error(), "Not able to xml decode the response from the Azure queue")
}

func TestPollTaskUrlEmptyMessageList(t *testing.T) {
	var handler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, "<QueueMessagesList></QueueMessagesList>")
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		Log: logger.WithField("component", "Queue Service"),
		queues: []taskQueue{{
			SignedDeleteUrl: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollUrl:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	tasks, err := service.pollTaskUrl(&service.queues[0], 3)
	assert.Nil(t, err, "Error should not have been returned when empty message list provided.")
	assert.Equal(t, 0, len(tasks))
}

func TestPollTaskUrlNonEmptyMessageList(t *testing.T) {
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
		Log: logger.WithField("component", "Queue Service"),
		queues: []taskQueue{{
			SignedDeleteUrl: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollUrl:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	tasks, err := service.pollTaskUrl(&service.queues[0], 3)
	assert.Nil(t, err, "Error should not have been returned when empty message list provided.")
	assert.Equal(t, 2, len(tasks))
	// quick sanity check to make sure the messages are different
	assert.NotEqual(t, tasks[0].TaskId, tasks[1].TaskId)
	assert.NotEqual(t, tasks[0].SignedDeleteUrl, tasks[1].SignedDeleteUrl)
}

func TestPollTaskUrlInvalidMessageText(t *testing.T) {
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
		Log: logger.WithField("component", "Queue Service"),
		queues: []taskQueue{{
			SignedDeleteUrl: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
			SignedPollUrl:   fmt.Sprintf("%s/tasks", s.URL),
		}},
	}

	tasks, err := service.pollTaskUrl(&service.queues[0], 3)
	assert.Nil(t, err, "Error should not have been raised when unmarshalling invalid MessageText")
	assert.Equal(t, 0, len(tasks))
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
		Log: logger.WithField("component", "Queue Service"),
	}
	err := service.deleteFromAzure("1234", fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL))
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
		Log: logger.WithField("component", "Queue Service"),
	}
	err := service.deleteFromAzure("1234", fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL))
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "(Permanent) HTTP response code 403")
}
func TestRetrieveTasksFromQueue(t *testing.T) {
	messages := []string{
		// MessageText is {"taskId": "abc", "runId": 1}
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
		// MessageText[0] {"taskId": "def", "runId": 0}
		// MessageText[1] {"taskId": "ghi", "runId": 2}
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
		} else {
			message = messages[1]
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(message))
	}

	s := httptest.NewServer(http.HandlerFunc(handler))
	defer s.Close()
	logger, _ := runtime.CreateLogger(os.Getenv("LOGGING_LEVEL"))
	service := queueService{
		Log:              logger.WithField("component", "Queue Service"),
		ExpirationOffset: 300,
		Expires:          tcclient.Time(time.Now().Add(time.Minute * 10)),
		queues: []taskQueue{
			{
				SignedDeleteUrl: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
				SignedPollUrl:   fmt.Sprintf("%s/tasks/1234?messages=true", s.URL),
			},
			{
				SignedDeleteUrl: fmt.Sprintf("%s/delete/{{messageId}}/{{popReceipt}}", s.URL),
				SignedPollUrl:   fmt.Sprintf("%s/tasks/456?messages=true", s.URL),
			},
		},
	}

	tasks, err := service.retrieveTasksFromQueue(5)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, len(tasks))
	taskIds := []string{"abc", "def", "ghi"}
	for i, v := range taskIds {
		assert.Equal(t, v, tasks[i].TaskId)
	}
}
