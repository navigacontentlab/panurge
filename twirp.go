package panurge

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/navigacontentlab/panurge/lambda"
	"github.com/navigacontentlab/panurge/navigaid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twitchtv/twirp"
	"golang.org/x/sync/errgroup"
)

// StandardApp provides a framework for setting up our applications in
// a consistent way.
type StandardApp struct {
	port         int
	internalPort int
	services     map[string]NewServiceFunc
	authHook     *twirp.ServerHooks
	authOrg      func(ctx context.Context) string
	imasURL      string
	healthcheck  HealthcheckFunc
	version      string
	name         string
	cors         CORSOptions
	testServers  *TestServers
	metricsOpts  []TwirpMetricOptionFunc
	logger       *slog.Logger

	internalServer *http.Server

	Server *http.Server
	Mux    *http.ServeMux
}

type NewServiceFunc func(hooks *twirp.ServerHooks) http.Handler

type StandardAppOption func(app *StandardApp)

// WithAppAuth hook is used to add "legacy" authentication methods.
func WithAppAuthHook(
	authHook *twirp.ServerHooks,
	authOrg func(ctx context.Context) string,
) StandardAppOption {
	return func(app *StandardApp) {
		app.authHook = authHook
		app.authOrg = authOrg
	}
}

// WithImasURL configures the application to fetch JWKs from IMAS and
// verify incoming bearer access tokens for twirp APIs.
func WithImasURL(imasURL string) StandardAppOption {
	return func(app *StandardApp) {
		app.imasURL = imasURL
	}
}

// WithAppService exposes a Twirp service.
func WithAppService(pathPrefix string, fn NewServiceFunc) StandardAppOption {
	return func(app *StandardApp) {
		app.services[pathPrefix] = fn
	}
}

// WithAppHealthCheck provides a custom function that evaluates the
// health of the application.
func WithAppHealthCheck(check HealthcheckFunc) StandardAppOption {
	return func(app *StandardApp) {
		app.healthcheck = check
	}
}

// WithAppPorts sets the public and internal listener ports.
func WithAppPorts(public, internal int) StandardAppOption {
	return func(app *StandardApp) {
		app.port = public
		app.internalPort = internal
	}
}

// WithAppVersion sets the application version for reporting purposes.
func WithAppVersion(version string) StandardAppOption {
	return func(app *StandardApp) {
		app.version = version
	}
}

// WithTwirpCORSOptions customise the cors options for the Twirp
// services.
func WithTwirpCORSOptions(opts CORSOptions) StandardAppOption {
	return func(app *StandardApp) {
		app.cors = opts
	}
}

// WithTwirpMetricsOptions changes the metric collection behaviours.
func WithTwirpMetricsOptions(opts ...TwirpMetricOptionFunc) StandardAppOption {
	return func(app *StandardApp) {
		app.metricsOpts = opts
	}
}

// NewStandardApp creates a standard panurge Twirp application.
func NewStandardApp(
	logger *slog.Logger, name string, opts ...StandardAppOption,
) (*StandardApp, error) {
	app := StandardApp{
		healthcheck:  NoopHealthcheck,
		port:         8081,
		internalPort: 8090,
		services:     map[string]NewServiceFunc{},
		name:         name,
		version:      "dev",
		logger:       logger,
	}

	for i := range opts {
		opts[i](&app)
	}

	mux := http.NewServeMux()

	if len(app.services) > 0 {
		cors := NewCORSMiddleware(app.cors)

		twirpHooks, err := StandardTwirpHooks(logger, TwirpHookOptions{
			AuthHook:       app.authHook,
			MetricsOptions: app.metricsOpts,
			ImasURL:        app.imasURL,
		})
		if err != nil {
			return nil, err
		}

		for prefix, newFunc := range app.services {
			handler := newFunc(twirpHooks)

			mux.Handle(prefix, AddTwirpRequestHeaders(
				cors.Handler(handler),
				"Authorization", "x-imid-token",
			))
		}
	}

	ConfigureXRay(logger, app.version)

	internalMux := StandardInternalMux(logger, app.healthcheck)
	instrumentedHandler := xray.Handler(
		xray.NewFixedSegmentNamer(app.name),
		AnnotationMiddleware(mux),
	)

	app.Mux = mux

	if app.testServers != nil {
		app.testServers.public = httptest.NewServer(instrumentedHandler)
		app.testServers.internal = httptest.NewServer(internalMux)
	}

	app.Server = StandardServer(app.port, instrumentedHandler)
	app.internalServer = StandardServer(app.internalPort, internalMux)

	return &app, nil
}

// ListenAndServe starts both the internal and external servers. If
// the application was configured with test servers this function will
// return once they have been set up, otherwise it will block as long
// as the servers are listening.
func (app *StandardApp) ListenAndServe() error {
	if app.testServers != nil {
		return nil
	}

	var grp errgroup.Group

	grp.Go(app.Server.ListenAndServe)
	grp.Go(app.internalServer.ListenAndServe)

	err := grp.Wait()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// LambdaHandler creates an HTTP event handler (Loadbalancer/APIGateway) that proxies requests to the
// application ServeMux.
func (app *StandardApp) LambdaHandler() lambda.HandlerFunc {
	return lambda.Handler(app.Server.Handler, app.logger)
}

// TwirpHookOptions controls the configuration of the standard twirp
// hooks.
type TwirpHookOptions struct {
	AuthHook       *twirp.ServerHooks
	ImasURL        string
	MetricsOptions []TwirpMetricOptionFunc
}

// StandardTwirpHooks sets up the standard twirp server hooks for
// metrics, authentication, and error logging.
func StandardTwirpHooks(
	logger *slog.Logger, opts TwirpHookOptions,
) (*twirp.ServerHooks, error) {
	var auth *twirp.ServerHooks

	metrics, err := NewTwirpMetricsHooks(opts.MetricsOptions...)
	if err != nil {
		return nil, err
	}

	if opts.AuthHook != nil {
		auth = opts.AuthHook
	} else if opts.ImasURL != "" {
		svc := navigaid.NewJWKS(
			navigaid.ImasJWKSEndpoint(opts.ImasURL),
		)

		auth = navigaid.NewTwirpAuthHook(logger, svc, func(ctx context.Context, org string, user string) {
			AddUserAnnotation(ctx, user)
			AddAnnotation(ctx, "imid_org", org)
		})
	}

	hooks := metrics

	if auth != nil {
		hooks = CombineMetricsAndAuthHooks(metrics, auth)
	}

	hooks = twirp.ChainHooks(hooks, NewErrorLoggingHooks(logger))

	return hooks, nil
}

// NewErrorLoggingHooks will log outgoing error responses. XRay
// annotations should be logged together with the error, so we do not
// add information about the method and service here.
func NewErrorLoggingHooks(logger *slog.Logger) *twirp.ServerHooks {
	return &twirp.ServerHooks{
		Error: func(ctx context.Context, err twirp.Error) context.Context {
			var attr []slog.Attr
			attr = append(attr, slog.Int("status_code", twirp.ServerHTTPStatusFromErrorCode(err.Code())))
			attr = append(attr, slog.Any("twirp_code", err.Code()))
			attr = append(attr, slog.String("twirp_msg", err.Msg()))

			if err.MetaMap() != nil {
				attr = append(attr, slog.Any("twirp_meta", err.MetaMap()))
			}

			args := make([]any, 0, len(attr)*2)
			for _, a := range attr {
				args = append(args, a.Key, a.Value.Any())
			}

			logger.ErrorContext(ctx, "error response", args...)

			return ctx
		},
	}
}

// CombineMetricsAndAuthHooks tweaks how the hooks are chained so that
// the metrics.RequestRouted always is called regardless of auth
// errors. An auth error will still fail the request, but any errors
// returned by the metrics hook will be ignored.
func CombineMetricsAndAuthHooks(metrics, auth *twirp.ServerHooks) *twirp.ServerHooks {
	chained := twirp.ChainHooks(metrics, auth)

	chained.RequestRouted = func(ctx context.Context) (context.Context, error) {
		var err error
		if auth.RequestRouted != nil {
			ctx, err = auth.RequestRouted(ctx)
		}

		if metrics.RequestRouted != nil {
			if mCtx, mErr := metrics.RequestRouted(ctx); mErr != nil {
				ctx = mCtx
			}
		}

		if err != nil {
			err = fmt.Errorf("%w", err)
		}

		return ctx, err
	}

	return chained
}

type TwirpMetricsOptions struct {
	reg         prometheus.Registerer
	testLatency time.Duration
	contextOrg  func(ctx context.Context) string
}

type TwirpMetricOptionFunc func(opts *TwirpMetricsOptions)

// WithTwirpMetricsOrgFunction uses a custom function for resolving
// the current organisation from the context.
func WithTwirpMetricsOrgFunction(fn func(ctx context.Context) string) TwirpMetricOptionFunc {
	return func(opts *TwirpMetricsOptions) {
		opts.contextOrg = fn
	}
}

// WithTwirpMetricsRegisterer uses a custom registerer for Twirp metrics.
func WithTwirpMetricsRegisterer(reg prometheus.Registerer) TwirpMetricOptionFunc {
	return func(opts *TwirpMetricsOptions) {
		opts.reg = reg
	}
}

// WithTwirpMetricsStaticTestLatency configures the RPC metrics to report
// a static duration.
func WithTwirpMetricsStaticTestLatency(latency time.Duration) TwirpMetricOptionFunc {
	return func(opts *TwirpMetricsOptions) {
		opts.testLatency = latency
	}
}

// NewTwirpMetricsHooks creates new twirp hooks enabling prometheus metrics.
func NewTwirpMetricsHooks(opts ...TwirpMetricOptionFunc) (*twirp.ServerHooks, error) {
	opt := TwirpMetricsOptions{
		reg: prometheus.DefaultRegisterer,
		contextOrg: func(ctx context.Context) string {
			info, err := navigaid.GetAuth(ctx)
			if err != nil {
				return ""
			}

			return info.Claims.Org
		},
	}

	for i := range opts {
		opts[i](&opt)
	}

	requestsReceived := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_requests_total",
			Help: "Number of RPC requests received.",
		},
		[]string{"service", "method", "organisation"},
	)
	if err := opt.reg.Register(requestsReceived); err != nil {
		return nil, fmt.Errorf("failed to register metric: %w", err)
	}

	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "rpc_duration",
		Help:    "Duration for a rpc call.",
		Buckets: prometheus.ExponentialBuckets(5, 2, 15),
	}, []string{"service", "method", "organisation"})
	if err := opt.reg.Register(duration); err != nil {
		return nil, fmt.Errorf("failed to register metric: %w", err)
	}

	responsesSent := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rpc_responses_total",
			Help: "Number of RPC responses sent.",
		},
		[]string{"service", "method", "status", "organisation"},
	)
	if err := opt.reg.Register(responsesSent); err != nil {
		return nil, fmt.Errorf("failed to register metric: %w", err)
	}

	var hooks twirp.ServerHooks

	var reqStartTimestampKey = new(int)

	hooks.RequestReceived = func(ctx context.Context) (context.Context, error) {
		return context.WithValue(ctx, reqStartTimestampKey, time.Now()), nil
	}

	hooks.ResponseSent = func(ctx context.Context) {
		serviceName, sOk := twirp.ServiceName(ctx)
		method, mOk := twirp.MethodName(ctx)

		if !mOk || !sOk {
			return
		}

		organisation := opt.contextOrg(ctx)
		status, _ := twirp.StatusCode(ctx)

		responsesSent.WithLabelValues(
			serviceName, method, status, organisation,
		).Inc()

		if start, ok := ctx.Value(reqStartTimestampKey).(time.Time); ok {
			dur := time.Since(start).Seconds() // 100ms = 0.1 sek

			if opt.testLatency != 0 {
				dur = opt.testLatency.Seconds()
			}

			duration.WithLabelValues(
				serviceName, method, organisation,
			).Observe(dur * 1000)
		}
	}

	hooks.RequestRouted = func(ctx context.Context) (context.Context, error) {
		serviceName, sOk := twirp.ServiceName(ctx)
		method, mOk := twirp.MethodName(ctx)

		if !(sOk && mOk) {
			return ctx, nil
		}

		organisation := opt.contextOrg(ctx)

		seg := xray.GetSegment(ctx)
		if seg != nil {
			_ = seg.AddAnnotation("twirp_service", serviceName)
			_ = seg.AddAnnotation("twirp_method", method)
		}

		requestsReceived.WithLabelValues(
			serviceName, method, organisation,
		).Inc()

		return ctx, nil
	}

	return &hooks, nil
}

// AddTwirpRequestHeaders is a middleware that adds HTTP request
// headers to the context for twirp to consume.
func AddTwirpRequestHeaders(next http.Handler, names ...string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := make(http.Header)
		for i := range names {
			header.Set(names[i], r.Header.Get(names[i]))
		}

		ctx, _ := twirp.WithHTTPRequestHeaders(r.Context(), header)

		request := r.WithContext(ctx)

		next.ServeHTTP(w, request)
	})
}
