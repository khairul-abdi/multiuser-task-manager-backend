package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"multiuser-task-manager-backend/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("record not found")

type TaskFilter struct {
	Status string
	Search string
	Page   int
	Limit  int
}

type Repository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	FindUserByEmail(ctx context.Context, email string) (*domain.User, error)
	FindUserByID(ctx context.Context, id uint) (*domain.User, error)
	CreateTask(ctx context.Context, task *domain.Task) error
	ListTasks(ctx context.Context, user domain.User, filter TaskFilter) ([]domain.Task, int64, error)
	FindTaskForUser(ctx context.Context, id uint, user domain.User) (*domain.Task, error)
	UpdateTask(ctx context.Context, task *domain.Task) error
	DeleteTask(ctx context.Context, task *domain.Task) error
	FindValidIdempotency(ctx context.Context, key string, userID uint, now time.Time) (*domain.IdempotencyRecord, error)
	CreateIdempotency(ctx context.Context, record *domain.IdempotencyRecord) error
	AssignTaskTx(ctx context.Context, taskID, actorID, assigneeID uint) (*domain.Task, error)
}

type GormRepository struct{ db *gorm.DB }

func NewGorm(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) CreateUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormRepository) FindUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &user, nil
}

func (r *GormRepository) FindUserByID(ctx context.Context, id uint) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &user, nil
}

func (r *GormRepository) CreateTask(ctx context.Context, task *domain.Task) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *GormRepository) ListTasks(ctx context.Context, user domain.User, filter TaskFilter) ([]domain.Task, int64, error) {
	var tasks []domain.Task
	var total int64
	q := r.db.WithContext(ctx).Model(&domain.Task{}).Where("team_id = ? AND (owner_id = ? OR assignee_id = ?)", user.TeamID, user.ID, user.ID)
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Search != "" {
		q = q.Where("title ILIKE ?", "%"+filter.Search+"%")
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (filter.Page - 1) * filter.Limit
	if err := q.Order("created_at DESC").Limit(filter.Limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}
	return tasks, total, nil
}

func (r *GormRepository) FindTaskForUser(ctx context.Context, id uint, user domain.User) (*domain.Task, error) {
	var task domain.Task
	err := r.db.WithContext(ctx).Where("id = ? AND team_id = ? AND (owner_id = ? OR assignee_id = ?)", id, user.TeamID, user.ID, user.ID).First(&task).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &task, nil
}

func (r *GormRepository) UpdateTask(ctx context.Context, task *domain.Task) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *GormRepository) DeleteTask(ctx context.Context, task *domain.Task) error {
	return r.db.WithContext(ctx).Delete(task).Error
}

func (r *GormRepository) FindValidIdempotency(ctx context.Context, key string, userID uint, now time.Time) (*domain.IdempotencyRecord, error) {
	var rec domain.IdempotencyRecord
	err := r.db.WithContext(ctx).Where("key = ? AND user_id = ? AND expires_at > ?", key, userID, now).First(&rec).Error
	if err != nil {
		return nil, mapNotFound(err)
	}
	return &rec, nil
}

func (r *GormRepository) CreateIdempotency(ctx context.Context, record *domain.IdempotencyRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

func (r *GormRepository) AssignTaskTx(ctx context.Context, taskID, actorID, assigneeID uint) (*domain.Task, error) {
	var task domain.Task
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var actor, assignee domain.User
		if err := tx.First(&actor, actorID).Error; err != nil {
			return mapNotFound(err)
		}
		if err := tx.First(&assignee, assigneeID).Error; err != nil {
			return mapNotFound(err)
		}
		if actor.TeamID != assignee.TeamID {
			return ErrNotFound
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND team_id = ?", taskID, actor.TeamID).First(&task).Error; err != nil {
			return mapNotFound(err)
		}
		before := task.AssigneeID
		task.AssigneeID = assigneeID
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		return tx.Create(&domain.TaskLog{
			TaskID: task.ID, ActorID: actorID, Action: "assign",
			Before: uintToString(before), After: uintToString(assigneeID),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func uintToString(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
