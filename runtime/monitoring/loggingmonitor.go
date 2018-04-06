package monitoring

import (
	"fmt"
	godebug "runtime/debug"
	"strings"
	"time"

	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"github.com/taskcluster/taskcluster-worker/runtime"
)

type loggingMonitor struct {
	*logrus.Entry
	prefix string
}

// NewLoggingMonitor creates a monitor that just logs everything. This won't
// attempt to send anything to sentry or statsum.
func NewLoggingMonitor(logLevel string, tags map[string]string, syslogName string) runtime.Monitor {
	// Create logger and parse logLevel
	logger := logrus.New()
	switch strings.ToLower(logLevel) {
	case logrus.DebugLevel.String():
		logger.Level = logrus.DebugLevel
	case logrus.InfoLevel.String():
		logger.Level = logrus.InfoLevel
	case logrus.WarnLevel.String():
		logger.Level = logrus.WarnLevel
	case logrus.ErrorLevel.String():
		logger.Level = logrus.ErrorLevel
	case logrus.FatalLevel.String():
		logger.Level = logrus.FatalLevel
	case logrus.PanicLevel.String():
		logger.Level = logrus.PanicLevel
	default:
		panic(fmt.Sprintf("Unsupported log-level: %s", logLevel))
	}

	// Convert tags to logrus.Fields
	fields := make(logrus.Fields, len(tags))
	for k, v := range tags {
		fields[k] = v
	}

	m := &loggingMonitor{
		Entry: logrus.NewEntry(logger).WithFields(fields),
	}

	if syslogName != "" {
		if err := setupSyslog(logger, syslogName); err != nil {
			m.ReportError(err, "Cannot set up syslog output")
		}
	}

	return m
}

func (m *loggingMonitor) Measure(name string, value ...float64) {
	strs := make([]string, len(value))
	for _, v := range value {
		strs = append(strs, fmt.Sprintf("%f", v))
	}
	m.Debugf("measure: %s%s recorded %s", m.prefix, name, strings.Join(strs, ","))
}

func (m *loggingMonitor) Count(name string, value float64) {
	m.Debugf("counter: %s%s incremented by %f", m.prefix, name, value)
}

func (m *loggingMonitor) Time(name string, fn func()) {
	start := time.Now()
	fn()
	m.Measure(name, time.Since(start).Seconds()*1000)
}

func (m *loggingMonitor) CapturePanic(fn func()) (incidentID string) {
	defer func() {
		if crash := recover(); crash != nil {
			message := fmt.Sprint(crash)
			incidentID = uuid.NewRandom().String()
			trace := godebug.Stack()
			m.Entry.WithField("incidentId", incidentID).WithField("panic", crash).Error(
				"Recovered from panic: ", message, "\nAt:\n", string(trace),
			)
		}
	}()
	fn()
	return
}

func (m *loggingMonitor) ReportError(err error, message ...interface{}) string {
	incidentID := uuid.NewRandom().String()
	m.Entry.WithField("incidentId", incidentID).WithError(err).Error(message...)
	return incidentID
}

func (m *loggingMonitor) ReportWarning(err error, message ...interface{}) string {
	incidentID := uuid.NewRandom().String()
	m.Entry.WithField("incidentId", incidentID).WithError(err).Warn(message...)
	return incidentID
}

func (m *loggingMonitor) WithTags(tags map[string]string) runtime.Monitor {
	// Construct fields for logrus (just satisfiying the type system)
	fields := make(map[string]interface{}, len(tags))
	for k, v := range tags {
		fields[k] = v
	}
	fields["prefix"] = m.prefix // don't allow overwrite "prefix"
	return &loggingMonitor{
		Entry:  m.Entry.WithFields(fields),
		prefix: m.prefix,
	}
}

func (m *loggingMonitor) WithTag(key, value string) runtime.Monitor {
	return m.WithTags(map[string]string{key: value})
}

func (m *loggingMonitor) WithPrefix(prefix string) runtime.Monitor {
	prefix = m.prefix + prefix
	return &loggingMonitor{
		Entry:  m.Entry.WithField("prefix", prefix),
		prefix: prefix + ".",
	}
}
