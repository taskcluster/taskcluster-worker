package taskmgr

import (
	"errors"
	"fmt"
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

func TestShouldRefreshURLCheck(t *testing.T) {
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
