package main

import (
	"os"
	"time"

	"multiuser-task-manager-backend/internal/config"
	"multiuser-task-manager-backend/internal/domain"
	"multiuser-task-manager-backend/internal/handler"
	"multiuser-task-manager-backend/internal/middleware"
	"multiuser-task-manager-backend/internal/repository"
	"multiuser-task-manager-backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	zerolog.TimeFieldFormat = time.RFC3339Nano
	log.Logger = log.Output(os.Stdout)
	gin.SetMode(gin.ReleaseMode)

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("connect database")
	}
	if cfg.AutoMigrate {
		if err := db.AutoMigrate(&domain.User{}, &domain.Task{}, &domain.TaskLog{}, &domain.IdempotencyRecord{}); err != nil {
			log.Fatal().Err(err).Msg("auto migrate")
		}
	}

	repo := repository.NewGorm(db)
	svc := service.New(repo, cfg.JWTSecret, cfg.TokenTTL, cfg.IdempotencyTTL)
	r := gin.New()
	r.Use(middleware.Recovery(), middleware.RequestLogger())
	handler.New(svc).RegisterRoutes(r)
	log.Info().Str("addr", cfg.Addr()).Msg("server started")
	if err := r.Run(cfg.Addr()); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
