package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
)

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

func List(c *gin.Context, data any, page, limit, total int) {
	c.JSON(http.StatusOK, gin.H{
		"data": data,
		"meta": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func Error(c *gin.Context, err error) {
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation failed",
			"fields": ve.Fields,
		})
		return
	}

	status := mapError(err)
	c.JSON(status, gin.H{"error": err.Error()})
}

func BadRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func mapError(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrAlreadyExists):
		return http.StatusConflict
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
