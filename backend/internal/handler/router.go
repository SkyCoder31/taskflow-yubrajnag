package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yubrajnag/taskflow/backend/internal/auth"
)

func NewRouter(
	logger *slog.Logger,
	tokens *auth.TokenService,
	authH *AuthHandler,
	projectH *ProjectHandler,
	taskH *TaskHandler,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(requestLogger(logger))
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": gin.H{"status": "ok"}})
	})

	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", authH.Register)
		authGroup.POST("/login", authH.Login)
	}

	protected := r.Group("/", AuthMiddleware(tokens))

	projects := protected.Group("/projects")
	{
		projects.POST("", projectH.Create)
		projects.GET("", projectH.List)
		projects.GET("/:id", projectH.GetByID)
		projects.PUT("/:id", projectH.Update)
		projects.DELETE("/:id", projectH.Delete)

		projects.POST("/:id/tasks", taskH.Create)
		projects.GET("/:id/tasks", taskH.ListByProject)
		projects.GET("/:id/stats", taskH.Stats)
	}

	tasks := protected.Group("/tasks")
	{
		tasks.GET("/:id", taskH.GetByID)
		tasks.PUT("/:id", taskH.Update)
		tasks.DELETE("/:id", taskH.Delete)
	}

	return r
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		logger.LogAttrs(c.Request.Context(), slog.LevelInfo,
			"HTTP",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("duration", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}
