package pt

import (
	"fmt"
	"testing"
)

func Must(t *testing.T, err error, msg string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

func Mustf(t *testing.T, err error, format string, a ...interface{}) {
	t.Helper()

	msg := fmt.Sprintf(format, a...)

	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}
