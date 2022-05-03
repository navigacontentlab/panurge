package navigaid

import (
	"fmt"
	"net/http"
)

func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: &Transport{
			Base: http.DefaultTransport,
		},
	}
}

// Transport is an http.RoundTripper that makes OAuth 2.0 HTTP
// requests based of the incoming NavigaID context.
type Transport struct {
	// Base is the base RoundTripper used to make HTTP requests.
	// If nil, http.DefaultTransport is used.
	Base http.RoundTripper
}

// RoundTrip authorizes and authenticates the request with an
// access token from Transport's Source.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqBodyClosed := false
	if req.Body != nil {
		defer func() {
			if !reqBodyClosed {
				req.Body.Close()
			}
		}()
	}

	auth, err := GetAuth(req.Context())
	if err != nil {
		return nil, fmt.Errorf("no authentication information in context: %w", err)
	}

	req2 := cloneRequest(req) // per RoundTripper contract
	req2.Header.Set("Authorization", "Bearer "+auth.AccessToken)

	// req.Body is assumed to be closed by the base RoundTripper.
	reqBodyClosed = true
	return t.base().RoundTrip(req2)
}

func (t *Transport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}
