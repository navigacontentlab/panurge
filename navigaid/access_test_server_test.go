package navigaid_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/navigacontentlab/panurge/navigaid"
)

const dummyPrivatePemKey string = `-----BEGIN PRIVATE KEY-----
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
z3YIPsJ7PD4BGnEaBg0eMq72qTrD
-----END PRIVATE KEY-----`

const dummyPrivatePemKeyID string = "test-dummy-key"

type Jwks struct {
	Keys        []Jwk `json:"keys"`
	MaxTokenTTL int   `json:"maxTokenTTL"` //nolint:tagliatelle
}

type Jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type TokenResp struct {
	AccessToken string `json:"access_token"` //nolint:tagliatelle
	TokenType   string `json:"token_type"`   //nolint:tagliatelle
	ExpiresIn   int    `json:"expires_in"`   //nolint:tagliatelle
}

func TestNavigaIdMockServiceWithCustomPrivateKey(t *testing.T) {
	opts := navigaid.MockServerOptions{
		Claims: navigaid.Claims{
			Org: "sampleorg",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "75255a64-58f8-4b25-b102-af1304641096",
			},
			Permissions: navigaid.PermissionsClaim{},
		},
		PrivatePemKey:   dummyPrivatePemKey,
		PrivatePemKeyID: dummyPrivatePemKeyID,
	}

	mockServer, err := navigaid.NewMockServer(opts)

	if err != nil {
		t.Fatal(err)
	}

	server := mockServer.Server

	t.Cleanup(server.Close)

	t.Run("should use private key in /jwks", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/v1/jwks")

		defer func() {
			_ = resp.Body.Close()
		}()

		if err != nil {
			t.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)

		if err != nil {
			t.Fatal(err)
		}

		var jwks Jwks
		err = json.Unmarshal(body, &jwks)

		if err != nil {
			t.Fatal(err)
		}

		if jwks.Keys[0].Kid != dummyPrivatePemKeyID {
			t.Fatalf("specified keyId (%s) was not found in /jwks response: %v", dummyPrivatePemKeyID, jwks)
		}
	})

	t.Run("should return token signed with private key", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/v1/token")

		defer func() {
			_ = resp.Body.Close()
		}()

		if err != nil {
			t.Fatal(err)
		}

		body, err := io.ReadAll(resp.Body)

		if err != nil {
			t.Fatal(err)
		}

		var tokenResp TokenResp
		err = json.Unmarshal(body, &tokenResp)

		if err != nil {
			t.Fatal(err)
		}

		if tokenResp.AccessToken == "" {
			t.Fatalf("Got unexpected empty access-token: %s", body)
		}

		privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(dummyPrivatePemKey))
		if err != nil {
			t.Fatal(err)
		}

		_, err = jwt.Parse(tokenResp.AccessToken, func(_ *jwt.Token) (interface{}, error) {
			return &privateKey.PublicKey, nil
		})

		if err != nil {
			t.Fatal(err)
		}
	})
}
