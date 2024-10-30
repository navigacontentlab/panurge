package panurge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/navigacontentlab/panurge"
	"github.com/navigacontentlab/panurge/pt"
)

// testBuffer är en wrapper runt bytes.Buffer som implementerar io.Writer.
type testBuffer struct {
	buf bytes.Buffer
}

func (b *testBuffer) Write(p []byte) (n int, err error) {
	write, err := b.buf.Write(p)

	if err != nil {
		return write, fmt.Errorf("%w", err)
	}

	return write, nil
}

type logOutput struct {
	TestName    string                 `json:"-"`
	TraceID     string                 `json:"trace_id"` //nolint:tagliatelle
	Segment     string                 `json:"segment"`
	Annotations map[string]interface{} `json:"annotations"`
	Metadata    map[string]interface{} `json:"metadata"`
	Level       string                 `json:"level"`
	Msg         string                 `json:"msg"`
	Time        time.Time              `json:"time"`
	User        string                 `json:"user"`
	Error       string                 `json:"error"`
}

func TestLogger(t *testing.T) {
	verifyLogEntries(t, false)
}

func TestLogger_XRayDummy(t *testing.T) {
	verifyLogEntries(t, true)
}

func verifyLogEntries(t *testing.T, dummy bool) {
	t.Helper()

	err := xray.Configure(xray.Config{
		SamplingStrategy: SamplingStrategy(false),
	})
	pt.Must(t, err, "failed to disable the centralised XRay sampling strategy")

	// Skapa en buffer för att fånga loggar
	buf := &testBuffer{}

	logger := panurge.Logger(slog.LevelInfo.String(), buf)

	ctx, seg := xray.BeginSegment(context.Background(), "testSeg")
	seg.Dummy = dummy
	ctx = panurge.ContextWithAnnotations(ctx)

	panurge.AddUserAnnotation(ctx, "some-individual")

	logger.InfoContext(ctx, "when it all began")

	panurge.AddAnnotation(ctx, "document", "abc123")
	panurge.AddMetadata(ctx, "data", "BIG HONKING VALUE")

	// Do some work in a child context
	func(ctx context.Context) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		panurge.AddAnnotation(ctx, "relevantInfo", "Stig was here 1994")
	}(ctx)

	logger.InfoContext(ctx, "thing was done")
	logger.ErrorContext(ctx, "but then the shit hit the fan",
		"error", "nicely jobs everyone",
	)
	logger.Warn("I know nothing")

	wantEntries := []logOutput{
		{
			TestName: "Initial",
			Level:    "info",
			Msg:      "when it all began",
			TraceID:  seg.TraceID,
			Segment:  "testSeg",
			User:     "some-individual",
		},
		{
			TestName: "Info",
			Level:    "info",
			Msg:      "thing was done",
			TraceID:  seg.TraceID,
			Segment:  "testSeg",
			User:     "some-individual",
			Annotations: map[string]interface{}{
				"document":     "abc123",
				"relevantInfo": "Stig was here 1994",
			},
		},
		{
			TestName: "Error",
			Level:    "error",
			Msg:      "but then the shit hit the fan",
			Error:    "nicely jobs everyone",
			TraceID:  seg.TraceID,
			Segment:  "testSeg",
			User:     "some-individual",
			Annotations: map[string]interface{}{
				"document":     "abc123",
				"relevantInfo": "Stig was here 1994",
			},
			Metadata: map[string]interface{}{
				"data": "BIG HONKING VALUE",
			},
		},
		{
			TestName: "Nothing",
			Level:    "warn",
			Msg:      "I know nothing",
		},
	}

	dec := json.NewDecoder(&buf.buf)

	for i := range wantEntries {
		want := wantEntries[i]

		t.Run(want.TestName, func(t *testing.T) {
			var got logOutput

			err := dec.Decode(&got)
			if err != nil {
				t.Fatalf("failed to decode log output: %v", err)
			}

			if got.Time.IsZero() {
				t.Error("expected log entry time to be non-zero")
			}

			ignore := []string{"Time", "TestName"}

			if dummy {
				ignore = append(ignore, "TraceID")
			}

			opts := cmpopts.IgnoreFields(logOutput{}, ignore...)

			if diff := cmp.Diff(want, got, opts); diff != "" {
				t.Errorf("logger output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
