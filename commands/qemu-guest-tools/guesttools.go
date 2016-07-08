package qemuguesttools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"gopkg.in/djherbis/buffer.v1"
	"gopkg.in/djherbis/nio.v2"

	"github.com/Sirupsen/logrus"
	"github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
)

type guestTools struct {
	baseURL string
	got     *got.Got
	log     *logrus.Entry
	taskLog io.Writer
}

var backOff = &got.BackOff{
	DelayFactor:         100 * time.Millisecond,
	RandomizationFactor: 0.25,
	MaxDelay:            5 * time.Second,
}

func new(host string, log *logrus.Entry) *guestTools {
	got := got.New()
	got.Client = &http.Client{Timeout: 5 * time.Second}
	got.MaxSize = 10 * 1024 * 1024
	got.Log = log.WithField("guest-tools", "http-got")
	got.Retries = 15
	got.BackOff = backOff

	return &guestTools{
		baseURL: "http://" + host + "/",
		got:     got,
		log:     log,
	}
}

func (g *guestTools) Run() {
	// Start reverse-request to execute interactive requests
	go g.startInteractiveRequests()

	// Poll for instructions on how to execute the current task
	task := metaservice.Execute{}
	for {
		res, err := g.got.Get(g.baseURL + "engine/v1/execute").Send()
		if err != nil {
			g.log.Println("Failed to GET /engine/v1/execute, error: ", err)
			goto retry
		}
		err = json.Unmarshal(res.Body, &task)
		if err != nil {
			g.log.Println("Failed to parse JSON form /engine/v1/execute, error: ", err)
			goto retry
		}
		g.log.Printf("Received task: %+v\n", task)
		break
	retry:
		time.Sleep(200 * time.Millisecond)
	}

	// Start sending task log
	taskLog := g.createTaskLog()

	// Construct environment variables in golang format
	env := os.Environ()
	for key, value := range task.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Execute the task
	cmd := exec.Command(task.Command[0], task.Command[1:]...)
	cmd.Env = env
	cmd.Stdout = taskLog
	cmd.Stderr = taskLog

	result := "success"
	if cmd.Run() != nil {
		result = "failed"
	}

	// Close/flush the task log
	err := taskLog.Close()
	if err != nil {
		g.log.Println("Failed to flush the log, error: ", err)
		result = "failed"
	}

	res, err := g.got.Put(g.baseURL+"engine/v1/"+result, nil).Send()
	if err != nil {
		g.log.Println("Failed to report result ", result, ", error: ", err)
	} else if res.StatusCode != http.StatusOK {
		g.log.Println("Failed to report result ", result, ", status: ", res.StatusCode)
	}
}

func (g *guestTools) createTaskLog() io.WriteCloser {
	reader, writer := nio.Pipe(buffer.New(5))
	req, err := http.NewRequest("POST", g.baseURL+"engine/v1/log", reader)
	if err != nil {
		g.log.Panic("Failed to create request for log, error: ", err)
	}

	go func() {
		client := http.Client{Timeout: 0}
		res, err := client.Do(req)

		if err != nil {
			g.log.Println("Failed to send log, error: ", err)
		} else if res.StatusCode != http.StatusOK {
			g.log.Println("Failed to send log, status: ", res.StatusCode)
		}
	}()

	return writer
}

func (g *guestTools) startInteractiveRequests() {
	//TODO: Implement something here that calls the meta-data service and
	//      does long-polling to see if there is an operation to undertake.
	//			This is things like an artifact to export, or a bash command to run.
}
