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

type userRepo struct {
	pool *pgxpool.Pool
}

var _ repository.UserRepository = (*userRepo)(nil)

func NewUserRepo(pool *pgxpool.Pool) repository.UserRepository {
	return &userRepo{pool: pool}
}

func (r *userRepo) Create(ctx context.Context, user *domain.User) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, name, email, password, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		user.ID, user.Name, user.Email, user.Password, user.CreatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return domain.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *userRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, email, password, created_at
		 FROM users WHERE id = $1`, id,
	)
	return scanUser(row)
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, email, password, created_at
		 FROM users WHERE email = $1`, email,
	)
	return scanUser(row)
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}
