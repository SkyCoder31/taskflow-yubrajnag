package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
)

type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

type UserWriter interface {
	Create(ctx context.Context, user *domain.User) error
}

type UserRepository interface {
	UserReader
	UserWriter
}

type ProjectReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Project, error)
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Project, int, error)
}

type ProjectWriter interface {
	Create(ctx context.Context, project *domain.Project) error
	Update(ctx context.Context, project *domain.Project) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProjectRepository interface {
	ProjectReader
	ProjectWriter
}

// Zero values mean "no filter."
type TaskFilter struct {
	Status   domain.TaskStatus
	Assignee uuid.UUID
	Page     int
	Limit    int
}

type TaskReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error)
	ListByProject(ctx context.Context, projectID uuid.UUID, filter TaskFilter) ([]domain.Task, int, error)
}

type TaskWriter interface {
	Create(ctx context.Context, task *domain.Task) error
	Update(ctx context.Context, task *domain.Task) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type TaskStatsResult struct {
	ByStatus   map[domain.TaskStatus]int `json:"by_status"`
	ByAssignee map[uuid.UUID]int         `json:"by_assignee"`
}

type TaskRepository interface {
	TaskReader
	TaskWriter
	Stats(ctx context.Context, projectID uuid.UUID) (*TaskStatsResult, error)
}
