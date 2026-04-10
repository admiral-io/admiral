package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/uber-go/tally/v4"
	tallyprom "github.com/uber-go/tally/v4/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/gateway/meta"
	"go.admiral.io/admiral/internal/gateway/mux"
	"go.admiral.io/admiral/internal/gateway/stats"
	"go.admiral.io/admiral/internal/logger"
	"go.admiral.io/admiral/internal/middleware"
	"go.admiral.io/admiral/internal/middleware/accesslog"
	"go.admiral.io/admiral/internal/middleware/errorintercept"
	"go.admiral.io/admiral/internal/middleware/timeouts"
	"go.admiral.io/admiral/internal/service"
)

type ComponentFactory struct {
	Services   service.Factory
	Middleware middleware.Factory
	Endpoints  endpoint.Factory
}

func Run(cfg *config.Config, cf *ComponentFactory, assets http.FileSystem) error { //nolint:revive
	log, err := logger.New(cfg.Server.Logger, "server")
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	// Initialize metrics scope and optional Prometheus HTTP handler.
	scopeOpts, metricsHandler := getStatsReporterConfiguration(cfg, log)
	scope, scopeCloser := tally.NewRootScope(
		scopeOpts,
		cfg.Server.Stats.FlushInterval,
	)
	defer func() {
		if err := scopeCloser.Close(); err != nil {
			panic(err)
		}
	}()
	initScope := scope.SubScope("gateway")

	// Create the error interceptor so services can register error interceptors if desired.
	errorInterceptMiddleware, err := errorintercept.NewMiddleware(nil, log, initScope)
	if err != nil {
		return fmt.Errorf("could not create error interceptor middleware: %w", err)
	}

	// Instantiate and register services.
	for _, entry := range cf.Services {
		log := log.With(zap.String("serviceName", entry.Name))
		log.Info("registering service")

		svc, err := entry.Factory(cfg, log, scope.SubScope(entry.Name))
		if err != nil {
			return fmt.Errorf("service %s instantiation failed: %w", entry.Name, err)
		}
		service.Registry[entry.Name] = svc

		if ei, ok := svc.(errorintercept.Interceptor); ok {
			log.Info("service registered an error conversion interceptor")
			errorInterceptMiddleware.AddInterceptor(ei.InterceptError)
		}
	}

	// Error interceptors should be first on the stack (last in chain).
	var interceptors []grpc.UnaryServerInterceptor
	interceptors = append(interceptors, errorInterceptMiddleware.UnaryInterceptor())

	// Access log.
	if cfg.Server.AccessLog != nil {
		a, err := accesslog.New(cfg.Server.AccessLog, log, scope)
		if err != nil {
			return fmt.Errorf("could not create accesslog interceptor: %w", err)
		}
		interceptors = append(interceptors, a.UnaryInterceptor())
	}

	// Create the timeout interceptor.
	timeoutInterceptor, err := timeouts.New(&cfg.Server.Timeouts, log, scope)
	if err != nil {
		return fmt.Errorf("could not create timeout interceptor: %w", err)
	}
	interceptors = append(interceptors, timeoutInterceptor.UnaryInterceptor())

	// Instantiate other configured middleware (order is preserved).
	for _, entry := range cf.Middleware {
		log := log.With(zap.String("middlewareName", entry.Name))
		log.Info("registering middleware")

		mid, err := entry.Factory(cfg, log, scope.SubScope(entry.Name))
		if err != nil {
			return fmt.Errorf("middleware %s instantiation failed: %w", entry.Name, err)
		}

		interceptors = append(interceptors, mid.UnaryInterceptor())
	}

	// Instantiate and register modules listed in the configuration.
	rpcMux, err := mux.New(interceptors, assets, metricsHandler, *cfg.Server)
	if err != nil {
		return fmt.Errorf("failed to create mux: %w", err)
	}
	ctx := context.TODO()

	// Create a client connection for the registrar to make grpc-gateway's handlers available.
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	if cfg.Server.MaxResponseSizeBytes > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(cfg.Server.MaxResponseSizeBytes))))
	}
	conn, err := grpc.NewClient(fmt.Sprintf("%s:%d", cfg.Server.Listener.Address, cfg.Server.Listener.Port), opts...)
	if err != nil {
		return fmt.Errorf("failed to create gRPC client connection: %w", err)
	}
	defer func() {
		if err != nil {
			if cerr := conn.Close(); cerr != nil {
				log.Warn("failed to close gRPC transport connection after err", zap.Error(err))
			}
			return
		}
		go func() {
			<-ctx.Done()
			if cerr := conn.Close(); cerr != nil {
				log.Warn("failed to close gRPC transport connection when done", zap.Error(err))
			}
		}()
	}()

	// Wire up the registrar so endpoints can bind to both gRPC and JSON gateway.
	reg := newRegistrar(ctx, rpcMux.JSONGateway, rpcMux.GRPCServer, conn)

	// Instantiate and register endpoints.
	for name, factory := range cf.Endpoints {
		log := log.With(zap.String("endpointName", name))
		log.Info("registering endpoint")

		h, err := factory(cfg, log, scope.SubScope(name))
		if err != nil {
			return fmt.Errorf("endpoint %s instantiation failed: %w", name, err)
		}

		if err := h.Register(reg); err != nil {
			return fmt.Errorf("endpoint %s registration failed: %w", name, err)
		}
	}

	// Now that everything is registered, enable gRPC reflection.
	rpcMux.EnableGRPCReflection()

	// Save metadata on what RPCs being served for fast-lookup by internal services.
	if err := meta.ResolveMethodOptions(rpcMux.GRPCServer); err != nil {
		return fmt.Errorf("failed to resolve method options: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Listener.Address, cfg.Server.Listener.Port)
	log.Info("listening", zap.Namespace("tcp"), zap.String("addr", addr))

	// Figure out the maximum global timeout and set as a backstop (with 1s buffer).
	timeout := computeMaximumTimeout(&cfg.Server.Timeouts)
	if timeout > 0 {
		timeout += time.Second
	}

	// Start collecting go runtime stats if enabled
	if cfg.Server.Stats != nil && cfg.Server.Stats.GoRuntimeStats != nil {
		runtimeStats := stats.NewRuntimeStats(scope, cfg.Server.Stats.GoRuntimeStats)
		go runtimeStats.Collect(ctx)
	}

	srv := &http.Server{
		Handler:      mux.InsecureHandler(rpcMux),
		Addr:         addr,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(
		sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-sc:
	case err := <-errCh:
		return fmt.Errorf("listener failed: %w", err)
	}

	signal.Stop(sc)

	ctxShutDown, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err = srv.Shutdown(ctxShutDown); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Debug("server shutdown gracefully")
	return nil
}

func computeMaximumTimeout(cfg *config.Timeouts) time.Duration {
	if cfg == nil {
		return timeouts.DefaultTimeout
	}

	ret := cfg.Default
	for _, e := range cfg.Overrides {
		override := e.Timeout
		if ret == 0 || override == 0 {
			return 0
		}

		if override > ret {
			ret = override
		}
	}

	return ret
}

func getStatsReporterConfiguration(cfg *config.Config, logger *zap.Logger) (tally.ScopeOptions, http.Handler) {
	var metricsHandler http.Handler
	var scopeOpts tally.ScopeOptions

	statsPrefix := "admiral_api"
	if cfg.Server.Stats.Prefix != "" {
		statsPrefix = cfg.Server.Stats.Prefix
	}

	switch cfg.Server.Stats.ReporterType {
	case config.ReporterTypeNull:
		scopeOpts = tally.ScopeOptions{
			Reporter: tally.NullStatsReporter,
		}
		return scopeOpts, nil
	case config.ReporterTypeLog:
		scopeOpts = tally.ScopeOptions{
			Reporter: stats.NewLogReporter(logger),
			Prefix:   statsPrefix,
		}
		return scopeOpts, nil
	case config.ReporterTypePrometheus:
		reporter, err := stats.NewPrometheusReporter()
		if err != nil {
			logger.Error("error creating prometheus reporter, falling back to null", zap.Error(err))
			return tally.ScopeOptions{Reporter: tally.NullStatsReporter}, nil
		}
		scopeOpts = tally.ScopeOptions{
			CachedReporter:  reporter,
			Prefix:          statsPrefix,
			SanitizeOptions: &tallyprom.DefaultSanitizerOpts,
		}
		metricsHandler = reporter.HTTPHandler()
		return scopeOpts, metricsHandler
	default:
		return tally.ScopeOptions{}, nil
	}
}
