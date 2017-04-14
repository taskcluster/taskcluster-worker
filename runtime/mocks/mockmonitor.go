package mocks

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	godebug "runtime/debug"

	"github.com/pborman/uuid"
	"github.com/taskcluster/taskcluster-worker/runtime"
	"github.com/taskcluster/taskcluster-worker/runtime/util"
)

var mockMonitorLog = util.Debug("monitor")

type metricCache struct {
	m        sync.Mutex
	measures map[string]bool
	counters map[string]bool
}

// MockMonitor implements runtime.Monitor for use in unit tests.
type MockMonitor struct {
	tags         map[string]string
	prefix       string
	metadata     string
	panicOnError bool
	cache        *metricCache
}

// NewMockMonitor returns a Monitor that prints all messages using util.Debug()
// meaning that you must set environment variable DEBUG='monitor' to see the
// messages.
//
// If panicOnError is set this will panic if Error() or reportError() is called.
// This is often useful for testing components that takes a Monitor as argument.
func NewMockMonitor(panicOnError bool) *MockMonitor {
	return &MockMonitor{
		panicOnError: panicOnError,
		cache: &metricCache{
			measures: make(map[string]bool),
			counters: make(map[string]bool),
		},
	}
}

// Measure records values for given name
func (m *MockMonitor) Measure(name string, value ...float64) {
	m.cache.m.Lock()
	defer m.cache.m.Unlock()

	m.cache.measures[m.prefix+name] = true
}

// Count incrments counter by name with given value
func (m *MockMonitor) Count(name string, value float64) {
	m.cache.m.Lock()
	defer m.cache.m.Unlock()

	m.cache.counters[m.prefix+name] = true
}

// Time measures and records the execution time of fn
func (m *MockMonitor) Time(name string, fn func()) {
	start := time.Now()
	fn()
	m.Measure(name, time.Since(start).Seconds()*1000)
}

// HasMeasure returns true if a measure with given name has been reported
func (m *MockMonitor) HasMeasure(name string) bool {
	m.cache.m.Lock()
	defer m.cache.m.Unlock()

	return m.cache.measures[m.prefix+name]
}

// HasCounter returns true if a counter with given name has been reported
func (m *MockMonitor) HasCounter(name string) bool {
	m.cache.m.Lock()
	defer m.cache.m.Unlock()

	return m.cache.counters[m.prefix+name]
}

// CapturePanic recovers from panic in fn and returns incidentID, if any
func (m *MockMonitor) CapturePanic(fn func()) (incidentID string) {
	defer func() {
		if crash := recover(); crash != nil {
			incidentID = uuid.NewRandom().String()
			trace := godebug.Stack()
			text := fmt.Sprint("Recovered from panic: ", crash, "\nAt:\n", string(trace))
			m.WithTag("incidentId", incidentID).(*MockMonitor).output("PANIC", text)
			if m.panicOnError {
				panic(fmt.Sprintf("Panic: %s", text))
			}
		}
	}()
	fn()
	return
}

// ReportError records an error, and panics if panicOnError was set
func (m *MockMonitor) ReportError(err error, message ...interface{}) string {
	incidentID := uuid.NewRandom().String()
	text := fmt.Sprint(append([]interface{}{"error: ", err}, message))
	m.WithTag("incidentId", incidentID).(*MockMonitor).output("ERROR-REPORT", text)
	if m.panicOnError {
		panic(fmt.Sprintf("ReportError: %s", text))
	}
	return incidentID
}

// ReportWarning logs a warning
func (m *MockMonitor) ReportWarning(err error, message ...interface{}) string {
	incidentID := uuid.NewRandom().String()
	text := fmt.Sprint(append([]interface{}{"error: ", err}, message))
	m.WithTag("incidentId", incidentID).(*MockMonitor).output("WARNING-REPORT", text)
	return incidentID
}

func (m *MockMonitor) output(kind string, a ...interface{}) {
	mockMonitorLog("%s: %s (%s)", kind, fmt.Sprint(a...), m.metadata)
}

// Debug writes a debug message
func (m *MockMonitor) Debug(a ...interface{}) { m.output("DEBUG", a...) }

// Debugln writes a debug message
func (m *MockMonitor) Debugln(a ...interface{}) { m.Debug(fmt.Sprintln(a...)) }

// Debugf writes debug message labelled as Debug
func (m *MockMonitor) Debugf(f string, a ...interface{}) { m.Debug(fmt.Sprintf(f, a...)) }

// Print writes debug message labelled as Print
func (m *MockMonitor) Print(a ...interface{}) { m.output("INFO", a...) }

// Println writes debug message labelled as Print
func (m *MockMonitor) Println(a ...interface{}) { m.Print(fmt.Sprintln(a...)) }

// Printf writes debug message labelled as Print
func (m *MockMonitor) Printf(f string, a ...interface{}) { m.Print(fmt.Sprintf(f, a...)) }

// Info writes debug message labelled as Info
func (m *MockMonitor) Info(a ...interface{}) { m.output("INFO", a...) }

// Infoln writes debug message labelled as Info
func (m *MockMonitor) Infoln(a ...interface{}) { m.Info(fmt.Sprintln(a...)) }

// Infof writes debug message labelled as Info
func (m *MockMonitor) Infof(f string, a ...interface{}) { m.Info(fmt.Sprintf(f, a...)) }

// Warn writes debug message labelled as Warn
func (m *MockMonitor) Warn(a ...interface{}) { m.output("WARN", a...) }

// Warnln writes debug message labelled as Warn
func (m *MockMonitor) Warnln(a ...interface{}) { m.Warn(fmt.Sprintln(a...)) }

// Warnf writes debug message labelled as Warn
func (m *MockMonitor) Warnf(f string, a ...interface{}) { m.Warn(fmt.Sprintf(f, a...)) }

// Errorln writes debug message labelled as Error, and panics if panicOnError was set
func (m *MockMonitor) Errorln(a ...interface{}) { m.Error(fmt.Sprintln(a...)) }

// Errorf writes debug message labelled as Error, and panics if panicOnError was set
func (m *MockMonitor) Errorf(f string, a ...interface{}) { m.Error(fmt.Sprintf(f, a...)) }

// Panicln writes debug message labelled as Panic, and panics
func (m *MockMonitor) Panicln(a ...interface{}) { m.Panic(fmt.Sprintln(a...)) }

// Panicf writes debug message labelled as Panic, and panics
func (m *MockMonitor) Panicf(f string, a ...interface{}) { m.Panic(fmt.Sprintf(f, a...)) }

// Error writes debug message labelled as Error, and panics if panicOnError was set
func (m *MockMonitor) Error(a ...interface{}) {
	m.output("ERROR", a...)
	if m.panicOnError {
		panic(fmt.Sprint(a...))
	}
}

// Panic writes debug message labelled as Panic, and panics
func (m *MockMonitor) Panic(a ...interface{}) {
	m.output("PANIC", a...)
	panic(fmt.Sprint(a...))
}

// WithTags creates a new child Monitor with given tags
func (m *MockMonitor) WithTags(tags map[string]string) runtime.Monitor {
	allTags := make(map[string]string, len(m.tags))
	for k, v := range m.tags {
		allTags[k] = v
	}
	for k, v := range tags {
		allTags[k] = v
	}
	return &MockMonitor{
		tags:         allTags,
		prefix:       m.prefix,
		metadata:     mockMonitorMetadata(allTags, m.prefix),
		panicOnError: m.panicOnError,
		cache:        m.cache,
	}
}

// WithTag creates a new child Monitor with given tag
func (m *MockMonitor) WithTag(key, value string) runtime.Monitor {
	return m.WithTags(map[string]string{key: value})
}

// WithPrefix creates a new child Monitor with given prefix
func (m *MockMonitor) WithPrefix(prefix string) runtime.Monitor {
	if prefix != "" {
		prefix += "."
	}
	return &MockMonitor{
		tags:         m.tags,
		prefix:       m.prefix + prefix,
		metadata:     mockMonitorMetadata(m.tags, m.prefix+prefix),
		panicOnError: m.panicOnError,
		cache:        m.cache,
	}
}

func mockMonitorMetadata(tags map[string]string, prefix string) string {
	if strings.HasSuffix(prefix, ".") {
		prefix = prefix[:len(prefix)-1]
	}

	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(tags)+1)
	pairs = append(pairs, fmt.Sprintf("prefix=%s", prefix))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, tags[k]))
	}
	return strings.Join(pairs, " ")
}
