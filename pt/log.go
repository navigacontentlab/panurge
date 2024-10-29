package pt

import (
	"io"
	"testing"
)

func NewTestLogWriter(t *testing.T) io.Writer {
	t.Helper()

	return &testLogWriter{t: t}
}

type testLogWriter struct {
	t *testing.T
}

func (tlw testLogWriter) Write(d []byte) (int, error) {
	tlw.t.Log(string(d))

	return len(d), nil
}
