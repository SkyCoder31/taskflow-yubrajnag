package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
)

type projectRepo struct {
	pool *pgxpool.Pool
}

var _ repository.ProjectRepository = (*projectRepo)(nil)

func NewProjectRepo(pool *pgxpool.Pool) repository.ProjectRepository {
	return &projectRepo{pool: pool}
}

func (r *projectRepo) Create(ctx context.Context, project *domain.Project) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO projects (id, name, description, owner_id, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		project.ID, project.Name, project.Description, project.OwnerID, project.CreatedAt,
	)
	return err
}

func (r *projectRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	var p domain.Project
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, description, owner_id, created_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *projectRepo) Update(ctx context.Context, project *domain.Project) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE projects SET name = $1, description = $2
		 WHERE id = $3`,
		project.Name, project.Description, project.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *projectRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *projectRepo) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Project, int, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT p.id) FROM projects p
		 LEFT JOIN tasks t ON t.project_id = p.id AND t.assignee_id = $1
		 WHERE p.owner_id = $1 OR t.assignee_id = $1`, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT p.id, p.name, p.description, p.owner_id, p.created_at
		 FROM projects p
		 LEFT JOIN tasks t ON t.project_id = p.id AND t.assignee_id = $1
		 WHERE p.owner_id = $1 OR t.assignee_id = $1
		 ORDER BY p.created_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return projects, total, nil
}
