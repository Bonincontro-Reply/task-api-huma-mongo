package api

import (
	"reflect"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"task-api-huma-mongo/internal/service"
)

type HealthOutput struct {
	Body HealthResponse
}

type HealthResponse struct {
	Status string    `json:"status"`
	Mongo  string    `json:"mongo"`
	Time   time.Time `json:"time"`
}

type TaskOutput struct {
	Body service.Task
}

type ListTasksOutput struct {
	Body ListTasksResponse
}

type ListTasksResponse struct {
	Items []service.Task `json:"items"`
	Count int            `json:"count"`
}

type CreateTaskInput struct {
	Body CreateTaskBody
}

type CreateTaskBody struct {
	Title string   `json:"title" minLength:"3"`
	Done  *bool    `json:"done,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

type UpdateTaskInput struct {
	ID   string `path:"id"`
	Body UpdateTaskBody
}

type UpdateTaskBody struct {
	Title *string   `json:"title,omitempty" minLength:"3"`
	Done  *bool     `json:"done,omitempty"`
	Tags  *[]string `json:"tags,omitempty"`
}

type TaskIDInput struct {
	ID string `path:"id"`
}

type ListTasksInput struct {
	Done OptionalParam[bool] `query:"done"`
	Tag  string              `query:"tag"`
}

type OptionalParam[T any] struct {
	Value T
	IsSet bool
}

func (o OptionalParam[T]) Schema(r huma.Registry) *huma.Schema {
	return huma.SchemaFromType(r, reflect.TypeOf(o.Value))
}

func (o *OptionalParam[T]) Receiver() reflect.Value {
	return reflect.ValueOf(o).Elem().Field(0)
}

func (o *OptionalParam[T]) OnParamSet(isSet bool, parsed any) {
	o.IsSet = isSet
}

func (i *CreateTaskInput) Resolve(ctx huma.Context) []error {
	title := strings.TrimSpace(i.Body.Title)
	if len(title) < 3 {
		return []error{&huma.ErrorDetail{Message: "title must be at least 3 characters", Location: "body.title", Value: i.Body.Title}}
	}
	i.Body.Title = title
	return nil
}

func (i *UpdateTaskInput) Resolve(ctx huma.Context) []error {
	if i.Body.Title != nil {
		trimmed := strings.TrimSpace(*i.Body.Title)
		if len(trimmed) < 3 {
			return []error{&huma.ErrorDetail{Message: "title must be at least 3 characters", Location: "body.title", Value: *i.Body.Title}}
		}
		i.Body.Title = &trimmed
	}
	if i.Body.Title == nil && i.Body.Done == nil && i.Body.Tags == nil {
		return []error{&huma.ErrorDetail{Message: "at least one field must be provided", Location: "body"}}
	}
	return nil
}

func (i *ListTasksInput) Resolve(ctx huma.Context) []error {
	i.Tag = strings.TrimSpace(i.Tag)
	return nil
}
