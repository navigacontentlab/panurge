package navigaid

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type MockServer struct {
	Server     *httptest.Server
	PrivateKey *rsa.PrivateKey
	Client     *http.Client
}

type MockServerOptions struct {
	Claims Claims
	TTL    int `json:"ttl"`
}

const testKeyID = "a34db85a-3f67-42e0-94f6-17694fc85fd9"

// This mock server mocks two endpoints, one for creating new access tokens
// and another one for providing keys.
func NewMockServer(opts MockServerOptions) (*MockServer, error) {
	mux, privateKey, err := NewMockService(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create access token mock server: %w", err)
	}

	srv := httptest.NewServer(mux)

	mockServer := MockServer{
		Server:     srv,
		Client:     srv.Client(),
		PrivateKey: privateKey,
	}

	return &mockServer, nil
}

// This mock service mocks two endpoints, one for creating new access
// tokens and another one for providing keys.
func NewMockService(opts MockServerOptions) (*http.ServeMux, *rsa.PrivateKey, error) {
	mux := http.NewServeMux()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	mux.HandleFunc("/v1/token", func(w http.ResponseWriter, r *http.Request) {
		tokenTTL := 600 * time.Second

		if val := r.URL.Query().Get("ttl"); val != "" {
			if queryTTL, err := strconv.ParseUint(val, 0, 0); err == nil {
				tokenTTL = time.Duration(queryTTL) * time.Second
			}
		} else if opts.TTL != 0 {
			tokenTTL = time.Duration(opts.TTL) * time.Second
		}

		jwtClaims := jwt.MapClaims{
			"sub":         opts.Claims.Subject,
			"org":         opts.Claims.Org,
			"ntt":         "access_token",
			"exp":         time.Now().Add(tokenTTL).Unix(),
			"iat":         time.Now().Unix(),
			"jti":         "da20dda4-c8ce-4dac-98dc-435f2f0128f1",
			"permissions": opts.Claims.Permissions,
		}

		if hasHeaderSpecifiedClaims(r) {
			err = updateClaimsWithHeaderSpecifiedClaims(r, jwtClaims)
			if err != nil {
				_, _ = w.Write([]byte(fmt.Sprintf("failed to use header specified claims: %v", err.Error())))
			}
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS512, jwtClaims)

		token.Header["kid"] = testKeyID

		signed, err := token.SignedString(privateKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("failed to sign access token: %v", err.Error())))
			return
		}

		resp := fmt.Sprintf(`
		{
			"access_token": "%s",
			"token_type": "Bearer",
			"expires_in": %d
		}
		`, signed, int(tokenTTL.Seconds()))

		w.Header().Add("Content-Type", "application/json; charset=utf-8")

		_, err = io.WriteString(w, resp)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(fmt.Sprintf("failed to write out access token response: %v", err.Error())))
		}
	})

	mux.HandleFunc("/v1/jwks", func(w http.ResponseWriter, r *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())

		keys := fmt.Sprintf(`{
			"keys": [
				{
					"kty": "RSA",
					"use": "sig",
					"alg": "RS512",
					"kid": "%s",
					"n": "%s",
					"e": "AQAB"
				}],
				"maxTokenTTL": 604800
		}`, testKeyID, n)

		_, err = io.WriteString(w, keys)
		if err != nil {
			_, _ = w.Write([]byte(fmt.Sprintf("failed to write out jwks response: %v", err.Error())))
		}
	})

	return mux, privateKey, nil
}

func updateClaimsWithHeaderSpecifiedClaims(req *http.Request, jwtClaims jwt.MapClaims) error {
	rawClaims := req.Header.Get("X-NAVIGA-ID-MOCK-CLAIMS")
	var claims map[string]string
	err := json.Unmarshal([]byte(rawClaims), &claims)
	if err != nil {
		return err
	}
	for k, v := range claims {
		jwtClaims[k] = v
	}
	return nil
}

func hasHeaderSpecifiedClaims(req *http.Request) bool {
	return req.Header.Get("X-NAVIGA-ID-MOCK-CLAIMS") != ""
}
