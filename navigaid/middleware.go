package navigaid

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/twitchtv/twirp"
)

// AnnotationFunc is used to add authentication annotations to the context.
type AnnotationFunc func(ctx context.Context, organisation string, user string)

// HTTPMiddleware populates the request context with NavigaID
// authentication information. If there's an XRay segment on the
// context it will be decorated with the sub claim as the user and an
// "imid_org" annotation.
//
// It is the responsibility of the individual handlers to act on
// authentication errors by calling GetAuth() and inspecting the
// error.
func HTTPMiddleware(jwks *JWKS, next http.Handler, annotate AnnotationFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		accessToken, err := getAuthToken(r.Header)
		if err != nil {
			ctx = SetAuth(ctx, AuthInfo{}, err)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		claims, err := jwks.Validate(accessToken)
		if err != nil {
			ctx = SetAuth(ctx, AuthInfo{}, err)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		annotate(ctx, claims.Org, claims.Subject)

		ctx = SetAuth(ctx, AuthInfo{
			AccessToken: accessToken,
			Claims:      claims,
		}, nil)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NewTwirpAuthHook creates a twirp server hook that requires a valid
// NavigaID access token and adds the authentication result to the
// request context.
func NewTwirpAuthHook(log *logrus.Logger, jwks *JWKS, annotate AnnotationFunc) *twirp.ServerHooks {
	var hooks twirp.ServerHooks

	hooks.RequestRouted = func(ctx context.Context) (context.Context, error) {
		return TwirpAuthenticate(ctx, jwks, annotate)
	}

	return &hooks
}

// TwirpAuthenticate verifies that there is a valid access token and
// adds the authentication result to the request context.
func TwirpAuthenticate(ctx context.Context, jwks *JWKS, annotate AnnotationFunc) (context.Context, error) {
	headers, ok := twirp.HTTPRequestHeaders(ctx)
	if !ok {
		return ctx, twirp.NewError(twirp.Unauthenticated, "Unauthenticated")
	}

	accessToken, err := getAuthToken(headers)
	if err != nil {
		return ctx, twirp.NewError(
			twirp.Unauthenticated, "Unauthenticated")
	}

	claims, err := jwks.Validate(accessToken)
	if err != nil {
		return ctx, twirp.NewError(
			twirp.Unauthenticated, "Unauthenticated")
	}

	annotate(ctx, claims.Org, claims.Subject)

	authCtx := SetAuth(ctx, AuthInfo{
		AccessToken: accessToken,
		Claims:      claims,
	}, nil)

	return authCtx, nil
}
