package enginetest

import (
	"fmt"
	"strings"
	"sync"
)

// A LoggingTestCase holds information necessary to run tests that an engine
// can write things to the log.
type LoggingTestCase struct {
	*EngineProvider
	// String that we will look for in the log
	Target string
	// A task.payload as accepted by the engine, which will Target to the log and
	// exit successfully.
	TargetPayload string
	// A task.payload which will write Target, but the task will be unsuccessful.
	FailingPayload string
	// A task.payload which won't write Target to the log, but will by successful.
	SilentPayload string
}

func (c *LoggingTestCase) grepLogFromPayload(payload string, needle string, success, match bool) bool {
	debug(" - New run")
	r := c.newRun()
	defer r.Dispose()
	debug(" - New sandbox builder")
	r.NewSandboxBuilder(payload)
	debug(" - Build and run")
	s := r.buildRunSandbox()
	debug(" - Result: %v", s)
	if s != success {
		fmtPanic("Task with payload: ", payload, " had ResultSet.Success(): ", s)
	}
	log := r.ReadLog()
	retval := strings.Contains(log, needle)
	if retval != match {
		fmt.Println("--- Couldn't find: ", needle, " in log: ---")
		fmt.Println(log)
		fmt.Println("--- END OF LOG ---")
	}
	return retval
}

// TestLogTarget check that Target is logged by TargetPayload
func (c *LoggingTestCase) TestLogTarget() {
	debug("## TestLogTarget")
	if !c.grepLogFromPayload(c.TargetPayload, c.Target, true, true) {
		fmtPanic("Couldn't find target: ", c.Target, " in logs from TargetPayload")
	}
}

// TestLogTargetWhenFailing check that Target is logged by FailingPayload
func (c *LoggingTestCase) TestLogTargetWhenFailing() {
	debug("## TestLogTargetWhenFailing")
	if !c.grepLogFromPayload(c.FailingPayload, c.Target, false, true) {
		fmtPanic("Couldn't find target: ", c.Target, " in logs from FailingPayload")
	}
}

// TestSilentTask checks that Target isn't logged by SilentPayload
func (c *LoggingTestCase) TestSilentTask() {
	debug("## TestSilentTask")
	if c.grepLogFromPayload(c.SilentPayload, c.Target, true, false) {
		fmtPanic("Found target: ", c.Target, " in logs from SilentPayload")
	}
}

// Test will run all logging tests
func (c *LoggingTestCase) Test() {
	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() { c.TestLogTarget(); wg.Done() }()
	go func() { c.TestLogTargetWhenFailing(); wg.Done() }()
	go func() { c.TestSilentTask(); wg.Done() }()
	wg.Wait()
}
