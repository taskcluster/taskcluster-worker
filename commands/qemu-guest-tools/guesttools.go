package qemuguesttools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/djherbis/buffer.v1"
	"gopkg.in/djherbis/nio.v2"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

type guestTools struct {
	baseURL       string
	got           *got.Got
	log           *logrus.Entry
	taskLog       io.Writer
	pollingCtx    context.Context
	cancelPolling func()
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

	ctx, cancel := context.WithCancel(context.Background())
	return &guestTools{
		baseURL:       "http://" + host + "/",
		got:           got,
		log:           log,
		pollingCtx:    ctx,
		cancelPolling: cancel,
	}
}

func (g *guestTools) url(path string) string {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return g.baseURL + path
}

func (g *guestTools) Run() {
	// Poll for instructions on how to execute the current task
	task := metaservice.Execute{}
	for {
		res, err := g.got.Get(g.url("engine/v1/execute")).Send()
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
	taskLog, logSent := g.CreateTaskLog()

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

	// Wait for the log to be fully uploaded before reporting the result
	<-logSent

	// Report result
	res, err := g.got.Put(g.url("engine/v1/"+result), nil).Send()
	if err != nil {
		g.log.Println("Failed to report result ", result, ", error: ", err)
	} else if res.StatusCode != http.StatusOK {
		g.log.Println("Failed to report result ", result, ", status: ", res.StatusCode)
	}
}

func (g *guestTools) CreateTaskLog() (io.WriteCloser, <-chan struct{}) {
	reader, writer := nio.Pipe(buffer.New(4 * 1024 * 1024))
	req, err := http.NewRequest("POST", g.url("engine/v1/log"), reader)
	if err != nil {
		g.log.Panic("Failed to create request for log, error: ", err)
	}

	done := make(chan struct{})
	go func() {
		client := http.Client{Timeout: 0}
		res, err := client.Do(req)

		if err != nil {
			g.log.Println("Failed to send log, error: ", err)
		} else if res.StatusCode != http.StatusOK {
			g.log.Println("Failed to send log, status: ", res.StatusCode)
		}
		close(done)
	}()

	return writer, done
}

func (g *guestTools) ProcessActions() {
	for g.pollingCtx.Err() == nil {
		g.poll(g.pollingCtx)
	}
}

func (g *guestTools) StopProcessingActions() {
	g.cancelPolling()
}

const pollTimeout = metaservice.PollTimeout + 5*time.Second

func (g *guestTools) poll(ctx context.Context) {
	// Poll the metaservice for an action to perform
	req, err := http.NewRequest(http.MethodGet, g.url("engine/v1/poll"), nil)
	if err != nil {
		g.log.Panicln("Failed to create polling request, error: ", err)
	}

	// Do request with a timeout
	c, _ := context.WithTimeout(ctx, pollTimeout)
	res, err := ctxhttp.Do(c, nil, req)
	//res, res := http.DefaultClient.Do(req)
	if err != nil {
		g.log.Info("Poll request failed, error: ", err)

		// if this wasn't a deadline exceeded error, we'll sleep a second to avoid
		// spinning the CPU while waiting for DHCP to come up.
		if err != context.DeadlineExceeded && err != context.Canceled {
			time.Sleep(1 * time.Second)
		}
		return
	}

	// Read the request body
	defer res.Body.Close()
	data, err := ioext.ReadAtMost(res.Body, 2*1024*1024)
	if err != nil {
		g.log.Error("Failed to read poll request body, error: ", err)
		return
	}

	// Parse the request body
	action := metaservice.Action{}
	err = json.Unmarshal(data, &action)
	if err != nil {
		g.log.Error("Failed to parse poll request body, error: ", err)
		return
	}

	g.dispatchAction(action)
}

func (g *guestTools) dispatchAction(action metaservice.Action) {
	switch action.Type {
	case "none":
		return // Do nothing we have to poll again
	case "get-artifact":
		go g.doGetArtifact(action.ID, action.Path)
	case "list-folder":
		go g.doListFolder(action.ID, action.Path)
	case "exec-shell":
		go g.doExecShell(action.ID)
	default:
		g.log.Error("Unknown action type: ", action.Type, " ID = ", action.ID)
	}
}

func (g *guestTools) doGetArtifact(ID, path string) {
	g.log.Info("Sending artifact: ", path)

	// Construct body as buffered file read, if there is an error it's because
	// the file doesn't exist and we set the body the nil, as we still have to
	// report this in the reply (just with an empty body)
	var body io.Reader
	f, err := os.Open(path)
	if err == nil {
		body = bufio.NewReader(f)
	}

	// Create reply
	req, err := http.NewRequest(http.MethodPost, g.url("engine/v1/reply?id="+ID), body)
	if err != nil {
		g.log.Panic("Failed to create reply request, error: ", err)
	}
	// If body is nil, the file is missing
	if body == nil {
		req.Header.Set("X-Taskcluster-Worker-Error", "file-not-found")
	}

	// Send the reply
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		g.log.Error("Reply with artifact for path: ", path, " failed error: ", err)
		return
	}
	defer res.Body.Close()
	// Log any errors, we can't really do much
	if res.StatusCode != http.StatusOK {
		g.log.Error("Reply with artifact for path: ", path, " got status: ", res.StatusCode)
	}
}

func (g *guestTools) doListFolder(ID, path string) {
	g.log.Info("Listing path: ", path)

	files := []string{}
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		// Stop iteration and send an error to metaservice, if there is an error
		// with the path we were asked to iterate over.
		if p == path && err != nil {
			return err
		}

		// We ignore errors, directories and anything that isn't plain files
		if info != nil && err == nil && ioext.IsPlainFileInfo(info) {
			files = append(files, p)
		}
		return nil // Ignore other errors
	})
	notFound := err != nil

	// Create reply request... We use got here, this means that we get retries...
	// There is no harm in retries, server will just ignore them.
	req := g.got.Post(g.url("engine/v1/reply?id="+ID), nil)
	err = req.JSON(metaservice.Files{
		Files:    files,
		NotFound: notFound,
	})
	if err != nil {
		g.log.Panic("Failed to serialize JSON payload, error: ", err)
	}

	// Send the reply
	res, err := req.Send()
	if err != nil {
		g.log.Error("Reply with list-folder for path: ", path, " failed error: ", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		g.log.Error("Reply with list-folder for path: ", path, " got status: ", res.StatusCode)
	}
}

var dialer = websocket.Dialer{
	HandshakeTimeout: metaservice.ShellHandshakeTimeout,
	ReadBufferSize:   metaservice.ShellMaxMessageSize,
	WriteBufferSize:  metaservice.ShellMaxMessageSize,
}

func (g *guestTools) doExecShell(ID string) {
	// Establish a websocket reply
	ws, _, err := dialer.Dial("ws:"+g.url("engine/v1/reply?id=" + ID)[5:], nil)
	if err != nil {
		g.log.Error("Failed to establish websocket for reply to ID = ", ID)
		return
	}

	// Create a new shellHandler
	handler := interactive.NewShellHandler(ws, g.log.WithField("shell", ID))

	// Create a shell
	shell := exec.Command("sh")
	shell.Stdin = handler.StdinPipe()
	shell.Stdout = handler.StdoutPipe()
	shell.Stderr = handler.StderrPipe()

	// Start the shell, this must finished before we can call Kill()
	err = shell.Start()

	// Start communication
	handler.Communicate(func() {
		shell.Process.Kill()
	})

	// If starting the shell didn't fail, then we wait for the shell to terminate
	if err == nil {
		err = shell.Wait()
	}
	// If we didn't any error starting or waiting for the shell, then it was a
	// success.
	handler.Terminated(err == nil)
}
