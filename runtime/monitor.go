package runtime

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

	// CapturePanic reports panics to log/sentry and returns incidentID, if any
	CapturePanic(fn func()) (incidentID string)

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
