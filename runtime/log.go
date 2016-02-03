package runtime

import (
	"fmt"

	"github.com/Sirupsen/logrus"
)

// Logger interface allows a logging object to be passed around as long as they
// implement these methods without requiring a specific package.  Allows migrating
// between logging packages along as this contract is maintained.
type Logger interface {
	WithField(key string, value interface{}) *logrus.Entry
	WithFields(fields logrus.Fields) *logrus.Entry
	WithError(err error) *logrus.Entry

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})

	Debug(args ...interface{})
	Info(args ...interface{})
	Print(args ...interface{})
	Warn(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})

	Debugln(args ...interface{})
	Infoln(args ...interface{})
	Println(args ...interface{})
	Warnln(args ...interface{})
	Warningln(args ...interface{})
	Errorln(args ...interface{})
	Fatalln(args ...interface{})
	Panicln(args ...interface{})
}

// Create a logger that can be passed around through the environment.
// Loggers can be created based on the one returned from this method by calling
// WithField or WithFields and specifying additional fields that the package
// would like.
func CreateLogger(level interface{}) (*logrus.Logger, error) {
	if level == nil || level == "" {
		level = "warn"
	}
	loggingLevel := level.(string)

	lvl, err := logrus.ParseLevel(loggingLevel)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse logging level: %s\n", level)
	}

	logger := logrus.New()
	logger.Level = lvl
	return logger, nil
}
