package taskmgr

import (
	"fmt"
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

var (
	WORKER_TYPE = (fmt.Sprintf("dummy-type-%s", slugid.Nice()))[0:22]
	client      = queue.New(
		&tcclient.Credentials{
			ClientId:    os.Getenv("TASKCLUSTER_CLIENT_ID"),
			AccessToken: os.Getenv("TASKCLUSTER_ACCESS_TOKEN"),
			Certificate: os.Getenv("TASKCLUSTER_CERTIFICATE"),
		},
	)
)

// TODO (garndt): Mock out queue endpoints for this type of testing.
func TestRetrievePollTaskUrls(t *testing.T) {
	logger, err := runtime.CreateLogger("")
	if err != nil {
		t.Fatal(err)
	}
	service := queueService{
		client:           client,
		ProvisionerId:    PROVISIONER_ID,
		WorkerType:       WORKER_TYPE,
		Log:              logger.WithField("component", "Queue Service"),
		ExpirationOffset: 300,
	}

	service.refreshTaskQueueUrls()
	assert.Equal(t,
		len(service.queues),
		2,
		fmt.Sprintf("Queue Service should contain two sets of url pairs but got %d", len(service.queues)),
	)
}

func TestShouldRefreshURLCheck(t *testing.T) {
	service := queueService{
		Expires:          tcclient.Time(time.Now().Add(time.Minute * 6)),
		ExpirationOffset: 300,
	}

	shouldRefresh := service.shouldRefreshQueueUrls()
	assert.Equal(t, shouldRefresh, false)

	service.Expires = tcclient.Time(time.Now().Add(time.Minute * 2))
	shouldRefresh = service.shouldRefreshQueueUrls()
	assert.Equal(t, shouldRefresh, true)
}
