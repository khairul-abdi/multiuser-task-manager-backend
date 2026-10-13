package middleware

import (
	"net/http"
	"strings"
	"time"

	"multiuser-task-manager-backend/internal/apperror"
	"multiuser-task-manager-backend/internal/domain"
	"multiuser-task-manager-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const userKey = "auth_user"

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := uuid.NewString()
		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
		status := c.Writer.Status()
		event := log.Info()
		if status >= 500 {
			event = log.Error()
		} else if status >= 400 {
			event = log.Warn()
		}
		event.Str("request_id", requestID).Str("method", c.Request.Method).Str("path", c.FullPath()).
			Int("status_code", status).Dur("latency", time.Since(start)).Msg("http_request")
	}
}

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		log.Error().Any("panic", recovered).Msg("panic recovered")
		apperror.JSON(c, apperror.New(http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "internal server error"))
	})
}

func Auth(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			apperror.JSON(c, apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "missing bearer token"))
			return
		}
		user, err := svc.Authenticate(c.Request.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			apperror.JSON(c, err)
			return
		}
		c.Set(userKey, *user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) domain.User {
	user, _ := c.Get(userKey)
	return user.(domain.User)
}
