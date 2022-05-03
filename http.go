package panurge

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

func StandardServer(port int, handler http.Handler) *http.Server {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
		// ReadTimeout covers the time from when the
		// connection is accepted to when the request body is
		// fully read (if you do read the body, otherwise to
		// the end of the headers).
		ReadTimeout: 5 * time.Minute,
		// WriteTimeout normally covers the time from the end
		// of the request header read to the end of the
		// response write (a.k.a. the lifetime of the
		// ServeHTTP)
		WriteTimeout: 5 * time.Minute,
		Handler:      handler,
	}

	return srv
}

type TestServers struct {
	public   *httptest.Server
	internal *httptest.Server
}

func (ts *TestServers) Close() {
	ts.public.Close()
	ts.internal.Close()
}

func (ts *TestServers) GetPublic() *httptest.Server {
	return ts.public
}

func (ts *TestServers) GetInternal() *httptest.Server {
	return ts.internal
}

func WithAppTestServers(ts *TestServers) StandardAppOption {
	return func(app *StandardApp) {
		app.testServers = ts
	}
}
