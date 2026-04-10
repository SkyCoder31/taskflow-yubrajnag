package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yubrajnag/taskflow/backend/internal/auth"
	"github.com/yubrajnag/taskflow/backend/internal/config"
	"github.com/yubrajnag/taskflow/backend/internal/handler"
	"github.com/yubrajnag/taskflow/backend/internal/repository/logging"
	"github.com/yubrajnag/taskflow/backend/internal/repository/postgres"
	"github.com/yubrajnag/taskflow/backend/internal/service"
	"github.com/yubrajnag/taskflow/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	pool, err := connectDB(cfg.Database)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()
	logger.Info("connected to database")

	if err := migrations.Run(cfg.Database.DSN()); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	logger.Info("migrations complete")

	userRepo := logging.NewUserRepo(postgres.NewUserRepo(pool), logger)
	projectRepo := logging.NewProjectRepo(postgres.NewProjectRepo(pool), logger)
	taskRepo := logging.NewTaskRepo(postgres.NewTaskRepo(pool), logger)

	tokenService := auth.NewTokenService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)
	authService := service.NewAuthService(userRepo, tokenService, cfg.Auth.BcryptCost)
	projectService := service.NewProjectService(projectRepo)
	taskService := service.NewTaskService(taskRepo, projectRepo)

	authHandler := handler.NewAuthHandler(authService)
	projectHandler := handler.NewProjectHandler(projectService)
	taskHandler := handler.NewTaskHandler(taskService)

	router := handler.NewRouter(logger, tokenService, authHandler, projectHandler, taskHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server starting", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}

func connectDB(dbCfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	for attempt := 1; attempt <= 5; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err = pgxpool.New(ctx, dbCfg.DSN())
		if err == nil {
			err = pool.Ping(ctx)
		}
		cancel()

		if err == nil {
			return pool, nil
		}

		slog.Warn("database not ready, retrying...",
			"attempt", attempt,
			"error", err.Error(),
		)
		time.Sleep(time.Duration(attempt) * time.Second)
	}

	return nil, fmt.Errorf("after 5 attempts: %w", err)
}
