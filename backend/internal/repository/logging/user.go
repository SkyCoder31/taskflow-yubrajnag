package logging

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
)

type userRepoLogger struct {
	next   repository.UserRepository
	logger *slog.Logger
}

var _ repository.UserRepository = (*userRepoLogger)(nil)

func NewUserRepo(next repository.UserRepository, logger *slog.Logger) repository.UserRepository {
	return &userRepoLogger{
		next:   next,
		logger: logger.With("component", "repository.user"),
	}
}

func (r *userRepoLogger) Create(ctx context.Context, user *domain.User) error {
	start := time.Now()
	err := r.next.Create(ctx, user)
	r.logger.LogAttrs(ctx, logLevel(err),
		"Create",
		slog.String("user_id", user.ID.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return err
}

func (r *userRepoLogger) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	start := time.Now()
	user, err := r.next.GetByID(ctx, id)
	r.logger.LogAttrs(ctx, logLevel(err),
		"GetByID",
		slog.String("user_id", id.String()),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return user, err
}

func (r *userRepoLogger) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	start := time.Now()
	user, err := r.next.GetByEmail(ctx, email)
	r.logger.LogAttrs(ctx, logLevel(err),
		"GetByEmail",
		slog.String("email", email),
		slog.Duration("duration", time.Since(start)),
		errAttr(err),
	)
	return user, err
}

// ErrNotFound is a normal miss, not worth alarming on.
func logLevel(err error) slog.Level {
	if err != nil && err != domain.ErrNotFound {
		return slog.LevelError
	}
	return slog.LevelInfo
}

func errAttr(err error) slog.Attr {
	if err != nil {
		return slog.String("error", err.Error())
	}
	return slog.Attr{}
}
