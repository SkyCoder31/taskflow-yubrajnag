package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
)

type taskRepoLogger struct {
	next   repository.TaskRepository
	logger *slog.Logger
}

var _ repository.TaskRepository = (*taskRepoLogger)(nil)

func NewTaskRepo(next repository.TaskRepository, logger *slog.Logger) repository.TaskRepository {
	return &taskRepoLogger{
		next:   next,
		logger: logger.With("component", "repository.task"),
	}
}

func (r *taskRepoLogger) Create(ctx context.Context, task *domain.Task) error {
	start := time.Now()
	err := r.next.Create(ctx, task)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Create",
		slog.String("task_id", task.ID.String()),
		slog.String("project_id", task.ProjectID.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return err
}

func (r *taskRepoLogger) GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	start := time.Now()
	task, err := r.next.GetByID(ctx, id)
	r.logger.LogAttrs(ctx, logLevel(err),
		"GetByID",
		slog.String("task_id", id.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return task, err
}

func (r *taskRepoLogger) ListByProject(ctx context.Context, projectID uuid.UUID, filter repository.TaskFilter) ([]domain.Task, int, error) {
	start := time.Now()
	tasks, total, err := r.next.ListByProject(ctx, projectID, filter)
	r.logger.LogAttrs(ctx, logLevel(err),
		"ListByProject",
		slog.String("project_id", projectID.String()),
		slog.String("status_filter", string(filter.Status)),
		slog.Int("total", total),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return tasks, total, err
}

func (r *taskRepoLogger) Update(ctx context.Context, task *domain.Task) error {
	start := time.Now()
	err := r.next.Update(ctx, task)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Update",
		slog.String("task_id", task.ID.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return err
}

func (r *taskRepoLogger) Delete(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	err := r.next.Delete(ctx, id)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Delete",
		slog.String("task_id", id.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return err
}

func (r *taskRepoLogger) Stats(ctx context.Context, projectID uuid.UUID) (*repository.TaskStatsResult, error) {
	start := time.Now()
	result, err := r.next.Stats(ctx, projectID)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Stats",
		slog.String("project_id", projectID.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return result, err
}
