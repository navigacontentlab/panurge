package panurge_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-xray-sdk-go/strategy/sampling"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/dgrijalva/jwt-go"
	"github.com/navigacontentlab/panurge"
	"github.com/navigacontentlab/panurge/internal/rpc/testservice"
	"github.com/navigacontentlab/panurge/navigaid"
	"github.com/navigacontentlab/panurge/pt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/twitchtv/twirp"
	"golang.org/x/oauth2"
)

func TestServers__MetricsAndTracing(t *testing.T) {
	var testServers panurge.TestServers

	logger := panurge.Logger("warning")
	logger.Out = pt.NewTestLogWriter(t)

	opts := navigaid.MockServerOptions{
		Claims: navigaid.Claims{
			Org: "testorg",
			StandardClaims: jwt.StandardClaims{
				Subject: "75255a64-58f8-4b25-b102-af1304641096",
			},
		},
	}

	mockServer, err := navigaid.NewMockServer(opts)
	pt.Must(t, err, "failed to create NavigaID mock server")

	t.Cleanup(mockServer.Server.Close)

	service := navigaid.New(
		navigaid.AccessTokenEndpoint(mockServer.Server.URL),
		navigaid.WithAccessTokenClient(mockServer.Client),
	)

	reg := prometheus.NewPedanticRegistry()

	_, err = panurge.NewStandardApp(logger, "testservice",
		panurge.WithAppVersion("v0.0.0"),
		panurge.WithAppTestServers(&testServers),
		panurge.WithImasURL(mockServer.Server.URL),
		panurge.WithTwirpMetricsOptions(
			panurge.WithTwirpMetricsRegisterer(reg),
			panurge.WithTwirpMetricsStaticTestLatency(1*time.Second),
		),
		panurge.WithAppService(
			testservice.TestPathPrefix,
			func(hooks *twirp.ServerHooks) http.Handler {
				return testservice.NewTestServer(&Greeter{}, hooks)
			},
		),
	)
	if err != nil {
		t.Fatalf("failed to create test application: %v", err)
	}

	t.Cleanup(func() {
		testServers.Close()
	})

	server := testServers.GetPublic()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Unauthorized client to get a fail
	uClient := testservice.NewTestJSONClient(server.URL, server.Client())

	_, err = uClient.DoThing(ctx, &testservice.ThingReq{
		Name: "Who cares",
	})
	if err == nil {
		t.Error("expected unauthorised call to fail")
	}

	var tErr twirp.Error
	if !errors.As(err, &tErr) {
		t.Error("expected to get a Twirp error back")
	} else if tErr.Code() != twirp.Unauthenticated {
		t.Errorf("expected to get a %q error, got %q",
			twirp.Unauthenticated, tErr.Code())
	}

	tok, err := service.NewAccessToken("testNavigaIDToken")
	pt.Must(t, err, "failed to create test token")

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: tok.AccessToken,
	})

	httpClient := oauth2.NewClient(ctx, tokenSource)

	client := testservice.NewTestProtobufClient(server.URL, httpClient)

	err = xray.Configure(xray.Config{
		SamplingStrategy: SamplingStrategy(true),
		Emitter:          DummyEmitter{},
	})
	pt.Must(t, err, "failed to configure XRay to sample all requests")

	res, err := client.DoThing(ctx, &testservice.ThingReq{
		Name: "Horatio Hornblower",
	})

	if err != nil {
		t.Fatalf("got error response: %v", err)
	}

	want := "Hello Horatio Hornblower!"
	if res.Response != want {
		t.Errorf("got %q, want %q", res.Response, want)
	}

	wantMetrics := strings.NewReader(`
# HELP rpc_duration Duration for a rpc call.
# TYPE rpc_duration histogram
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="5"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="10"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="20"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="40"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="80"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="160"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="320"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="640"} 0
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="1280"} 1
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="2560"} 1
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="5120"} 1
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="10240"} 1
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="20480"} 1
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="40960"} 1
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="81920"} 1
rpc_duration_bucket{method="DoThing",organisation="",service="Test",le="+Inf"} 1
rpc_duration_sum{method="DoThing",organisation="",service="Test"} 1000
rpc_duration_count{method="DoThing",organisation="",service="Test"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="5"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="10"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="20"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="40"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="80"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="160"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="320"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="640"} 0
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="1280"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="2560"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="5120"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="10240"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="20480"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="40960"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="81920"} 1
rpc_duration_bucket{method="DoThing",organisation="testorg",service="Test",le="+Inf"} 1
rpc_duration_sum{method="DoThing",organisation="testorg",service="Test"} 1000
rpc_duration_count{method="DoThing",organisation="testorg",service="Test"} 1
# HELP rpc_requests_total Number of RPC requests received.
# TYPE rpc_requests_total counter
rpc_requests_total{method="DoThing",organisation="",service="Test"} 1
rpc_requests_total{method="DoThing",organisation="testorg",service="Test"} 1
# HELP rpc_responses_total Number of RPC responses sent.
# TYPE rpc_responses_total counter
rpc_responses_total{method="DoThing",organisation="",service="Test",status="401"} 1
rpc_responses_total{method="DoThing",organisation="testorg",service="Test",status="200"} 1
`)

	err = testutil.GatherAndCompare(reg, wantMetrics,
		"rpc_duration", "rpc_requests_total", "rpc_responses_total")
	if err != nil {
		t.Errorf("didn't gather the expected metrics: %v", err)
	}

	err = xray.Configure(xray.Config{
		SamplingStrategy: SamplingStrategy(false),
	})
	pt.Must(t, err, "failed to configure XRay to not sample any requests")

	resTwo, err := client.DoThing(ctx, &testservice.ThingReq{
		Name: "Slughorn",
	})

	if err != nil {
		t.Fatalf("got error response: %v", err)
	}

	wantTwo := "Hello Slughorn!"
	if resTwo.Response != wantTwo {
		t.Errorf("got %q, want %q", resTwo.Response, wantTwo)
	}
}

type Greeter struct{}

func (g *Greeter) DoThing(ctx context.Context, in *testservice.ThingReq) (*testservice.ThingRes, error) {
	name := "John Doe"
	if in.Name != "" {
		name = in.Name
	}

	ann := panurge.GetContextAnnotations(ctx)
	if ann == nil {
		return nil, errors.New("missing context annotations")
	}

	auth, err := navigaid.GetAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth context: %w", err)
	}

	if ann.GetUser() != auth.Claims.Subject {
		return nil, errors.New("missing user annotation")
	}

	annotations := ann.GetAnnotations()

	segOrg, ok := annotations["imid_org"].(string)
	if !ok {
		return nil, errors.New("missing organisation annotation")
	}

	if segOrg != auth.Claims.Org {
		return nil, fmt.Errorf("wrong organisation annotation, want %q, got %q",
			segOrg, "foo")
	}

	return &testservice.ThingRes{
		Response: "Hello " + name + "!",
	}, nil
}

type SamplingStrategy bool

func (s SamplingStrategy) ShouldTrace(request *sampling.Request) *sampling.Decision {
	return &sampling.Decision{
		Sample: bool(s),
		Rule:   aws.String("dummy strategy"),
	}
}

type DummyEmitter struct{}

func (e DummyEmitter) Emit(seg *xray.Segment) {
}

func (e DummyEmitter) RefreshEmitterWithAddress(raddr *net.UDPAddr) {
}
