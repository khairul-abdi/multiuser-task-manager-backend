package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"multiuser-task-manager-backend/internal/domain"
	"multiuser-task-manager-backend/internal/repository"
	"multiuser-task-manager-backend/internal/service"
)

func TestCreateTaskIdempotentSequential(t *testing.T) {
	repo := newMemoryRepo()
	svc := service.New(repo, "secret", time.Hour, 24*time.Hour)
	user := domain.User{ID: 1, TeamID: 10}
	key := "9a96ef8d-1064-4b9f-ad13-eef0f7333719"
	req := domain.CreateTaskRequest{Title: "write api"}

	first, err := svc.CreateTaskIdempotent(context.Background(), user, key, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateTaskIdempotent(context.Background(), user, key, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.StatusCode != 201 || second.StatusCode != 201 {
		t.Fatalf("expected both responses to be 201, got %d and %d", first.StatusCode, second.StatusCode)
	}
	if !second.Replayed {
		t.Fatal("expected second response to be replayed")
	}
	if repo.createdTasks != 1 {
		t.Fatalf("expected one task created, got %d", repo.createdTasks)
	}
}

func TestCreateTaskIdempotentConcurrentDuplicate(t *testing.T) {
	repo := newMemoryRepo()
	svc := service.New(repo, "secret", time.Hour, 24*time.Hour)
	user := domain.User{ID: 1, TeamID: 10}
	key := "36da41db-c714-4cfe-b18d-65f3c0e9f113"
	req := domain.CreateTaskRequest{Title: "same retry"}
	const goroutines = 50

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateTaskIdempotent(context.Background(), user, key, req)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if repo.createdTasks != 1 {
		t.Fatalf("expected exactly one task created, got %d", repo.createdTasks)
	}
}

type memoryRepo struct {
	mu           sync.Mutex
	nextTaskID   uint
	tasks        map[uint]domain.Task
	idem         map[string]domain.IdempotencyRecord
	createdTasks int
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{nextTaskID: 1, tasks: map[uint]domain.Task{}, idem: map[string]domain.IdempotencyRecord{}}
}

func (m *memoryRepo) CreateUser(context.Context, *domain.User) error { return nil }
func (m *memoryRepo) FindUserByEmail(context.Context, string) (*domain.User, error) {
	return nil, repository.ErrNotFound
}
func (m *memoryRepo) FindUserByID(context.Context, uint) (*domain.User, error) {
	return nil, repository.ErrNotFound
}
func (m *memoryRepo) ListTasks(context.Context, domain.User, repository.TaskFilter) ([]domain.Task, int64, error) {
	return nil, 0, nil
}
func (m *memoryRepo) FindTaskForUser(context.Context, uint, domain.User) (*domain.Task, error) {
	return nil, repository.ErrNotFound
}
func (m *memoryRepo) UpdateTask(context.Context, *domain.Task) error { return nil }
func (m *memoryRepo) DeleteTask(context.Context, *domain.Task) error { return nil }
func (m *memoryRepo) AssignTaskTx(context.Context, uint, uint, uint) (*domain.Task, error) {
	return nil, errors.New("not implemented")
}

func (m *memoryRepo) CreateTask(_ context.Context, task *domain.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task.ID = m.nextTaskID
	m.nextTaskID++
	m.tasks[task.ID] = *task
	m.createdTasks++
	return nil
}

func (m *memoryRepo) FindValidIdempotency(_ context.Context, key string, userID uint, now time.Time) (*domain.IdempotencyRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.idem[key]
	if !ok || rec.UserID != userID || !rec.ExpiresAt.After(now) {
		return nil, repository.ErrNotFound
	}
	copy := rec
	return &copy, nil
}

func (m *memoryRepo) CreateIdempotency(_ context.Context, rec *domain.IdempotencyRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idem[rec.Key] = *rec
	return nil
}
