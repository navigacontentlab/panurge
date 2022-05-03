package panurge_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/navigacontentlab/panurge"
	"github.com/navigacontentlab/panurge/pt"
)

type logOutput struct {
	TestName    string                 `json:"-"`
	TraceID     string                 `json:"trace_id"`
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

	var buf bytes.Buffer
	logger := panurge.Logger("info")
	logger.Out = &buf

	ctx, seg := xray.BeginSegment(context.Background(), "testSeg")
	seg.Dummy = dummy
	ctx = panurge.ContextWithAnnotations(ctx)

	panurge.AddUserAnnotation(ctx, "some-individual")

	logger.WithContext(ctx).Info("when it all began")

	panurge.AddAnnotation(ctx, "document", "abc123")
	panurge.AddMetadata(ctx, "data", "BIG HONKING VALUE")

	// Do some work in a child context. The XRay segment is
	// shared/inherited so logging will include XRay information
	// as long as the context is preserved.
	func(ctx context.Context) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		panurge.AddAnnotation(ctx, "relevantInfo", "Stig was here 1994")

		// Do stuff
	}(ctx)

	logger.WithContext(ctx).Info("thing was done")
	logger.WithContext(ctx).WithError(errors.New("nicely jobs everyone")).Error("but then the shit hit the fan")
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
			Level:    "warning",
			Msg:      "I know nothing",
		},
	}

	dec := json.NewDecoder(&buf)

	for i := range wantEntries {
		want := wantEntries[i]

		t.Run(want.TestName, func(t *testing.T) {
			var got logOutput

			err := dec.Decode(&got)
			if err != nil {
				t.Fatalf("failed to decode log output")
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
