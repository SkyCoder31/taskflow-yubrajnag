package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
)

type projectRepoLogger struct {
	next   repository.ProjectRepository
	logger *slog.Logger
}

var _ repository.ProjectRepository = (*projectRepoLogger)(nil)

func NewProjectRepo(next repository.ProjectRepository, logger *slog.Logger) repository.ProjectRepository {
	return &projectRepoLogger{
		next:   next,
		logger: logger.With("component", "repository.project"),
	}
}

func (r *projectRepoLogger) Create(ctx context.Context, project *domain.Project) error {
	start := time.Now()
	err := r.next.Create(ctx, project)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Create",
		slog.String("project_id", project.ID.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return err
}

func (r *projectRepoLogger) GetByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	start := time.Now()
	project, err := r.next.GetByID(ctx, id)
	r.logger.LogAttrs(ctx, logLevel(err),
		"GetByID",
		slog.String("project_id", id.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return project, err
}

func (r *projectRepoLogger) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Project, int, error) {
	start := time.Now()
	projects, total, err := r.next.ListByUser(ctx, userID, page, limit)
	r.logger.LogAttrs(ctx, logLevel(err),
		"ListByUser",
		slog.String("user_id", userID.String()),
		slog.Int("page", page),
		slog.Int("limit", limit),
		slog.Int("total", total),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return projects, total, err
}

func (r *projectRepoLogger) Update(ctx context.Context, project *domain.Project) error {
	start := time.Now()
	err := r.next.Update(ctx, project)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Update",
		slog.String("project_id", project.ID.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return err
}

func (r *projectRepoLogger) Delete(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	err := r.next.Delete(ctx, id)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Delete",
		slog.String("project_id", id.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return err
}
