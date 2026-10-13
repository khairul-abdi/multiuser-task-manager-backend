package domain

import (
	"time"

	"gorm.io/datatypes"
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"size:120;not null" json:"name"`
	Email        string    `gorm:"size:160;uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	TeamID       uint      `gorm:"index;not null" json:"team_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Task struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Title       string    `gorm:"size:180;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	Status      string    `gorm:"size:32;index;not null;default:todo" json:"status"`
	OwnerID     uint      `gorm:"index;not null" json:"owner_id"`
	AssigneeID  uint      `gorm:"index;not null" json:"assignee_id"`
	TeamID      uint      `gorm:"index;not null" json:"team_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index;not null" json:"task_id"`
	ActorID   uint      `gorm:"index;not null" json:"actor_id"`
	Action    string    `gorm:"size:64;not null" json:"action"`
	Before    string    `gorm:"type:text" json:"before"`
	After     string    `gorm:"type:text" json:"after"`
	CreatedAt time.Time `json:"created_at"`
}

type IdempotencyRecord struct {
	ID           uint           `gorm:"primaryKey"`
	Key          string         `gorm:"size:64;uniqueIndex:idx_idempotency_key_user;not null"`
	UserID       uint           `gorm:"uniqueIndex:idx_idempotency_key_user;index;not null"`
	RequestHash  string         `gorm:"size:64;not null"`
	StatusCode   int            `gorm:"not null"`
	ResponseBody datatypes.JSON `gorm:"type:jsonb;not null"`
	ExpiresAt    time.Time      `gorm:"index;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
