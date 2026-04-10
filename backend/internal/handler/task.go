package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
	"github.com/yubrajnag/taskflow/backend/internal/service"
)

type TaskHandler struct {
	tasks *service.TaskService
}

func NewTaskHandler(tasks *service.TaskService) *TaskHandler {
	return &TaskHandler{tasks: tasks}
}

type createTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	AssigneeID  string `json:"assignee_id"`
	DueDate     string `json:"due_date"`
}

type updateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
}

type taskResponse struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	ProjectID   string  `json:"project_id"`
	AssigneeID  *string `json:"assignee_id"`
	DueDate     *string `json:"due_date"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

func toTaskResponse(t *domain.Task) taskResponse {
	resp := taskResponse{
		ID:          t.ID.String(),
		Title:       t.Title,
		Description: t.Description,
		Status:      string(t.Status),
		Priority:    string(t.Priority),
		ProjectID:   t.ProjectID.String(),
		CreatedAt:   t.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   t.UpdatedAt.Format(time.RFC3339),
	}
	if t.AssigneeID != nil {
		s := t.AssigneeID.String()
		resp.AssigneeID = &s
	}
	if t.DueDate != nil {
		s := t.DueDate.Format(time.RFC3339)
		resp.DueDate = &s
	}
	return resp
}


func (h *TaskHandler) Create(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	var req createTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err)
		return
	}

	var assigneeID *uuid.UUID
	if req.AssigneeID != "" {
		parsed, err := uuid.Parse(req.AssigneeID)
		if err != nil {
			ve := domain.NewValidationError()
			ve.Add("assignee_id", "must be a valid UUID")
			Error(c, ve)
			return
		}
		assigneeID = &parsed
	}

	var dueDate *time.Time
	if req.DueDate != "" {
		parsed, err := time.Parse(time.RFC3339, req.DueDate)
		if err != nil {
			ve := domain.NewValidationError()
			ve.Add("due_date", "must be RFC3339 format")
			Error(c, ve)
			return
		}
		dueDate = &parsed
	}

	status := domain.TaskStatus(req.Status)
	priority := domain.TaskPriority(req.Priority)

	task, err := h.tasks.Create(
		c.Request.Context(),
		req.Title, req.Description,
		status, priority,
		projectID, assigneeID, dueDate,
	)
	if err != nil {
		Error(c, err)
		return
	}

	Created(c, toTaskResponse(task))
}


func (h *TaskHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	task, err := h.tasks.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, toTaskResponse(task))
}


func (h *TaskHandler) ListByProject(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	var assignee uuid.UUID
	if a := c.Query("assignee"); a != "" {
		assignee, _ = uuid.Parse(a)
	}

	filter := repository.TaskFilter{
		Status:   domain.TaskStatus(c.Query("status")),
		Assignee: assignee,
		Page:     page,
		Limit:    limit,
	}

	tasks, total, err := h.tasks.ListByProject(c.Request.Context(), projectID, filter)
	if err != nil {
		Error(c, err)
		return
	}

	resp := make([]taskResponse, len(tasks))
	for i := range tasks {
		resp[i] = toTaskResponse(&tasks[i])
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	List(c, resp, page, limit, total)
}


func (h *TaskHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	var req updateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err)
		return
	}

	var status *domain.TaskStatus
	if req.Status != nil {
		s := domain.TaskStatus(*req.Status)
		status = &s
	}

	var priority *domain.TaskPriority
	if req.Priority != nil {
		p := domain.TaskPriority(*req.Priority)
		priority = &p
	}

	var assigneeID *uuid.UUID
	if req.AssigneeID != nil {
		parsed, err := uuid.Parse(*req.AssigneeID)
		if err != nil {
			ve := domain.NewValidationError()
			ve.Add("assignee_id", "must be a valid UUID")
			Error(c, ve)
			return
		}
		assigneeID = &parsed
	}

	var dueDate *time.Time
	if req.DueDate != nil {
		parsed, err := time.Parse(time.RFC3339, *req.DueDate)
		if err != nil {
			ve := domain.NewValidationError()
			ve.Add("due_date", "must be RFC3339 format")
			Error(c, ve)
			return
		}
		dueDate = &parsed
	}

	task, err := h.tasks.Update(
		c.Request.Context(),
		id, req.Title, req.Description,
		status, priority, assigneeID, dueDate,
	)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, toTaskResponse(task))
}


func (h *TaskHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	callerID := GetUserID(c)
	if err := h.tasks.Delete(c.Request.Context(), id, callerID); err != nil {
		Error(c, err)
		return
	}

	Success(c, gin.H{"deleted": true})
}


func (h *TaskHandler) Stats(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	result, err := h.tasks.Stats(c.Request.Context(), projectID)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, result)
}
