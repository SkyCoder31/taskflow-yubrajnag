package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/yubrajnag/taskflow/backend/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err)
		return
	}

	token, err := h.auth.Register(c.Request.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		Error(c, err)
		return
	}

	Created(c, authResponse{Token: token})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err)
		return
	}

	token, err := h.auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		Error(c, err)
		return
	}

	Success(c, authResponse{Token: token})
}
