package navigaid_test

import (
	"bytes"
	"context"
	"crypto/rsa"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/navigacontentlab/panurge/navigaid"
)

//nolint:funlen
func TestHTTPMiddleware(t *testing.T) {
	opts := navigaid.MockServerOptions{
		Claims: navigaid.Claims{
			Org: "sampleorg",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "75255a64-58f8-4b25-b102-af1304641096",
			},
		},
	}
	mockServer, err := navigaid.NewMockServer(opts)

	if err != nil {
		t.Fatal(err)
	}

	server := mockServer.Server
	signKey := mockServer.PrivateKey
	signKeyID := mockServer.PrivateKeyID

	t.Cleanup(server.Close)

	jwks := navigaid.NewJWKS(
		navigaid.ImasJWKSEndpoint(server.URL),
		navigaid.WithJwksClient(server.Client()),
	)

	message := []byte("** TOP SECRET **")

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		auth, err := navigaid.GetAuth(req.Context())
		if errors.As(err, &navigaid.ErrNoToken{}) {
			w.WriteHeader(http.StatusTeapot)

			return
		}

		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		if auth.Claims.Org != "hms-govt" {
			w.WriteHeader(http.StatusUnavailableForLegalReasons)

			return
		}

		access := auth.Claims.HasPermissionsInUnit(
			"mi6", "access-building", "read-files",
		)
		if !access {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		_, _ = w.Write(message)
	})

	handler := navigaid.HTTPMiddleware(jwks, apiHandler, func(_ context.Context, _, _ string) {})

	apiServer := httptest.NewServer(handler)
	t.Cleanup(apiServer.Close)

	t.Run("BondAccess", func(t *testing.T) {
		bondToken := getAccessToken(t, signKey, signKeyID, navigaid.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "hms-govt://agent/007",
				ExpiresAt: &jwt.NumericDate{Time: time.Now().AddDate(2, 0, 0)},
			},
			Org: "hms-govt",
			Permissions: navigaid.PermissionsClaim{
				Org: []string{"permission-to-kill"},
				Units: map[string][]string{
					"mi6": {"access-building", "read-files", "q-equipment"},
				},
			},
		})

		res := getWithToken(t, apiServer.Client(), apiServer.URL, bondToken)

		defer func() {
			_ = res.Body.Close()
		}()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("server responded with: %s", res.Status)
		}

		recievedMsg, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}

		if !bytes.Equal(recievedMsg, message) {
			t.Fatalf("wrong message received, want %q, got %q",
				string(message), string(recievedMsg))
		}
	})

	t.Run("CleanerAccess", func(t *testing.T) {
		token := getAccessToken(t, signKey, signKeyID, navigaid.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "hms-govt://cleaner/101",
				ExpiresAt: &jwt.NumericDate{Time: time.Now().AddDate(2, 0, 0)},
			},
			Org: "hms-govt",
			Permissions: navigaid.PermissionsClaim{
				Org: []string{"permission-to-clean"},
				Units: map[string][]string{
					"mi6": {"access-building"},
				},
			},
		})

		res := getWithToken(t, apiServer.Client(), apiServer.URL, token)

		defer func() {
			_ = res.Body.Close()
		}()

		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("expected resource to be forbidden, server responded with: %s", res.Status)
		}
	})

	t.Run("MooreAccess", func(t *testing.T) {
		bondToken := getAccessToken(t, signKey, signKeyID, navigaid.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "hms-govt://agent/007/roger-moore",
				ExpiresAt: &jwt.NumericDate{Time: time.Date(1985, time.May, 23, 0, 0, 0, 0, time.UTC)},
			},
			Org: "hms-govt",
			Permissions: navigaid.PermissionsClaim{
				Org: []string{"permission-to-kill"},
				Units: map[string][]string{
					"mi6": {"access-building", "read-files", "q-equipment"},
				},
			},
		})

		res := getWithToken(t, apiServer.Client(), apiServer.URL, bondToken)
		defer func() {
			_ = res.Body.Close()
		}()

		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized status, server responded with: %s", res.Status)
		}
	})

	t.Run("RandomCrackpot", func(t *testing.T) {
		res, err := http.Get(apiServer.URL)
		if err != nil {
			t.Fatalf("failed to perform request: %v", err)
		}

		err = res.Body.Close()
		if err != nil {
			t.Fatalf("failed close response body: %v", err)
		}

		if res.StatusCode != http.StatusTeapot {
			t.Fatalf("expected teapot status, got %s", res.Status)
		}
	})
}

func getWithToken(t *testing.T, client *http.Client, url string, token string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("failed to perform request: %v", err)
	}

	t.Cleanup(func() {
		err := res.Body.Close()
		if err != nil {
			t.Errorf("failed to close response body: %v", err)
		}
	})

	return res
}

func getAccessToken(t *testing.T, signKey *rsa.PrivateKey, keyID string, claims navigaid.Claims) string {
	t.Helper()

	claims.TokenType = navigaid.TokenTypeAccessToken

	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	token.Header["kid"] = keyID

	accessToken, err := token.SignedString(signKey)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	return accessToken
}
