package panurge

import (
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/sirupsen/logrus"
)

// Logger creates a new logger for structured logging.
func Logger(rawLogLevel string) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.WarnLevel)
	logger.Hooks.Add(&AnnotationHook{})

	if rawLogLevel != "" {
		logLevel, err := logrus.ParseLevel(rawLogLevel)
		if err != nil {
			logger.WithError(err).WithField(
				"raw_log_level", rawLogLevel,
			).Error("invalid log level")
		} else {
			logger.SetLevel(logLevel)
		}
	}

	return logger
}

type AnnotationHook struct{}

func (h *AnnotationHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *AnnotationHook) Fire(e *logrus.Entry) error {
	ann := GetContextAnnotations(e.Context)
	if ann == nil {
		return nil
	}

	e.Data["trace_id"] = ann.GetID()
	e.Data["user"] = ann.GetUser()
	e.Data["annotations"] = ann.GetAnnotations()

	// Only log metadata if something is going wrong
	if e.Level <= logrus.WarnLevel {
		e.Data["metadata"] = ann.GetMetadata()
	}

	seg := xray.GetSegment(e.Context)
	if seg == nil {
		return nil
	}

	e.Data["segment"] = seg.Name

	return nil
}
