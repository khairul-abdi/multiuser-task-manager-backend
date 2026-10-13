package handler

import (
	"net/http"
	"strconv"

	"multiuser-task-manager-backend/internal/apperror"
	"multiuser-task-manager-backend/internal/domain"
	"multiuser-task-manager-backend/internal/middleware"
	"multiuser-task-manager-backend/internal/repository"
	"multiuser-task-manager-backend/internal/service"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *service.Service }

func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.StaticFile("/swagger/openapi.yaml", "./docs/openapi.yaml")
	r.POST("/auth/register", h.register)
	r.POST("/auth/login", h.login)
	tasks := r.Group("/tasks", middleware.Auth(h.svc))
	tasks.POST("", h.createTask)
	tasks.GET("", h.listTasks)
	tasks.GET("/:id", h.detailTask)
	tasks.PUT("/:id", h.updateTask)
	tasks.DELETE("/:id", h.deleteTask)
	tasks.POST("/:id/assign", h.assignTask)
}

func (h *Handler) register(c *gin.Context) {
	var req domain.RegisterRequest
	if !bind(c, &req) {
		return
	}
	resp, err := h.svc.Register(c.Request.Context(), req)
	respond(c, http.StatusCreated, resp, err)
}

func (h *Handler) login(c *gin.Context) {
	var req domain.LoginRequest
	if !bind(c, &req) {
		return
	}
	resp, err := h.svc.Login(c.Request.Context(), req)
	respond(c, http.StatusOK, resp, err)
}

func (h *Handler) createTask(c *gin.Context) {
	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		apperror.JSON(c, apperror.New(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required"))
		return
	}
	var req domain.CreateTaskRequest
	if !bind(c, &req) {
		return
	}
	resp, err := h.svc.CreateTaskIdempotent(c.Request.Context(), middleware.CurrentUser(c), key, req)
	if err != nil {
		apperror.JSON(c, err)
		return
	}
	c.JSON(resp.StatusCode, resp.Body)
}

func (h *Handler) listTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	resp, err := h.svc.ListTasks(c.Request.Context(), middleware.CurrentUser(c), repository.TaskFilter{
		Status: c.Query("status"), Search: c.Query("search"), Page: page, Limit: limit,
	})
	respond(c, http.StatusOK, resp, err)
}

func (h *Handler) detailTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	task, err := h.svc.FindTask(c.Request.Context(), id, middleware.CurrentUser(c))
	respond(c, http.StatusOK, gin.H{"data": task}, err)
}

func (h *Handler) updateTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req domain.UpdateTaskRequest
	if !bind(c, &req) {
		return
	}
	task, err := h.svc.UpdateTask(c.Request.Context(), id, middleware.CurrentUser(c), req)
	respond(c, http.StatusOK, gin.H{"data": task}, err)
}

func (h *Handler) deleteTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	err := h.svc.DeleteTask(c.Request.Context(), id, middleware.CurrentUser(c))
	if err != nil {
		apperror.JSON(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) assignTask(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req domain.AssignTaskRequest
	if !bind(c, &req) {
		return
	}
	task, err := h.svc.AssignTask(c.Request.Context(), id, middleware.CurrentUser(c).ID, req.AssigneeID)
	respond(c, http.StatusOK, gin.H{"data": task}, err)
}

func bind(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		apperror.JSON(c, apperror.New(http.StatusBadRequest, "VALIDATION_ERROR", err.Error()))
		return false
	}
	return true
}

func respond(c *gin.Context, status int, data any, err error) {
	if err != nil {
		apperror.JSON(c, err)
		return
	}
	c.JSON(status, data)
}

func idParam(c *gin.Context) (uint, bool) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		apperror.JSON(c, apperror.New(http.StatusBadRequest, "INVALID_PATH_PARAM", "id must be a positive integer"))
		return 0, false
	}
	return uint(id), true
}
