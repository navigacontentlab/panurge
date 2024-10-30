package navigaid

import (
	"context"
	"errors"
)

type contextKey int

// authInfoKey is used to retrieve the access token.
const authInfoKey = contextKey(iota)

type AuthInfo struct {
	AccessToken string
	Claims      Claims
}

type ai struct {
	Ac  AuthInfo
	Err error
}

// GetAutch retrieves authentication information from the context.
func GetAuth(ctx context.Context) (AuthInfo, error) {
	auth, ok := ctx.Value(authInfoKey).(ai)
	if !ok {
		return AuthInfo{}, errors.New("no authentication information in context")
	}

	if auth.Err != nil {
		return AuthInfo{}, auth.Err
	}

	return auth.Ac, nil
}

// SetClaims adds specified Claims to the context.
func SetAuth(ctx context.Context, auth AuthInfo, err error) context.Context {
	return context.WithValue(ctx, authInfoKey, ai{
		Ac:  auth,
		Err: err,
	})
}
