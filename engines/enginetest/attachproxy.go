package enginetest

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/taskcluster/taskcluster-worker/engines"
)

// A ProxyTestCase holds information necessary to run tests that an engine
// can attach proxies, call them and forward calls correctly
type ProxyTestCase struct {
	engineProvider
	// Name of engine
	Engine string
	// A valid name for a proxy attachment
	ProxyName string
	// A task.payload as accepted by the engine, which will write "Pinging"
	// to the log, then ping the proxy given by ProxyName with GET to the path
	// "/v1/ping", and write the response to log.
	// The task payload must exit successfully if proxy response code is 200,
	// and unsuccessful if the response code is 404.
	PingProxyPayload string
}

// TestPingProxyPayload checks that PingProxyPayload works as defined
func (c *ProxyTestCase) TestPingProxyPayload() {
	c.ensureEngine(c.Engine)
	ctx, control := c.newTestTaskContext()

	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: ctx,
		Payload:     parseTestPayload(c.engine, c.PingProxyPayload),
	})
	nilOrPanic(err, "Error creating SandboxBuilder")

	pinged := false
	pingMethod := "-"
	pingPath := ""
	err = sandboxBuilder.AttachProxy(c.ProxyName, http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		pinged = true
		pingMethod = r.Method
		pingPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Yay, you managed to ping the end-point, secret=42!!!"))
	}))
	nilOrPanic(err, "Error failed to AttachProxy")

	result := buildRunSandbox(sandboxBuilder)
	nilOrPanic(control.CloseLog(), "Failed to close log")
	reader, err := ctx.NewLogReader()
	nilOrPanic(err, "Failed to open log reader")
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log")
	nilOrPanic(reader.Close(), "Failed to close log reader")
	nilOrPanic(control.Dispose(), "Failed to dispose TaskContext")
	log := string(data)

	if !result {
		fmtPanic("PingProxyPayload exited unsuccessfully, log: ", log)
	}
	if !pinged {
		fmtPanic("PingProxyPayload didn't call the attachedProxy, log: ", log)
	}
	if pingMethod != "GET" && pingMethod != "" {
		fmtPanic("PingProxyPayload pinged with method: ", pingMethod)
	}
	if pingPath != "/v1/ping" {
		fmtPanic("PingProxyPayload pinged path: ", pingPath)
	}

	if !strings.Contains(log, "secret=42") {
		fmtPanic("Didn't find secret=42 from ping response in log", log)
	}
}

// TestPing404IsUnsuccessful checks that 404 returns unsuccessful
func (c *ProxyTestCase) TestPing404IsUnsuccessful() {
	c.ensureEngine(c.Engine)
	ctx, control := c.newTestTaskContext()

	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: ctx,
		Payload:     parseTestPayload(c.engine, c.PingProxyPayload),
	})
	nilOrPanic(err, "Error creating SandboxBuilder")

	pinged := false
	pingPath := ""
	err = sandboxBuilder.AttachProxy(c.ProxyName, http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		pinged = true
		pingPath = r.URL.Path
		w.WriteHeader(404)
		w.Write([]byte("Yay, you managed to ping the end-point, secret=42!!!"))
	}))
	nilOrPanic(err, "Error failed to AttachProxy")

	result := buildRunSandbox(sandboxBuilder)
	nilOrPanic(control.CloseLog(), "Failed to close log")
	reader, err := ctx.NewLogReader()
	nilOrPanic(err, "Failed to open log reader")
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log")
	nilOrPanic(reader.Close(), "Failed to close log reader")
	nilOrPanic(control.Dispose(), "Failed to dispose TaskContext")
	log := string(data)

	if result {
		panic("PingProxyPayload exited successfully, when we returned 404")
	}
	if !pinged {
		panic("PingProxyPayload didn't call the attachedProxy")
	}
	if pingPath != "/v1/ping" {
		fmtPanic("PingProxyPayload pinged path: ", pingPath)
	}

	if !strings.Contains(log, "secret=42") {
		fmtPanic("Didn't find secret=42 from ping response in log", log)
	}
}

// TestLiveLogging checks that "Pinging" is readable from log before the task
// is finished.
func (c *ProxyTestCase) TestLiveLogging() {
	c.ensureEngine(c.Engine)
	ctx, control := c.newTestTaskContext()

	sandboxBuilder, err := c.engine.NewSandboxBuilder(engines.SandboxOptions{
		TaskContext: ctx,
		Payload:     parseTestPayload(c.engine, c.PingProxyPayload),
	})
	nilOrPanic(err, "Error creating SandboxBuilder")

	// Read livelog until we see "Pinging"
	readPinging := make(chan struct{})
	go func() {
		reader, err := ctx.NewLogReader()
		defer evalNilOrPanic(reader.Close, "Failed to close livelog reader")
		nilOrPanic(err, "Failed to open livelog reader")
		buf := bytes.Buffer{}
		for !strings.Contains(string(buf.Bytes()), "Pinging") {
			b := []byte{0}
			n, err := reader.Read(b)
			nilOrPanic(err, "Failed while reading from livelog...")
			if n != 1 {
				panic("Expected one byte to be read!")
			}
			buf.WriteByte(b[0])
		}
		close(readPinging)
	}()

	pinged := false
	pingPath := ""
	err = sandboxBuilder.AttachProxy(c.ProxyName, http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		// Wait until readPinging is done, before we proceed to reply
		<-readPinging
		pinged = true
		pingPath = r.URL.Path
		w.WriteHeader(200)
		w.Write([]byte("Yay, you managed to ping the end-point, secret=42!!!"))
	}))
	nilOrPanic(err, "Error failed to AttachProxy")

	result := buildRunSandbox(sandboxBuilder)
	nilOrPanic(control.CloseLog(), "Failed to close log")
	reader, err := ctx.NewLogReader()
	nilOrPanic(err, "Failed to open log reader")
	data, err := ioutil.ReadAll(reader)
	nilOrPanic(err, "Failed to read log")
	nilOrPanic(reader.Close(), "Failed to close log reader")
	nilOrPanic(control.Dispose(), "Failed to dispose TaskContext")
	log := string(data)

	if !result {
		panic("PingProxyPayload exited unsuccessfully")
	}
	if !pinged {
		panic("PingProxyPayload didn't call the attachedProxy")
	}
	if pingPath != "/v1/ping" {
		fmtPanic("PingProxyPayload pinged path: ", pingPath)
	}

	if !strings.Contains(log, "secret=42") {
		fmtPanic("Didn't find 'secret=42' from ping response in log", log)
	}
	if !strings.Contains(log, "Pinging") {
		fmtPanic("Didn't find 'Pinging' in log", string(data))
	}
}

// TestParallelPings checks that two parallel pings is possible when running
// two engines next to each other.
func (c *ProxyTestCase) TestParallelPings() {
	// TODO: Make two sandboxes. inside http.handler use a WaitGroup to ensure
	// that both sandboxes has sent their request to the proxy before either
	// one of the two handlers respond.
}

// Test runs all tests for the ProxyTestCase is parallel
func (c *ProxyTestCase) Test(t *testing.T) {
	c.ensureEngine(c.Engine)
	wg := sync.WaitGroup{}
	wg.Add(4)
	go func() { c.TestPingProxyPayload(); wg.Done() }()
	go func() { c.TestPing404IsUnsuccessful(); wg.Done() }()
	go func() { c.TestLiveLogging(); wg.Done() }()
	go func() { c.TestParallelPings(); wg.Done() }()
	wg.Wait()
}
