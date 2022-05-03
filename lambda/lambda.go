package lambda

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/sirupsen/logrus"
)

type RequestContext struct {
	events.ALBTargetGroupRequestContext
	events.APIGatewayV2HTTPRequestContext
}

// LambdaRequest wraps ALBTargetGroupRequest and APIGatewayV2HTTPRequest
// into a generic request struct
type LambdaRequest struct {
	events.ALBTargetGroupRequest   //nolint // ambiguous selectors declared below
	events.APIGatewayV2HTTPRequest //nolint // ambiguous selectors declared below

	// Added to resolve "ambiguous selectors" error
	Headers               map[string]string `json:"headers"`
	QueryStringParameters map[string]string `json:"queryStringParameters"`
	RequestContext        RequestContext    `json:"requestContext"`
	Body                  string            `json:"body"`
	IsBase64Encoded       bool              `json:"isBase64Encoded"`
}

// LambdaRequest mimics ALBTargetGroupResponse and APIGatewayV2HTTPResponse
// into a generic response struct
type LambdaResponse struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded"`
	Cookies           []string            `json:"cookies"`
}

// LambdaHandlerFunc is a generic LambdaHandlerFunc for incoming http requests/events
// incoming request will be run through http server which will return the response
type LambdaHandlerFunc func(
	ctx context.Context, event LambdaRequest,
) (LambdaResponse, error)

func LambdaHandler(handler http.Handler, logger *logrus.Logger) LambdaHandlerFunc {
	return func(ctx context.Context, event LambdaRequest) (LambdaResponse, error) {
		req, err := AWSRequestToHTTPRequest(ctx, event)

		logger.WithFields(logrus.Fields{
			"Method":  req.Method,
			"host":    req.Host,
			"URI":     req.RequestURI,
			"Headers": req.Header,
		}).Debug("GeneratedHTTPRequest")

		if err != nil {
			logger.WithError(err).Error("failed to convert event to request")
			return LambdaResponse{}, fmt.Errorf(
				"failed to convert event to a request: %w", err)
		}

		w := NewProxyResponseWriter()

		handler.ServeHTTP(w, req)

		return w.GetLambdaResponse()
	}
}

func AWSRequestToHTTPRequest(ctx context.Context, event LambdaRequest) (*http.Request, error) {
	HTTPMethod := event.HTTPMethod
	if event.Version == "2.0" {
		HTTPMethod = event.RequestContext.HTTP.Method
	}

	params := url.Values{}
	for k, v := range event.QueryStringParameters {
		params.Set(k, v)
	}
	for k, vals := range event.MultiValueQueryStringParameters {
		for _, v := range vals {
			params.Add(k, v)
		}
	}

	headers := make(http.Header)
	for k, v := range event.Headers {
		headers.Set(k, v)
	}
	for k, vals := range event.MultiValueHeaders {
		for _, v := range vals {
			headers.Add(k, v)
		}
	}

	u := url.URL{
		Host:     headers.Get("Host"),
		RawPath:  event.Path,
		RawQuery: params.Encode(),
	}
	if event.Version == "2.0" {
		u.RawPath = event.RawPath
		u.RawQuery = event.RawQueryString
	}

	p, err := url.PathUnescape(u.RawPath)
	if err != nil {
		return nil, err
	}
	u.Path = p

	if u.Path == u.RawPath {
		u.RawPath = ""
	}

	var body io.Reader = strings.NewReader(event.Body)
	if event.IsBase64Encoded {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}

	req, err := http.NewRequest(HTTPMethod, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("could not convert to request: %w", err)
	}

	req.RequestURI = u.RequestURI()
	req.Header = headers

	return req.WithContext(ctx), nil
}
