package navigaid_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/navigacontentlab/panurge/navigaid"
)

func TestAccessTokenService(t *testing.T) {
	expectedTokenTTL := 600
	opts := navigaid.MockServerOptions{
		Claims: navigaid.Claims{
			Org: "sampleorg",
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "75255a64-58f8-4b25-b102-af1304641096",
			},
		},
		TTL: expectedTokenTTL,
	}
	mockServer, err := navigaid.NewMockServer(opts)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(mockServer.Server.Close)

	service := navigaid.New(
		navigaid.AccessTokenEndpoint(mockServer.Server.URL),
		navigaid.WithAccessTokenClient(mockServer.Client),
	)

	jwks := navigaid.NewJWKS(
		navigaid.ImasJWKSEndpoint(mockServer.Server.URL),
		navigaid.WithJwksClient(mockServer.Client),
	)

	// Test creating and then validing an access token
	resp, err := service.NewAccessToken("testNavigaIDToken")
	if err != nil {
		t.Fatalf("failed to exchange ID token for an access token: %v", err)
	}

	_, err = jwks.Validate(resp.AccessToken)
	if err != nil {
		t.Errorf("expected token to be valid, was invalid: %v", err)
	}

	var claims jwt.RegisteredClaims

	_, _, err = new(jwt.Parser).ParseUnverified(resp.AccessToken, &claims)

	if err != nil {
		t.Errorf("failed to parse token")
	}

	fmt.Printf("claims: %v\n", claims) //nolint:forbidigo
	actualTokenTTL := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)

	if actualTokenTTL.Seconds() != float64(expectedTokenTTL) {
		t.Errorf("expected token TTL to be %f but got %f", float64(expectedTokenTTL), actualTokenTTL.Seconds())
	}

	if actualTokenTTL.Seconds() != float64(resp.ExpiresIn) {
		t.Errorf("expected token TTL (%d) "+
			"to match token response ExpiresIn (%d)", actualTokenTTL, resp.ExpiresIn)
	}

	if expectedTokenTTL != resp.ExpiresIn {
		t.Errorf("expected configured token TTL (%d) "+
			"to match token response ExpiresIn (%d)", expectedTokenTTL, resp.ExpiresIn)
	}

	// Test validating an invalid access token
	//nolint:gosec,lll
	invalidToken := "eyJhbGciOiJSUzUxMiIsImtpZCI6ImEzNGRiODVhLTNmNjctNDJlMC05NGY2LTE3Njk0ZmM4NWZkOSIsInR5cCI6IkpXVCJ9.eyJleHAiOjE1ODYyNTAwNTAsImlhdCI6MTU4NjI0OTc1MCwianRpIjoiZGEyMGRkYTQtYzhjZS00ZGFjLTk4ZGMtNDM1ZjJmMDEyOGYxIiwibnR0IjoiYWNjZXNzX3Rva2VuIiwib3JnIjoic2FtcGxlb3JnIiwic3ViIjoiNzUyNTVhNjQtNThmOC00YjI1LWIxMDItYWYxMzA0NjQxMDk2In0.GHCuL6SU2T2_cZQMJhCPpYcqPQxascYgjgCIZQuUFinNSUeBegDKLWvkQMSu6huK8JPq7klftQ7CbK5Lc6jREQsgsXOoW6xmQO2xKUF04ugjWEKtgWZaCmx23uyNRy77B-S-pIpRbC5pDoyxORGqb5r19EnrXAbWNGtgRxYZqSBK6AO-9QFgaxthZDcJ9y5GMCAkFbGgc8PEPYiPKZ9C0MyIlAmth1NU6601w0SDfGsUkkbbOQF-F7wZYkuLspbgZuRqLNLBJPI8VNPfquZlUVhr1ZWq9lEvXhLlrVQbw6BsD7RmrmPJHqjMkmyClLyZv2pWa3Z9nXxVZBOctStkHg"
	_, err = jwks.Validate(invalidToken)

	if err == nil {
		t.Errorf("expected token to be invalid but was valid")
	}

	t.Run("SelfSigned", func(t *testing.T) {
		// Test validating a self-signed token
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("failed to generate key: %v", err)
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS512, jwt.MapClaims{
			"sub": "75255a64-58f8-4b25-b102-af1304641096",
			"org": "sampleorg",
			"ntt": "access_token",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
			"iat": time.Now().Unix(),
			"jti": "da20dda4-c8ce-4dac-98dc-435f2f0128f1",
		})

		token.Header["kid"] = "dummy-kid"

		signed, err := token.SignedString(privateKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}

		_, err = jwks.Validate(signed)
		if err == nil {
			t.Fatalf("expected token to be invalid but was valid")
		}

		t.Logf("got an error as expected: %v", err)
	})
}

func ExampleAccessTokenService() {
	service := navigaid.New(
		"https://access-token.stage.imid.infomaker.io/v1/token",
	)

	jwks := navigaid.NewJWKS(
		navigaid.ImasJWKSEndpoint("https://imas.stage.imid.infomaker.io"),
	)

	navigaIDToken := "..."

	at, err := service.NewAccessToken(navigaIDToken)
	if err != nil {
		fmt.Println(err)
	}

	claims, err := jwks.Validate(at.AccessToken)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("%#v\n", claims)
}
