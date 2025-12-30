package api

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"task-api-huma-mongo/internal/service"
)

func RegisterRoutes(api huma.API, svc *service.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Health check",
	}, func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		if err := svc.Ping(ctx); err != nil {
			return nil, NewAPIError(
				http.StatusServiceUnavailable,
				"service_unavailable",
				"mongo unavailable",
				CorrelationIDFromContext(ctx),
				nil,
			)
		}

		return &HealthOutput{Body: HealthResponse{
			Status: "ok",
			Mongo:  "ok",
			Time:   time.Now().UTC(),
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-task",
		Method:        http.MethodPost,
		Path:          "/tasks",
		Summary:       "Create a task",
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, input *CreateTaskInput) (*TaskOutput, error) {
		task, err := svc.Create(ctx, service.CreateTaskRequest{
			Title: input.Body.Title,
			Done:  input.Body.Done,
			Tags:  input.Body.Tags,
		})
		if err != nil {
			return nil, MapServiceError(ctx, err)
		}

		return &TaskOutput{Body: *task}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-tasks",
		Method:      http.MethodGet,
		Path:        "/tasks",
		Summary:     "List tasks",
	}, func(ctx context.Context, input *ListTasksInput) (*ListTasksOutput, error) {
		var done *bool
		if input.Done.IsSet {
			value := input.Done.Value
			done = &value
		}
		tasks, err := svc.List(ctx, service.TaskFilter{
			Done: done,
			Tag:  input.Tag,
		})
		if err != nil {
			return nil, MapServiceError(ctx, err)
		}

		return &ListTasksOutput{Body: ListTasksResponse{
			Items: tasks,
			Count: len(tasks),
		}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-task",
		Method:      http.MethodGet,
		Path:        "/tasks/{id}",
		Summary:     "Get task by ID",
	}, func(ctx context.Context, input *TaskIDInput) (*TaskOutput, error) {
		task, err := svc.Get(ctx, input.ID)
		if err != nil {
			return nil, MapServiceError(ctx, err)
		}

		return &TaskOutput{Body: *task}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-task",
		Method:      http.MethodPatch,
		Path:        "/tasks/{id}",
		Summary:     "Update task",
	}, func(ctx context.Context, input *UpdateTaskInput) (*TaskOutput, error) {
		task, err := svc.Update(ctx, input.ID, service.UpdateTaskRequest{
			Title: input.Body.Title,
			Done:  input.Body.Done,
			Tags:  input.Body.Tags,
		})
		if err != nil {
			return nil, MapServiceError(ctx, err)
		}

		return &TaskOutput{Body: *task}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-task",
		Method:        http.MethodDelete,
		Path:          "/tasks/{id}",
		Summary:       "Delete task",
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, input *TaskIDInput) (*struct{}, error) {
		if err := svc.Delete(ctx, input.ID); err != nil {
			return nil, MapServiceError(ctx, err)
		}
		return nil, nil
	})
}
