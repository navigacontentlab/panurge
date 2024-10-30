package panurge

import (
	"fmt"
	"log/slog"

	"github.com/aws/aws-xray-sdk-go/strategy/ctxmissing"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/aws/aws-xray-sdk-go/xraylog"
)

// ConfigureXRay sets up XRay with a slog logger and makes sure that
// XRay doesn't panic when a context is missing.
func ConfigureXRay(logger *slog.Logger, version string) {
	err := xray.Configure(xray.Config{
		ServiceVersion:         version,
		ContextMissingStrategy: ctxmissing.NewDefaultLogErrorStrategy(),
	})
	if err != nil {
		logger.Error(fmt.Sprintf("failed to configure XRay: %v", err))
	}

	xray.SetLogger(&xrayLogrusAdapter{logger: logger})
}

type xrayLogrusAdapter struct {
	logger *slog.Logger
}

func (xl *xrayLogrusAdapter) Log(level xraylog.LogLevel, msg fmt.Stringer) {
	switch level {
	case xraylog.LogLevelDebug:
		xl.logger.Debug(msg.String())
	case xraylog.LogLevelInfo:
		xl.logger.Info(msg.String())
	case xraylog.LogLevelWarn:
		xl.logger.Warn(msg.String())
	case xraylog.LogLevelError:
		xl.logger.Error(msg.String())
	default:
		xl.logger.Warn(msg.String())
	}
}
