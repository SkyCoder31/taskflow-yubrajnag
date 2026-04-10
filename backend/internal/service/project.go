package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
)

type ProjectService struct {
	projects repository.ProjectRepository
}

func NewProjectService(projects repository.ProjectRepository) *ProjectService {
	return &ProjectService{projects: projects}
}

func (s *ProjectService) Create(ctx context.Context, name, description string, ownerID uuid.UUID) (*domain.Project, error) {
	project, err := domain.NewProject(name, description, ownerID)
	if err != nil {
		return nil, err
	}

	if err := s.projects.Create(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ProjectService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	return s.projects.GetByID(ctx, id)
}

func (s *ProjectService) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Project, int, error) {
	return s.projects.ListByUser(ctx, userID, page, limit)
}

func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, name, description string, callerID uuid.UUID) (*domain.Project, error) {
	project, err := s.projects.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if project.OwnerID != callerID {
		return nil, domain.ErrForbidden
	}

	if name != "" {
		project.Name = name
	}
	if description != "" {
		project.Description = description
	}

	if err := s.projects.Update(ctx, project); err != nil {
		return nil, err
	}

	return project, nil
}

func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID, callerID uuid.UUID) error {
	project, err := s.projects.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if project.OwnerID != callerID {
		return domain.ErrForbidden
	}

	return s.projects.Delete(ctx, id)
}
