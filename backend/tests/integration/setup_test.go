package integration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yubrajnag/taskflow/backend/internal/auth"
	"github.com/yubrajnag/taskflow/backend/internal/handler"
	"github.com/yubrajnag/taskflow/backend/internal/repository/postgres"
	"github.com/yubrajnag/taskflow/backend/internal/service"
	"github.com/yubrajnag/taskflow/backend/migrations"
)

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://taskflow:taskflow@localhost:5432/taskflow_test?sslmode=disable"
}

func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()

	dsn := testDSN()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to test DB: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging test DB: %v (is Docker Compose running?)", err)
	}
	t.Cleanup(func() { pool.Close() })

	if err := migrations.Run(dsn); err != nil {
		t.Fatalf("running migrations: %v", err)
	}

	_, err = pool.Exec(ctx, "TRUNCATE tasks, projects, users CASCADE")
	if err != nil {
		t.Fatalf("truncating tables: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	userRepo := postgres.NewUserRepo(pool)
	projectRepo := postgres.NewProjectRepo(pool)
	taskRepo := postgres.NewTaskRepo(pool)

	// bcrypt cost 4 (minimum) for fast tests
	tokenService := auth.NewTokenService("test-secret-for-integration-tests", 1*time.Hour)
	authService := service.NewAuthService(userRepo, tokenService, 4)
	projectService := service.NewProjectService(projectRepo)
	taskService := service.NewTaskService(taskRepo, projectRepo)

	authHandler := handler.NewAuthHandler(authService)
	projectHandler := handler.NewProjectHandler(projectService)
	taskHandler := handler.NewTaskHandler(taskService)

	return handler.NewRouter(logger, tokenService, authHandler, projectHandler, taskHandler)
}

func registerAndLogin(t *testing.T, router *gin.Engine, name, email, password string) string {
	t.Helper()

	body := fmt.Sprintf(`{"name":"%s","email":"%s","password":"%s"}`, name, email, password)
	w := doRequest(router, "POST", "/auth/register", body, "")
	if w.Code != 201 {
		t.Fatalf("registerAndLogin: register failed with %d: %s", w.Code, w.Body.String())
	}

	return extractToken(t, w)
}
