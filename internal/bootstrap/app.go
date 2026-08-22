package bootstrap

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	alarmapp "github.com/wyw14/cry-086/internal/application/alarm"
	authapp "github.com/wyw14/cry-086/internal/application/auth"
	"github.com/wyw14/cry-086/internal/application/dashboard"
	"github.com/wyw14/cry-086/internal/application/evidence"
	fleetapp "github.com/wyw14/cry-086/internal/application/fleet"
	maintenanceapp "github.com/wyw14/cry-086/internal/application/maintenance"
	registryapp "github.com/wyw14/cry-086/internal/application/registry"
	reportapp "github.com/wyw14/cry-086/internal/application/report"
	safetyapp "github.com/wyw14/cry-086/internal/application/safety"
	telemetryapp "github.com/wyw14/cry-086/internal/application/telemetry"
	"github.com/wyw14/cry-086/internal/config"
	"github.com/wyw14/cry-086/internal/middleware"
	"github.com/wyw14/cry-086/internal/platform/clock"
	"github.com/wyw14/cry-086/internal/platform/files"
	"github.com/wyw14/cry-086/internal/platform/id"
	"github.com/wyw14/cry-086/internal/platform/notification"
	"github.com/wyw14/cry-086/internal/platform/token"
	"github.com/wyw14/cry-086/internal/repository/memory"
	"github.com/wyw14/cry-086/internal/repository/postgres"
	httptransport "github.com/wyw14/cry-086/internal/transport/http"
	"go.uber.org/zap"
)

type Repository interface {
	registryapp.Repository
	telemetryapp.Repository
	safetyapp.Repository
	alarmapp.Repository
	fleetapp.Repository
	maintenanceapp.Repository
	reportapp.Repository
	authapp.Repository
	dashboard.Repository
	evidence.Catalog
	demoRepository
}

type App struct {
	server          *http.Server
	logger          *zap.Logger
	releaseStorage  func()
	shutdownTimeout time.Duration
	closeOnce       sync.Once
}

type platformResources struct {
	repository Repository
	release    func()
	ready      func(context.Context) error
	clock      *clock.MonotonicUTC
	ids        *id.Generator
	signer     *token.Signer
	files      *files.LocalStore
}

type serviceGraph struct {
	auth        *authapp.Service
	telemetry   *telemetryapp.Service
	safety      *safetyapp.Service
	alarms      *alarmapp.Service
	fleet       *fleetapp.Service
	maintenance *maintenanceapp.Service
	reports     *reportapp.Service
	dashboard   *dashboard.Service
	evidence    *evidence.Service
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	resources, err := acquirePlatform(ctx, cfg)
	if err != nil {
		_ = logger.Sync()
		return nil, err
	}
	if cfg.SeedDemo {
		if err := seedDemo(ctx, resources.repository); err != nil {
			resources.release()
			_ = logger.Sync()
			return nil, err
		}
	}
	services := connectServices(resources)
	router := connectHTTP(cfg, logger, resources, services)
	server := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTP.RequestTimeout,
		WriteTimeout:      cfg.HTTP.RequestTimeout,
		IdleTimeout:       time.Minute,
	}
	return &App{
		server: server, logger: logger, releaseStorage: resources.release,
		shutdownTimeout: cfg.HTTP.ShutdownTimeout,
	}, nil
}

func acquirePlatform(ctx context.Context, cfg config.Config) (platformResources, error) {
	repository, release, ready, err := acquireRepository(ctx, cfg.Storage)
	if err != nil {
		return platformResources{}, err
	}
	fileStore, err := files.NewLocalStore(
		cfg.Storage.EvidenceRoot,
		cfg.Storage.MaxEvidenceBytes,
		[]string{".pdf", ".jpg", ".jpeg", ".png", ".csv"},
	)
	if err != nil {
		release()
		return platformResources{}, err
	}
	return platformResources{
		repository: repository,
		release:    release,
		ready:      ready,
		clock:      &clock.MonotonicUTC{},
		ids:        id.New("cg"),
		signer:     token.NewSigner(cfg.Credentials.AccessTokenSecret),
		files:      fileStore,
	}, nil
}

func acquireRepository(ctx context.Context, storage config.Storage) (Repository, func(), func(context.Context) error, error) {
	switch storage.Mode {
	case "postgres":
		repository, err := postgres.Open(ctx, storage.PostgresURL)
		if err != nil {
			return nil, nil, nil, err
		}
		return repository, repository.Close, repository.Ready, nil
	case "memory":
		repository := memory.New()
		return repository, func() {}, func(context.Context) error { return nil }, nil
	default:
		return nil, nil, nil, errors.New("unsupported storage mode")
	}
}

func connectServices(resources platformResources) serviceGraph {
	repository := resources.repository
	ingestion := telemetryapp.New(repository, telemetryapp.StandardConverter{}, resources.clock)
	return serviceGraph{
		auth:        authapp.New(repository, resources.signer, resources.clock, resources.ids),
		telemetry:   ingestion,
		safety:      safetyapp.New(repository, ingestion, resources.clock, resources.ids),
		alarms:      alarmapp.New(repository, repository, &notification.LocalSender{}, resources.clock, resources.ids),
		fleet:       fleetapp.New(repository, resources.clock),
		maintenance: maintenanceapp.New(repository, repository, resources.clock, resources.ids),
		reports:     reportapp.New(repository, repository, resources.clock, resources.ids),
		dashboard:   dashboard.New(repository, repository, resources.clock),
		evidence:    evidence.New(repository, repository, resources.files, resources.ids),
	}
}

func connectHTTP(cfg config.Config, logger *zap.Logger, resources platformResources, services serviceGraph) http.Handler {
	validate := validator.New()
	authHandler := httptransport.NewAuthHandler(services.auth, validate)
	telemetryHandler := httptransport.NewTelemetryHandler(services.telemetry, services.safety, validate)
	operationsHandler := httptransport.NewOperationsHandler(
		services.alarms, services.dashboard, services.fleet, services.maintenance, services.reports, validate,
	)
	evidenceHandler := httptransport.NewEvidenceHandler(services.evidence)
	settings := httptransport.RouterConfig{
		AllowedOrigins: cfg.HTTP.AllowedOrigins,
		SimulatorKey:   cfg.Credentials.SimulatorKey,
		WebDist:        cfg.HTTP.WebAssets,
	}
	return httptransport.NewRouter(
		settings, logger, &middleware.Metrics{}, resources.signer,
		authHandler, telemetryHandler, operationsHandler, evidenceHandler, resources.ready,
	)
}

func (a *App) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		return err
	}
	a.logger.Info("crane_safety_listening", zap.String("address", listener.Addr().String()))
	serveResult := make(chan error, 1)
	go func() { serveResult <- a.server.Serve(listener) }()

	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
		defer cancel()
		if err := a.server.Shutdown(shutdownContext); err != nil {
			return err
		}
		if err := <-serveResult; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func (a *App) Close() {
	a.closeOnce.Do(func() {
		a.releaseStorage()
		_ = a.logger.Sync()
	})
}
