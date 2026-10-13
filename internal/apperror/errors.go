package apperror

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Err }

func New(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func Wrap(status int, code, message string, err error) *AppError {
	return &AppError{Status: status, Code: code, Message: message, Err: err}
}

type ErrorResponse struct {
	Status    int       `json:"status"`
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func JSON(c *gin.Context, err error) {
	var app *AppError
	if !errors.As(err, &app) {
		app = New(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "internal server error")
	}
	c.AbortWithStatusJSON(app.Status, ErrorResponse{
		Status:    app.Status,
		Code:      app.Code,
		Message:   app.Message,
		Timestamp: time.Now().UTC(),
	})
}
