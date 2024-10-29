package panurge_test

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/navigacontentlab/panurge"
)

func Test_SafeFailingClose__SetsErr(t *testing.T) {
	tc := testCloser{Err: testDummyErr("close error")}

	err := testCloserHandler(t, tc, nil)
	if err == nil {
		t.Fatal("expected the handler to return an error")
	}

	if !errors.Is(err, tc.Err) {
		t.Fatalf("expected the returned error to wrap the close error, got: %v", err)
	}

	if testing.Verbose() {
		t.Logf("got the expected error %q", err)
	}
}

func Test_SafeFailingClose__LogsErr(t *testing.T) {
	tc := testCloser{Err: testDummyErr("close error")}
	opErr := testDummyErr("op failure")

	var lc testLogCapture

	err := testCloserHandler(&lc, tc, opErr)
	if err == nil {
		t.Fatal("expected the handler to return an error")
	}

	if !errors.Is(err, opErr) {
		t.Fatalf("expected the returned error chain to contain the op error, got: %v", err)
	}

	if testing.Verbose() {
		t.Logf("got the expected error %q", err)
	}

	if len(lc.Entries) != 1 {
		t.Fatalf("expected a single error log entry, got %d entries", len(lc.Entries))
	}

	if !strings.HasSuffix(lc.Entries[0], tc.Err.Error()) {
		t.Fatalf("expected the close error to be the suffix of the logged error, got %q", lc.Entries[0])
	}

	if testing.Verbose() {
		t.Logf("got the expected log entry %q", lc.Entries[0])
	}
}

func Test_SafeFailingClose__NoopOnNoErr(t *testing.T) {
	var tc testCloser

	var lc testLogCapture

	err := testCloserHandler(&lc, tc, nil)
	if err != nil {
		t.Fatalf("didn't expect the handler to return an error, got: %v", err)
	}

	if len(lc.Entries) != 0 {
		for _, e := range lc.Entries {
			t.Error(e)
		}

		t.Fatalf("didn't expect any error log entries, got %d entries", len(lc.Entries))
	}
}

func testDummyErr(msg string) error {
	return fmt.Errorf("%s [%d]", msg, time.Now().Unix())
}

type testLogCapture struct {
	Entries []string
}

func (lc *testLogCapture) Errorf(format string, args ...interface{}) {
	lc.Entries = append(lc.Entries, fmt.Sprintf(format, args...))
}

type testCloser struct {
	Err error
}

func (tc testCloser) Close() error {
	return tc.Err
}

func testCloserHandler(logger panurge.ErrorLogger, c io.Closer, failErr error) (outErr error) {
	defer panurge.SafeFailingClose(logger, &outErr, "test closer", c)

	if failErr != nil {
		return failErr
	}

	return nil
}
