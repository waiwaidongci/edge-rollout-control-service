package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/example/edge-rollout-control/api/handler"
	"github.com/example/edge-rollout-control/api/router"
	"github.com/example/edge-rollout-control/internal/application"
	"github.com/example/edge-rollout-control/internal/config"
	"github.com/example/edge-rollout-control/internal/infrastructure/repository"
	"github.com/example/edge-rollout-control/internal/infrastructure/sqlite"
	"github.com/example/edge-rollout-control/internal/system"
)

type App struct {
	config     config.Config
	logger     *slog.Logger
	repository *repository.UnitOfWork
	service    *application.Service
	dispatcher *application.WebhookDispatcher
	server     *http.Server
	cancel     context.CancelFunc
	waitGroup  sync.WaitGroup
}

func New(cfg config.Config, logger *slog.Logger, version, commit string) (*App, error) {
	database, err := sqlite.Open(context.Background(), cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open sqlite repository: %w", err)
	}
	repository := repository.New(database)
	clock := system.RealClock{}
	ids := system.UUIDGenerator{}
	service := application.NewService(repository, clock, ids, logger)
	dispatcher := application.NewWebhookDispatcher(repository, clock, logger, cfg.Worker.WebhookMaxAttempts)
	httpHandler := handler.New(service, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return repository.Ping(ctx)
	}, logger, version, commit)
	routerHandler := router.New(httpHandler, logger, ids, cfg.Server.BodyLimit, cfg.Server.WriteTimeout)
	server := &http.Server{
		Addr: cfg.Server.Address, Handler: routerHandler, ReadTimeout: cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout, IdleTimeout: cfg.Server.IdleTimeout,
	}
	return &App{config: cfg, logger: logger, repository: repository, service: service, dispatcher: dispatcher, server: server}, nil
}

func (a *App) Run(ctx context.Context) error {
	workerContext, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.startWorkers(workerContext)
	serverErrors := make(chan error, 1)
	go func() {
		a.logger.Info("http server starting", "address", a.server.Addr)
		serverErrors <- a.server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		return a.Shutdown(context.Background())
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http server: %w", err)
	}
}

func (a *App) startWorkers(ctx context.Context) {
	a.waitGroup.Add(2)
	go a.runTicker(ctx, "rollout-scheduler", a.config.Worker.SchedulerInterval, func(runContext context.Context) error {
		return a.service.ProcessRunnableRollouts(runContext, 100)
	})
	go a.runTicker(ctx, "webhook-dispatcher", a.config.Worker.WebhookInterval, func(runContext context.Context) error {
		return a.dispatcher.Process(runContext, 100)
	})
}

func (a *App) runTicker(ctx context.Context, name string, interval time.Duration, work func(context.Context) error) {
	defer a.waitGroup.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			a.logger.Info("worker stopped", "worker", name)
			return
		case <-ticker.C:
			runContext, cancel := context.WithTimeout(ctx, interval)
			err := work(runContext)
			cancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error("worker iteration failed", "worker", name, "error", err)
			}
		}
	}
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	shutdownContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	serverError := a.server.Shutdown(shutdownContext)
	a.waitGroup.Wait()
	repositoryError := a.repository.Close()
	return errors.Join(serverError, repositoryError)
}
