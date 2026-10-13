package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"multiuser-task-manager-backend/internal/apperror"
	"multiuser-task-manager-backend/internal/domain"
	"multiuser-task-manager-backend/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
)

type Service struct {
	repo      repository.Repository
	jwtSecret []byte
	tokenTTL  time.Duration
	idemTTL   time.Duration
	idemLocks sync.Map
}

type IdempotentResponse struct {
	StatusCode int
	Body       any
	Replayed   bool
}

func New(repo repository.Repository, jwtSecret string, tokenTTL, idemTTL time.Duration) *Service {
	return &Service{repo: repo, jwtSecret: []byte(jwtSecret), tokenTTL: tokenTTL, idemTTL: idemTTL}
}

func (s *Service) Register(ctx context.Context, req domain.RegisterRequest) (*domain.AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &domain.User{Name: req.Name, Email: req.Email, PasswordHash: string(hash), TeamID: req.TeamID}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, apperror.Wrap(http.StatusConflict, "EMAIL_ALREADY_EXISTS", "email already registered", err)
	}
	token, err := s.issueToken(user.ID)
	if err != nil {
		return nil, err
	}
	return &domain.AuthResponse{Token: token, User: *user}, nil
}

func (s *Service) Login(ctx context.Context, req domain.LoginRequest) (*domain.AuthResponse, error) {
	user, err := s.repo.FindUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperror.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return nil, apperror.New(http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
	}
	token, err := s.issueToken(user.ID)
	if err != nil {
		return nil, err
	}
	return &domain.AuthResponse{Token: token, User: *user}, nil
}

func (s *Service) Authenticate(ctx context.Context, tokenText string) (*domain.User, error) {
	token, err := jwt.Parse(tokenText, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("invalid signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}
	userIDf, ok := claims["sub"].(float64)
	if !ok {
		return nil, apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}
	user, err := s.repo.FindUserByID(ctx, uint(userIDf))
	if err != nil {
		return nil, apperror.New(http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
	}
	return user, nil
}

func (s *Service) CreateTaskIdempotent(ctx context.Context, user domain.User, key string, req domain.CreateTaskRequest) (*IdempotentResponse, error) {
	if _, err := uuid.Parse(key); err != nil {
		return nil, apperror.New(http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Idempotency-Key must be a valid UUID")
	}
	lock := s.lockFor(key, user.ID)
	lock.Lock()
	defer lock.Unlock()

	now := time.Now().UTC()
	requestHash := hashJSON(req)
	if rec, err := s.repo.FindValidIdempotency(ctx, key, user.ID, now); err == nil {
		if rec.RequestHash != requestHash {
			return nil, apperror.New(http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotency key was already used with a different request body")
		}
		var body any
		_ = json.Unmarshal(rec.ResponseBody, &body)
		return &IdempotentResponse{StatusCode: rec.StatusCode, Body: body, Replayed: true}, nil
	}
	if req.Status == "" {
		req.Status = "todo"
	}
	if req.AssigneeID == 0 {
		req.AssigneeID = user.ID
	}
	task := &domain.Task{Title: req.Title, Description: req.Description, Status: req.Status, OwnerID: user.ID, AssigneeID: req.AssigneeID, TeamID: user.TeamID}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	body := ginH{"data": task}
	raw, _ := json.Marshal(body)
	if err := s.repo.CreateIdempotency(ctx, &domain.IdempotencyRecord{
		Key: key, UserID: user.ID, RequestHash: requestHash, StatusCode: http.StatusCreated,
		ResponseBody: datatypes.JSON(raw), ExpiresAt: now.Add(s.idemTTL),
	}); err != nil {
		return nil, err
	}
	return &IdempotentResponse{StatusCode: http.StatusCreated, Body: body}, nil
}

func (s *Service) ListTasks(ctx context.Context, user domain.User, filter repository.TaskFilter) (*domain.TaskList, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 10
	}
	tasks, total, err := s.repo.ListTasks(ctx, user, filter)
	if err != nil {
		return nil, err
	}
	pages := int((total + int64(filter.Limit) - 1) / int64(filter.Limit))
	return &domain.TaskList{Data: tasks, Page: filter.Page, Limit: filter.Limit, Total: total, TotalPages: pages}, nil
}

func (s *Service) FindTask(ctx context.Context, id uint, user domain.User) (*domain.Task, error) {
	task, err := s.repo.FindTaskForUser(ctx, id, user)
	if err != nil {
		return nil, notFound(err)
	}
	return task, nil
}

func (s *Service) UpdateTask(ctx context.Context, id uint, user domain.User, req domain.UpdateTaskRequest) (*domain.Task, error) {
	task, err := s.FindTask(ctx, id, user)
	if err != nil {
		return nil, err
	}
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.AssigneeID != nil {
		task.AssigneeID = *req.AssigneeID
	}
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) DeleteTask(ctx context.Context, id uint, user domain.User) error {
	task, err := s.FindTask(ctx, id, user)
	if err != nil {
		return err
	}
	return s.repo.DeleteTask(ctx, task)
}

func (s *Service) AssignTask(ctx context.Context, id, actorID, assigneeID uint) (*domain.Task, error) {
	task, err := s.repo.AssignTaskTx(ctx, id, actorID, assigneeID)
	if err != nil {
		return nil, notFound(err)
	}
	log.Info().Uint("task_id", id).Uint("assignee_id", assigneeID).Msg("mock notification: task assigned")
	return task, nil
}

func (s *Service) issueToken(userID uint) (string, error) {
	claims := jwt.MapClaims{"sub": userID, "exp": time.Now().Add(s.tokenTTL).Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *Service) lockFor(key string, userID uint) *sync.Mutex {
	lockKey := strconv.FormatUint(uint64(userID), 10) + ":" + key
	val, _ := s.idemLocks.LoadOrStore(lockKey, &sync.Mutex{})
	return val.(*sync.Mutex)
}

func hashJSON(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func notFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return apperror.New(http.StatusNotFound, "NOT_FOUND", "resource not found")
	}
	return err
}

type ginH map[string]any
