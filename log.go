package panurge

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
)

type AnnotationHandler struct {
	handler slog.Handler
}

func NewAnnotationHandler(opts *slog.HandlerOptions, writer io.Writer) *AnnotationHandler {
	jsonOpts := &slog.HandlerOptions{
		Level: opts.Level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "time",
					Value: a.Value,
				}
			}
			if a.Key == slog.LevelKey {
				level := a.Value.Any().(slog.Level)

				return slog.Attr{
					Key:   "level",
					Value: slog.StringValue(strings.ToLower(level.String())),
				}
			}
			if a.Key == slog.MessageKey {
				return slog.Attr{
					Key:   "msg",
					Value: a.Value,
				}
			}

			return a
		},
	}

	if writer == nil {
		writer = os.Stdout
	}

	return &AnnotationHandler{
		handler: slog.NewJSONHandler(writer, jsonOpts),
	}
}

func (h *AnnotationHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *AnnotationHandler) Handle(ctx context.Context, r slog.Record) error {
	r.AddAttrs(slog.Time(slog.TimeKey, time.Now().UTC()))

	ann := GetContextAnnotations(ctx)
	if ann != nil {
		r.Add(
			slog.String("trace_id", ann.GetID()),
			slog.String("user", ann.GetUser()),
			slog.Any("annotations", ann.GetAnnotations()),
		)

		// Lägg till metadata endast för warn och error levels
		if r.Level >= slog.LevelWarn {
			r.Add(slog.Any("metadata", ann.GetMetadata()))
		}
	}

	// Lägg till X-Ray segment information
	if seg := xray.GetSegment(ctx); seg != nil {
		r.Add(slog.String("segment", seg.Name))
	}

	err := h.handler.Handle(ctx, r)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func (h *AnnotationHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &AnnotationHandler{
		handler: h.handler.WithAttrs(attrs),
	}
}

func (h *AnnotationHandler) WithGroup(name string) slog.Handler {
	return &AnnotationHandler{
		handler: h.handler.WithGroup(name),
	}
}

func Logger(logLevel string, writer io.Writer) *slog.Logger {
	level := slog.LevelWarn

	if logLevel != "" {
		err := level.UnmarshalText([]byte(logLevel))
		if err != nil {
			level = slog.LevelWarn

			slog.Error("invalid log level",
				"err", err,
				"log_level", logLevel)
		}
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := NewAnnotationHandler(opts, writer)
	logger := slog.New(handler)

	return logger
}
