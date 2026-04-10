package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
)

type TaskService struct {
	tasks    repository.TaskRepository
	projects repository.ProjectRepository
}

func NewTaskService(tasks repository.TaskRepository, projects repository.ProjectRepository) *TaskService {
	return &TaskService{
		tasks:    tasks,
		projects: projects,
	}
}

func (s *TaskService) Create(
	ctx context.Context,
	title, description string,
	status domain.TaskStatus,
	priority domain.TaskPriority,
	projectID uuid.UUID,
	assigneeID *uuid.UUID,
	dueDate *time.Time,
) (*domain.Task, error) {
	builder := domain.NewTaskBuilder().
		Title(title).
		Description(description).
		Status(status).
		Priority(priority).
		ProjectID(projectID)

	if assigneeID != nil {
		builder.AssigneeID(*assigneeID)
	}
	if dueDate != nil {
		builder.DueDate(*dueDate)
	}

	task, err := builder.Build()
	if err != nil {
		return nil, err
	}

	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *TaskService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	return s.tasks.GetByID(ctx, id)
}

func (s *TaskService) ListByProject(ctx context.Context, projectID uuid.UUID, filter repository.TaskFilter) ([]domain.Task, int, error) {
	return s.tasks.ListByProject(ctx, projectID, filter)
}

func (s *TaskService) Update(
	ctx context.Context,
	id uuid.UUID,
	title, description *string,
	status *domain.TaskStatus,
	priority *domain.TaskPriority,
	assigneeID *uuid.UUID,
	dueDate *time.Time,
) (*domain.Task, error) {
	task, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if title != nil {
		task.Title = *title
	}
	if description != nil {
		task.Description = *description
	}
	if status != nil {
		if !status.IsValid() {
			ve := domain.NewValidationError()
			ve.Add("status", "must be one of: todo, in_progress, done")
			return nil, ve
		}
		task.Status = *status
	}
	if priority != nil {
		if !priority.IsValid() {
			ve := domain.NewValidationError()
			ve.Add("priority", "must be one of: low, medium, high")
			return nil, ve
		}
		task.Priority = *priority
	}
	if assigneeID != nil {
		task.AssigneeID = assigneeID
	}
	if dueDate != nil {
		task.DueDate = dueDate
	}

	if err := s.tasks.Update(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *TaskService) Delete(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	task, err := s.tasks.GetByID(ctx, id)
	if err != nil {
		return err
	}

	project, err := s.projects.GetByID(ctx, task.ProjectID)
	if err != nil {
		return err
	}

	if project.OwnerID != callerID {
		return domain.ErrForbidden
	}

	return s.tasks.Delete(ctx, id)
}

func (s *TaskService) Stats(ctx context.Context, projectID uuid.UUID) (*repository.TaskStatsResult, error) {
	return s.tasks.Stats(ctx, projectID)
}
