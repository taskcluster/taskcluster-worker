package runtime

import (
	"fmt"

	"github.com/Sirupsen/logrus"
)

// CreateLogger returns a new logger that can be passed around through the environment.
// Loggers can be created based on the one returned from this method by calling
// WithField or WithFields and specifying additional fields that the package
// would like.
func CreateLogger(level string) (*logrus.Logger, error) {
	if level == "" {
		level = "warn"
	}

	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid logging level: '%s'", level)
	}

	logger := logrus.New()
	logger.Level = lvl
	return logger, nil
}
