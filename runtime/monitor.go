package runtime

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	raven "github.com/getsentry/raven-go"
	"github.com/pborman/uuid"
	"github.com/taskcluster/statsum"
	"github.com/taskcluster/taskcluster-worker/runtime/client"
)

// A Monitor is responsible for collecting logs, stats and error messages.
//
// A monitor is a context aware object for monitoring. That is to say that a
// Monitor is used to record metrics, write logs and report errors. When doing
// so the Monitor object adds meta-data to the metrics, logs and errors. The
// meta-data added is context dependent tags and prefix. These help identify
// where a log message, metric or error originates from.
//
// By encapsulating the context meta-data inside the Monitor object, an
// implementor gets a Monitor rarely needs to add tags or prefix. For example
// a monitor will always be prefixed by plugin name before being passed to a
// plugin, hence, it is easy trace any log message, metric or error report to
// the plugin that it was created in.
//
// When passing a Monitor to a sub-component it often makes sense to add
// additional tags or prefix. This way a downloader function that takes a
// Monitor need not worry about being able to distinguish its metrics, logs and
// errors from that of its parent.
//
// Prefixes should always be constants, such as engine, plugin, function or
// component names. Values that change such as taskId or runId should not be
// used as prefixes, such values is however great as tags.
//
// All metrics reported for a given prefix + name will be aggregated. Hence, if
// taskId was used as prefix, the dimensionality of metrics would explode and
// the aggregation wouldn't be useful.
type Monitor interface {
	// Measure values in statsum
	Measure(name string, value ...float64)
	// Increment counters in statsum
	Count(name string, value float64)
	// Measure time of fn in statsum
	Time(name string, fn func())

	// Report error/warning to sentry and write to log, returns incidentId which
	// can be included in task-logs, if relevant.
	ReportError(err error, message ...interface{}) string
	ReportWarning(err error, message ...interface{}) string

	// Write log messages to system log
	Debug(...interface{})
	Debugln(...interface{})
	Debugf(string, ...interface{})
	Print(...interface{})
	Println(...interface{})
	Printf(string, ...interface{})
	Info(...interface{})
	Infoln(...interface{})
	Infof(string, ...interface{})
	Warn(...interface{})
	Warnln(...interface{})
	Warnf(string, ...interface{})
	Error(...interface{})
	Errorln(...interface{})
	Errorf(string, ...interface{})
	Panic(...interface{})
	Panicln(...interface{})
	Panicf(string, ...interface{})

	// Create child monitor with given tags (tags don't apply to statsum)
	WithTags(tags map[string]string) Monitor
	WithTag(key, value string) Monitor
	// Create child monitor with given prefix (prefix applies to everything)
	WithPrefix(prefix string) Monitor
}

// NewMonitor creates a new monitor
func NewMonitor(project string, auth client.Auth, logLevel string, tags map[string]string) Monitor {
	// Create statsumConfigurer
	statsumConfigurer := func(project string) (statsum.Config, error) {
		res, err := auth.StatsumToken(project)
		if err != nil {
			return statsum.Config{}, err
		}
		return statsum.Config{
			Project: res.Project,
			BaseURL: res.BaseURL,
			Token:   res.Token,
			Expires: time.Time(res.Expires),
		}, nil
	}

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

	// Declare monitor so we can reference it in OnError
	var m *monitor
	m = &monitor{
		Statsum: statsum.New(project, statsumConfigurer, statsum.Options{
			OnError: func(err error) { m.ReportWarning(err) },
		}),
		Entry: logrus.NewEntry(logger).WithFields(fields),
		sentry: &sentry{
			client:  nil,
			project: project,
			auth:    auth,
		},
	}
	return m
}

type sentry struct {
	client     *raven.Client
	m          sync.Mutex
	project    string
	expiration time.Time
	auth       client.Auth
}

func (s *sentry) Client() (*raven.Client, error) {
	s.m.Lock()
	defer s.m.Unlock()

	// Refresh sentry DSN if necessary
	if s.expiration.Before(time.Now()) {
		// Fetch DSN
		res, err := s.auth.SentryDSN(s.project)
		if err != nil {
			return nil, err
		}
		// Create or update DSN for the client
		if s.client == nil {
			s.client, err = raven.New(res.Dsn.Secret)
		} else {
			err = s.client.SetDSN(res.Dsn.Secret)
		}
		if err != nil {
			return nil, err
		}
		// Set expiration, so we remember to refresh
		s.expiration = time.Time(res.Expires)
	}

	return s.client, nil
}

type monitor struct {
	*statsum.Statsum
	*logrus.Entry
	*sentry
	tags   map[string]string
	prefix string
}

func (m *monitor) Time(name string, fn func()) {
	m.Statsum.Time(name, fn)
}

func (m *monitor) ReportError(err error, message ...interface{}) string {
	incidentID := uuid.NewRandom().String()
	m.Entry.WithField("incidentId", incidentID).WithError(err).Error(message...)
	m.submitError(err, fmt.Sprint(message...), raven.ERROR, incidentID)
	return incidentID
}

func (m *monitor) ReportWarning(err error, message ...interface{}) string {
	incidentID := uuid.NewRandom().String()
	m.Entry.WithField("incidentId", incidentID).WithError(err).Warn(message...)
	m.submitError(err, fmt.Sprint(message...), raven.WARNING, incidentID)
	return incidentID
}

func (m *monitor) submitError(err error, message string, level raven.Severity, incidentID string) {
	// Capture stack trace
	exception := raven.NewException(err, raven.NewStacktrace(2, 5, []string{
		"github.com/taskcluster/",
	}))

	// Create error packet
	text := fmt.Sprintf("Error: %s\nMessage: %s", err.Error(), message)
	packet := raven.NewPacket(text, nil, exception)
	packet.Level = level
	packet.EventID = incidentID

	// Add incidentID and prefix to tags
	tags := make(map[string]string, len(m.tags)+2)
	for tag, value := range m.tags {
		tags[tag] = value
	}
	tags["incidentId"] = incidentID
	tags["prefix"] = m.prefix

	// Get client with fresh sentry DSN (if cached is old)
	client, rerr := m.sentry.Client()
	if rerr != nil {
		m.Error("Failed to obtain sentry DSN, error: ", rerr)
		m.Error("Failed to send error: ", err)
		return
	}

	// Send packet
	_, done := client.Capture(packet, tags)
	<-done
}

func (m *monitor) WithTags(tags map[string]string) Monitor {
	// Merge tags from monitor and tags
	allTags := make(map[string]string, len(m.tags)+len(tags))
	for k, v := range m.tags {
		allTags[k] = v
	}
	for k, v := range tags {
		allTags[k] = v
	}
	// Construct fields for logrus (just satisfiying the type system)
	fields := make(map[string]interface{}, len(allTags))
	for k, v := range allTags {
		fields[k] = v
	}
	fields["prefix"] = m.prefix // don't allow overwrite "prefix"
	return &monitor{
		Statsum: m.Statsum,
		Entry:   m.Entry.WithFields(fields),
		sentry:  m.sentry,
		tags:    allTags,
		prefix:  m.prefix,
	}
}

func (m *monitor) WithTag(key, value string) Monitor {
	return m.WithTags(map[string]string{key: value})
}

func (m *monitor) WithPrefix(prefix string) Monitor {
	completePrefix := prefix
	if m.prefix != "" {
		completePrefix = m.prefix + "." + prefix
	}
	return &monitor{
		Statsum: m.Statsum.WithPrefix(prefix),
		Entry:   m.Entry.WithField("prefix", completePrefix),
		sentry:  m.sentry,
		tags:    m.tags,
		prefix:  completePrefix,
	}
}

type loggingMonitor struct {
	*logrus.Entry
	prefix string
}

// NewLoggingMonitor creates a monitor that just logs everything. This won't
// attempt to send anything to sentry or statsum.
func NewLoggingMonitor(logLevel string, tags map[string]string) Monitor {
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

	return &loggingMonitor{
		Entry: logrus.NewEntry(logger).WithFields(fields),
	}
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

func (m *loggingMonitor) WithTags(tags map[string]string) Monitor {
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

func (m *loggingMonitor) WithTag(key, value string) Monitor {
	return m.WithTags(map[string]string{key: value})
}

func (m *loggingMonitor) WithPrefix(prefix string) Monitor {
	prefix = m.prefix + prefix
	return &loggingMonitor{
		Entry:  m.Entry.WithField("prefix", prefix),
		prefix: prefix + ".",
	}
}
