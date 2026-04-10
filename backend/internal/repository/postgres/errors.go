package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolation = "23505"

func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == uniqueViolation
	}
	return false
}
