package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/service"
)

type ProjectHandler struct {
	projects *service.ProjectService
}

func NewProjectHandler(projects *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projects: projects}
}

type createProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type projectResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
	CreatedAt   string `json:"created_at"`
}

func toProjectResponse(p *domain.Project) projectResponse {
	return projectResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		OwnerID:     p.OwnerID.String(),
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
	}
}


func (h *ProjectHandler) Create(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err)
		return
	}

	callerID := GetUserID(c)
	project, err := h.projects.Create(c.Request.Context(), req.Name, req.Description, callerID)
	if err != nil {
		Error(c, err)
		return
	}

	Created(c, toProjectResponse(project))
}


func (h *ProjectHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	project, err := h.projects.GetByID(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, toProjectResponse(project))
}


func (h *ProjectHandler) List(c *gin.Context) {
	callerID := GetUserID(c)
	page, _ := strconv.Atoi(c.Query("page"))
	limit, _ := strconv.Atoi(c.Query("limit"))

	projects, total, err := h.projects.ListByUser(c.Request.Context(), callerID, page, limit)
	if err != nil {
		Error(c, err)
		return
	}

	resp := make([]projectResponse, len(projects))
	for i := range projects {
		resp[i] = toProjectResponse(&projects[i])
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	List(c, resp, page, limit, total)
}


func (h *ProjectHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err)
		return
	}

	callerID := GetUserID(c)
	project, err := h.projects.Update(c.Request.Context(), id, req.Name, req.Description, callerID)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, toProjectResponse(project))
}


func (h *ProjectHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		Error(c, domain.ErrNotFound)
		return
	}

	callerID := GetUserID(c)
	if err := h.projects.Delete(c.Request.Context(), id, callerID); err != nil {
		Error(c, err)
		return
	}

	Success(c, gin.H{"deleted": true})
}
