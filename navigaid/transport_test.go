package navigaid_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dimelords/panurge/navigaid"
)

func TestTransport(t *testing.T) {
	token := "abc123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		expect := "Bearer " + token
		if req.Header.Get("Authorization") != expect {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))

	client := server.Client()
	client.Transport = &navigaid.Transport{
		Base: client.Transport,
	}

	ctx := navigaid.SetAuth(context.Background(), navigaid.AuthInfo{
		AccessToken: token,
	}, nil)

	req, err := http.NewRequest(http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create test request: %v", err)
	}

	res, err := client.Do(req.WithContext(ctx))
	defer func() {
		_ = res.Body.Close()
	}()

	if err != nil {
		t.Fatalf("failed to perform test request: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("error response from server: %s", res.Status)
	}
}
