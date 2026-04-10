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

type taskRepo struct {
	pool *pgxpool.Pool
}

var _ repository.TaskRepository = (*taskRepo)(nil)

func NewTaskRepo(pool *pgxpool.Pool) repository.TaskRepository {
	return &taskRepo{pool: pool}
}

func (r *taskRepo) Create(ctx context.Context, task *domain.Task) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tasks (id, title, description, status, priority, project_id, assignee_id, due_date, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		task.ID, task.Title, task.Description, task.Status, task.Priority,
		task.ProjectID, task.AssigneeID, task.DueDate, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (r *taskRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, title, description, status, priority, project_id, assignee_id, due_date, created_at, updated_at
		 FROM tasks WHERE id = $1`, id,
	)
	return scanTask(row)
}

func (r *taskRepo) Update(ctx context.Context, task *domain.Task) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tasks SET title = $1, description = $2, status = $3, priority = $4,
		 assignee_id = $5, due_date = $6
		 WHERE id = $7`,
		task.Title, task.Description, task.Status, task.Priority,
		task.AssigneeID, task.DueDate, task.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *taskRepo) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *taskRepo) ListByProject(ctx context.Context, projectID uuid.UUID, filter repository.TaskFilter) ([]domain.Task, int, error) {
	page := filter.Page
	limit := filter.Limit
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	qb := NewQueryBuilder(
		"SELECT id, title, description, status, priority, project_id, assignee_id, due_date, created_at, updated_at FROM tasks",
		"SELECT COUNT(*) FROM tasks",
	).
		Where("project_id = %s", projectID).
		WhereIf(filter.Status != "", "status = %s", string(filter.Status)).
		WhereIf(filter.Assignee != uuid.Nil, "assignee_id = %s", filter.Assignee).
		OrderBy("created_at DESC").
		Paginate(page, limit)

	query, queryArgs, countQuery, countArgs := qb.Build()

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		var t domain.Task
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
			&t.ProjectID, &t.AssigneeID, &t.DueDate, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

func (r *taskRepo) Stats(ctx context.Context, projectID uuid.UUID) (*repository.TaskStatsResult, error) {
	result := &repository.TaskStatsResult{
		ByStatus:   make(map[domain.TaskStatus]int),
		ByAssignee: make(map[uuid.UUID]int),
	}

	rows, err := r.pool.Query(ctx,
		`SELECT status, COUNT(*) FROM tasks WHERE project_id = $1 GROUP BY status`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status domain.TaskStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		result.ByStatus[status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.pool.Query(ctx,
		`SELECT assignee_id, COUNT(*) FROM tasks
		 WHERE project_id = $1 AND assignee_id IS NOT NULL
		 GROUP BY assignee_id`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var assigneeID uuid.UUID
		var count int
		if err := rows.Scan(&assigneeID, &count); err != nil {
			return nil, err
		}
		result.ByAssignee[assigneeID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func scanTask(row pgx.Row) (*domain.Task, error) {
	var t domain.Task
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority,
		&t.ProjectID, &t.AssigneeID, &t.DueDate, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}
