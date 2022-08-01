package navigaid

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// ImasJWKSEndpoint is a helper function that returns the v1 token
// endpoint URL given an URL that points to the access token service.
func AccessTokenEndpoint(serviceURL string) string {
	return fmt.Sprintf("%s/v1/token", strings.TrimSuffix(serviceURL, "/"))
}

type AccessTokenServiceOption func(ats *AccessTokenService)

// WithAccessTokenClient sets the HTTP client that should be used for
// access token requests.
func WithAccessTokenClient(client *http.Client) AccessTokenServiceOption {
	return func(ats *AccessTokenService) {
		ats.client = client
	}
}

// AccessTokenService can validate access tokens and create access tokens from
// naviga-id tokens.
type AccessTokenService struct {
	client        *http.Client
	tokenEndpoint string
}

// New creates a new access token service with given options.
func New(tokenEndpoint string, options ...AccessTokenServiceOption) *AccessTokenService {
	ats := AccessTokenService{
		tokenEndpoint: tokenEndpoint,
	}

	for _, o := range options {
		o(&ats)
	}

	if ats.client == nil {
		ats.client = http.DefaultClient
	}

	return &ats
}

// AccessTokenResponse is the response retrieved from navigaID.
type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewAccessToken takes an navigaID token and returns an access token.
func (ats *AccessTokenService) NewAccessToken(navigaIDToken string) (*AccessTokenResponse, error) {
	req, err := http.NewRequest("POST", ats.tokenEndpoint, strings.NewReader(""))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+navigaIDToken)
	res, err := ats.client.Do(req)
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var atr AccessTokenResponse
	err = json.Unmarshal(bytes, &atr)
	if err != nil {
		return nil, err
	}

	return &atr, nil
}

// ErrNoToken is used to communicate that no bearer token was included
// in the request.
type ErrNoToken struct{}

func (err ErrNoToken) Error() string {
	return "no token found"
}

func getAuthToken(header http.Header) (string, error) {
	auth := header.Get("Authorization")

	authType, token, _ := strings.Cut(auth, " ")
	if token == "" {
		return "", ErrNoToken{}
	}

	if strings.ToLower(authType) != "bearer" {
		return "", ErrNoToken{}
	}

	return token, nil
}
