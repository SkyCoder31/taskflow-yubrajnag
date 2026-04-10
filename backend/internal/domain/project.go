package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID          uuid.UUID
	Name        string
	Description string
	OwnerID     uuid.UUID
	CreatedAt   time.Time
}

func NewProject(name, description string, ownerID uuid.UUID) (*Project, error) {
	ve := NewValidationError()

	name = strings.TrimSpace(name)
	if name == "" {
		ve.Add("name", "is required")
	}

	if ownerID == uuid.Nil {
		ve.Add("owner_id", "is required")
	}

	if ve.HasErrors() {
		return nil, ve
	}

	return &Project{
		ID:          uuid.New(),
		Name:        name,
		Description: strings.TrimSpace(description),
		OwnerID:     ownerID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}
