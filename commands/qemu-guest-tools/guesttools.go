package qemuguesttools

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/taskcluster/go-got"
	"github.com/taskcluster/taskcluster-worker/engines/native/system"
	"github.com/taskcluster/taskcluster-worker/engines/qemu/metaservice"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive"
	"github.com/taskcluster/taskcluster-worker/plugins/interactive/shellconsts"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/atomics"
	"github.com/taskcluster/taskcluster-worker/runtime/ioext"
	buffer "gopkg.in/djherbis/buffer.v1"
	nio "gopkg.in/djherbis/nio.v2"
)

type guestTools struct {
	baseURL       string
	got           *got.Got
	gotpoll       *got.Got
	monitor       runtime.Monitor
	taskLog       io.Writer
	pollingCtx    context.Context
	cancelPolling func()
	killed        atomics.Once
}

var backOff = &got.BackOff{
	DelayFactor:         100 * time.Millisecond,
	RandomizationFactor: 0.25,
	MaxDelay:            5 * time.Second,
}

func new(host string, monitor runtime.Monitor) *guestTools {
	got := got.New()
	got.Client = &http.Client{Timeout: 5 * time.Second}
	got.MaxSize = 10 * 1024 * 1024
	got.Log = monitor.WithTag("guest-tools", "http-got")
	got.Retries = 15
	got.BackOff = backOff

	// Create got client for polling
	gotpoll := *got
	gotpoll.Client = &http.Client{Timeout: pollTimeout + 5*time.Second}

	ctx, cancel := context.WithCancel(context.Background())
	return &guestTools{
		baseURL:       "http://" + host + "/",
		got:           got,
		gotpoll:       &gotpoll,
		monitor:       monitor,
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
			g.monitor.Println("Failed to GET /engine/v1/execute, error: ", err)
			goto retry
		}
		err = json.Unmarshal(res.Body, &task)
		if err != nil {
			g.monitor.Println("Failed to parse JSON form /engine/v1/execute, error: ", err)
			goto retry
		}
		g.monitor.Printf("Received task: %+v\n", task)
		break
	retry:
		time.Sleep(200 * time.Millisecond)
	}

	// Start sending task log
	taskLog, logSent := g.CreateTaskLog()

	// Construct environment variables
	env := make(map[string]string, len(os.Environ())+len(task.Env))
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		env[parts[0]] = parts[1]
	}
	for k, v := range task.Env {
		env[k] = v
	}

	// Execute the task
	proc, err := system.StartProcess(system.ProcessOptions{
		Arguments:   task.Command,
		Environment: env,
		TTY:         true,
		Stdout:      taskLog,
		Stderr:      nil, // implies use of stdout
	})

	result := "failed"
	if err == nil {
		// kill if 'killed' is done
		done := make(chan struct{})
		go func() {
			select {
			case <-g.killed.Done():
				system.KillProcessTree(proc)
			case <-done:
			}
		}()

		if proc.Wait() {
			result = "success"
		}
		close(done)
	}

	// Close/flush the task log
	err = taskLog.Close()
	if err != nil {
		g.monitor.Println("Failed to flush the log, error: ", err)
		result = "failed"
	}

	// Wait for the log to be fully uploaded before reporting the result
	<-logSent

	// Report result
	res, err := g.got.Put(g.url("engine/v1/"+result), nil).Send()
	if err != nil {
		g.monitor.Println("Failed to report result ", result, ", error: ", err)
	} else if res.StatusCode != http.StatusOK {
		g.monitor.Println("Failed to report result ", result, ", status: ", res.StatusCode)
	}
}

func (g *guestTools) CreateTaskLog() (io.WriteCloser, <-chan struct{}) {
	reader, writer := nio.Pipe(buffer.New(4 * 1024 * 1024))
	req, err := http.NewRequest("POST", g.url("engine/v1/log"), reader)
	if err != nil {
		g.monitor.Panic("Failed to create request for log, error: ", err)
	}

	done := make(chan struct{})
	go func() {
		client := http.Client{Timeout: 0}
		res, err := client.Do(req)

		if err != nil {
			g.monitor.Println("Failed to send log, error: ", err)
		} else if res.StatusCode != http.StatusOK {
			g.monitor.Println("Failed to send log, status: ", res.StatusCode)
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
	// Do request with a timeout
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	// Poll the metaservice for an action to perform
	res, err := g.gotpoll.Get(g.url("engine/v1/poll")).WithContext(ctx).Send()
	if err != nil {
		// if this wasn't a deadline exceeded error, we'll sleep a second to avoid
		// spinning the CPU while waiting for DHCP to come up.
		if err != context.DeadlineExceeded && err != context.Canceled {
			g.monitor.Info("Poll request failed, error: ", err)
			time.Sleep(1 * time.Second)
		}
		return
	}

	// Parse the request body
	var action metaservice.Action
	err = json.Unmarshal(res.Body, &action)
	if err != nil {
		g.monitor.Error("Failed to parse poll request body, error: ", err)
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
		go g.doExecShell(action.ID, action.Command, action.TTY)
	case "kill-process":
		go g.doKillProcess(action.ID)
	default:
		g.monitor.Error("Unknown action type: ", action.Type, " ID = ", action.ID)
	}
}

func (g *guestTools) doGetArtifact(ID, path string) {
	g.monitor.Info("Sending artifact: ", path)

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
		g.monitor.Panic("Failed to create reply request, error: ", err)
	}
	// If body is nil, the file is missing
	if body == nil {
		req.Header.Set("X-Taskcluster-Worker-Error", "file-not-found")
	}

	// Send the reply
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		g.monitor.Error("Reply with artifact for path: ", path, " failed error: ", err)
		return
	}
	defer res.Body.Close()
	// Log any errors, we can't really do much
	if res.StatusCode != http.StatusOK {
		g.monitor.Error("Reply with artifact for path: ", path, " got status: ", res.StatusCode)
	}
}

func (g *guestTools) doListFolder(ID, path string) {
	g.monitor.Info("Listing path: ", path)

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
		g.monitor.Panic("Failed to serialize JSON payload, error: ", err)
	}

	// Send the reply
	res, err := req.Send()
	if err != nil {
		g.monitor.Error("Reply with list-folder for path: ", path, " failed error: ", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		g.monitor.Error("Reply with list-folder for path: ", path, " got status: ", res.StatusCode)
	}
}

func (g *guestTools) doKillProcess(ID string) {
	g.monitor.Info("Sending kill-process confirmation")

	// kill child process (from Run method)
	g.killed.Do(nil)

	// Send confirmation... If we got here, this means that we get retries...
	// There is no harm in retries, server will just ignore them.
	res, err := g.got.Post(g.url("engine/v1/reply?id="+ID), nil).Send()
	if err != nil {
		g.monitor.Error("Reply with confirmation for kill-process failed error:", err)
		return
	}
	if res.StatusCode != http.StatusOK {
		g.monitor.Error("Reply with confirmation for kill-process got status:", res.StatusCode)
	}
}

var dialer = websocket.Dialer{
	HandshakeTimeout: shellconsts.ShellHandshakeTimeout,
	ReadBufferSize:   shellconsts.ShellMaxMessageSize,
	WriteBufferSize:  shellconsts.ShellMaxMessageSize,
}

func (g *guestTools) doExecShell(ID string, command []string, tty bool) {
	// Establish a websocket reply
	ws, _, err := dialer.Dial("ws:"+g.url("engine/v1/reply?id=" + ID)[5:], nil)
	if err != nil {
		g.monitor.Error("Failed to establish websocket for reply to ID = ", ID)
		return
	}

	// Create a new shellHandler
	handler := interactive.NewShellHandler(ws, g.monitor.WithTag("shell", ID))

	// Set command to standard shell, if no command is given
	if len(command) == 0 {
		command = []string{"sh"}
		if goruntime.GOOS == "windows" {
			command = []string{"cmd.exe"}
		}
	}

	var stderr io.WriteCloser
	if !tty {
		stderr = handler.StderrPipe()
	}

	// Create a shell
	proc, err := system.StartProcess(system.ProcessOptions{
		Arguments:   command,
		Environment: nil, // TODO: Use env vars from command
		TTY:         tty,
		Stdin:       handler.StdinPipe(),
		Stdout:      handler.StdoutPipe(),
		Stderr:      stderr,
	})

	handler.Communicate(func(cols, rows uint16) error {
		proc.SetSize(cols, rows)
		return nil
	}, func() error {
		return system.KillProcessTree(proc)
	})

	result := false
	if err == nil {
		result = proc.Wait()
	}

	handler.Terminated(result)
}
