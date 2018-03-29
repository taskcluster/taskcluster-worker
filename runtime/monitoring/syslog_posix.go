// +build !windows

package monitoring

import (
	syslog "log/syslog"

	"github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
)

func getSyslogPriority(level logrus.Level) syslog.Priority {
	priority := syslog.LOG_DAEMON

	switch level {
	case logrus.PanicLevel:
		priority |= syslog.LOG_CRIT
	case logrus.FatalLevel:
		priority |= syslog.LOG_CRIT
	case logrus.ErrorLevel:
		priority |= syslog.LOG_ERR
	case logrus.WarnLevel:
		priority |= syslog.LOG_WARNING
	case logrus.InfoLevel:
		priority |= syslog.LOG_INFO
	case logrus.DebugLevel:
		priority |= syslog.LOG_DEBUG
	}

	return priority
}

func setupSyslog(logger *logrus.Logger, syslogName string) error {
	hook, err := logrus_syslog.NewSyslogHook("", "", getSyslogPriority(logger.Level), syslogName)
	if err != nil {
		return err
	}
	logger.Hooks.Add(hook)
	return nil
}
