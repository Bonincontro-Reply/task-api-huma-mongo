package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Task is the domain model returned by the API.
type Task struct {
	ID        string    `json:"id" bson:"_id,omitempty"`
	Title     string    `json:"title" bson:"title"`
	Done      bool      `json:"done" bson:"done"`
	Tags      []string  `json:"tags,omitempty" bson:"tags,omitempty"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	Internal  string    `json:"-" bson:"-"`
	// internalNote is unexported, so json/bson ignore it even with tags.
	internalNote string `json:"internalNote" bson:"internalNote"`
}

type CreateTaskRequest struct {
	Title string
	Done  *bool
	Tags  []string
}

type UpdateTaskRequest struct {
	Title *string
	Done  *bool
	Tags  *[]string
}

type TaskFilter struct {
	Done *bool
	Tag  string
}

var (
	ErrNotFound   = errors.New("task not found")
	ErrInvalidID  = errors.New("invalid task id")
	ErrValidation = errors.New("validation failed")
)

type ValidationError struct {
	Field   string
	Message string
	Value   any
}

func (e *ValidationError) Error() string {
	return e.Message
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

type TaskRepository interface {
	Create(ctx context.Context, task Task) (*Task, error)
	Get(ctx context.Context, id string) (*Task, error)
	List(ctx context.Context, filter TaskFilter) ([]Task, error)
	Update(ctx context.Context, id string, update UpdateTaskRequest) (*Task, error)
	Delete(ctx context.Context, id string) error
	Ping(ctx context.Context) error
}

type Service struct {
	repo TaskRepository
	now  func() time.Time
}

func New(repo TaskRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) Create(ctx context.Context, req CreateTaskRequest) (*Task, error) {
	title := strings.TrimSpace(req.Title)
	if len(title) < 3 {
		return nil, &ValidationError{
			Field:   "title",
			Message: "title must be at least 3 characters",
			Value:   req.Title,
		}
	}
	done := false
	if req.Done != nil {
		done = *req.Done
	}

	task := Task{
		Title:        title,
		Done:         done,
		Tags:         req.Tags,
		CreatedAt:    s.now().UTC(),
		Internal:     "internal",
		internalNote: "ignored",
	}

	return s.repo.Create(ctx, task)
}

func (s *Service) Get(ctx context.Context, id string) (*Task, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, filter TaskFilter) ([]Task, error) {
	return s.repo.List(ctx, filter)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateTaskRequest) (*Task, error) {
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if len(trimmed) < 3 {
			return nil, &ValidationError{
				Field:   "title",
				Message: "title must be at least 3 characters",
				Value:   *req.Title,
			}
		}
		req.Title = &trimmed
	}
	if req.Title == nil && req.Done == nil && req.Tags == nil {
		return nil, &ValidationError{
			Field:   "body",
			Message: "at least one field must be provided",
		}
	}

	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}
