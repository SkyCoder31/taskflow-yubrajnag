package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
)

func (s TaskStatus) IsValid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
)

func (p TaskPriority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	}
	return false
}

type Task struct {
	ID          uuid.UUID
	Title       string
	Description string
	Status      TaskStatus
	Priority    TaskPriority
	ProjectID   uuid.UUID
	AssigneeID  *uuid.UUID
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskBuilder struct {
	title       string
	description string
	status      TaskStatus
	priority    TaskPriority
	projectID   uuid.UUID
	assigneeID  *uuid.UUID
	dueDate     *time.Time
}

func NewTaskBuilder() *TaskBuilder {
	return &TaskBuilder{
		status:   StatusTodo,
		priority: PriorityMedium,
	}
}

func (b *TaskBuilder) Title(t string) *TaskBuilder {
	b.title = t
	return b
}

func (b *TaskBuilder) Description(d string) *TaskBuilder {
	b.description = d
	return b
}

func (b *TaskBuilder) Status(s TaskStatus) *TaskBuilder {
	b.status = s
	return b
}

func (b *TaskBuilder) Priority(p TaskPriority) *TaskBuilder {
	b.priority = p
	return b
}

func (b *TaskBuilder) ProjectID(id uuid.UUID) *TaskBuilder {
	b.projectID = id
	return b
}

func (b *TaskBuilder) AssigneeID(id uuid.UUID) *TaskBuilder {
	b.assigneeID = &id
	return b
}

func (b *TaskBuilder) DueDate(t time.Time) *TaskBuilder {
	b.dueDate = &t
	return b
}

func (b *TaskBuilder) Build() (*Task, error) {
	ve := NewValidationError()

	title := strings.TrimSpace(b.title)
	if title == "" {
		ve.Add("title", "is required")
	}

	if b.projectID == uuid.Nil {
		ve.Add("project_id", "is required")
	}

	if !b.status.IsValid() {
		ve.Add("status", "must be one of: todo, in_progress, done")
	}

	if !b.priority.IsValid() {
		ve.Add("priority", "must be one of: low, medium, high")
	}

	if ve.HasErrors() {
		return nil, ve
	}

	now := time.Now().UTC()
	return &Task{
		ID:          uuid.New(),
		Title:       title,
		Description: strings.TrimSpace(b.description),
		Status:      b.status,
		Priority:    b.priority,
		ProjectID:   b.projectID,
		AssigneeID:  b.assigneeID,
		DueDate:     b.dueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
