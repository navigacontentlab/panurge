package panurge

import (
	"fmt"
	"io"
)

type ErrorLogger interface {
	Errorf(format string, args ...interface{})
}

// SafeClose closes an io.Closer (f.ex. an io.ReadCloser) and logs an
// error if the Close() fails. This is meant to be used to capture
// close errors together with defer.
func SafeClose(logger ErrorLogger, name string, c io.Closer) {
	err := c.Close()
	if err != nil {
		logger.Errorf("failed to close %s: %v", name, err)
	}
}

// SafeClose closes an io.Closer (f.ex. an io.ReadCloser) and either
// sets outErr (if it's nil) or logs an error if the Close()
// fails. This is meant to be used to capture close errors together
// with defer.
func SafeFailingClose(logger ErrorLogger, outErr *error, name string, c io.Closer) {
	err := c.Close()
	if err != nil && *outErr == nil {
		*outErr = fmt.Errorf("failed to close %s: %w", name, err)
		return
	}

	if err != nil {
		logger.Errorf("failed to close %s: %v", name, err)
	}
}
