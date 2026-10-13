package domain

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	TeamID   uint   `json:"team_id" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type CreateTaskRequest struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status"`
	AssigneeID  uint   `json:"assignee_id"`
}

type UpdateTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	AssigneeID  *uint   `json:"assignee_id"`
}

type AssignTaskRequest struct {
	AssigneeID uint `json:"assignee_id" binding:"required"`
}

type TaskList struct {
	Data       []Task `json:"data"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
	Total      int64  `json:"total"`
	TotalPages int    `json:"total_pages"`
}
