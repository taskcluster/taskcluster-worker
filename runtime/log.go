package runtime

import (
	"fmt"

	"github.com/Sirupsen/logrus"
)

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
