package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"task-api-huma-mongo/internal/service"
)

type contextKey string

const (
	CorrelationHeader  = "X-Request-Id"
	correlationIDKey   = contextKey("correlation_id")
	defaultProblemType = "about:blank"
)

type InvalidParam struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type APIError struct {
	huma.ErrorModel
	ErrorCode     string         `json:"error,omitempty"`
	Message       string         `json:"message,omitempty"`
	CorrelationID string         `json:"correlationId,omitempty"`
	InvalidParams []InvalidParam `json:"invalidParams,omitempty"`
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Detail
}

func InstallErrorHandler() {
	huma.NewErrorWithContext = func(ctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
		correlationID := ""
		if ctx != nil {
			correlationID = CorrelationIDFromContext(ctx.Context())
		}

		apiErr := &APIError{
			ErrorModel: huma.ErrorModel{
				Type:   defaultProblemType,
				Title:  http.StatusText(status),
				Status: status,
				Detail: msg,
			},
			ErrorCode:     errorCodeFromStatus(status),
			Message:       msg,
			CorrelationID: correlationID,
		}

		for _, err := range errs {
			if err == nil {
				continue
			}
			if detailer, ok := err.(huma.ErrorDetailer); ok {
				apiErr.Errors = append(apiErr.Errors, detailer.ErrorDetail())
				continue
			}
			apiErr.Errors = append(apiErr.Errors, &huma.ErrorDetail{Message: err.Error()})
		}

		return apiErr
	}
}

func NewAPIError(status int, code, message, correlationID string, invalid []InvalidParam) *APIError {
	return &APIError{
		ErrorModel: huma.ErrorModel{
			Type:   defaultProblemType,
			Title:  http.StatusText(status),
			Status: status,
			Detail: message,
		},
		ErrorCode:     code,
		Message:       message,
		CorrelationID: correlationID,
		InvalidParams: invalid,
	}
}

func MapServiceError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	correlationID := CorrelationIDFromContext(ctx)

	var vErr *service.ValidationError
	switch {
	case errors.As(err, &vErr):
		invalid := []InvalidParam{{Name: vErr.Field, Reason: vErr.Message}}
		return NewAPIError(http.StatusBadRequest, "bad_request", vErr.Message, correlationID, invalid)
	case errors.Is(err, service.ErrInvalidID):
		invalid := []InvalidParam{{Name: "id", Reason: "must be a valid ObjectID hex"}}
		return NewAPIError(http.StatusBadRequest, "bad_request", "invalid task id", correlationID, invalid)
	case errors.Is(err, service.ErrNotFound):
		return NewAPIError(http.StatusNotFound, "not_found", "task not found", correlationID, nil)
	default:
		return NewAPIError(http.StatusInternalServerError, "internal_error", "internal server error", correlationID, nil)
	}
}

func CorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := r.Header.Get(CorrelationHeader)
		if correlationID == "" {
			correlationID = newCorrelationID()
		}
		w.Header().Set(CorrelationHeader, correlationID)
		ctx := WithCorrelationID(r.Context(), correlationID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value := ctx.Value(correlationIDKey)
	if value == nil {
		return ""
	}
	if id, ok := value.(string); ok {
		return id
	}
	return ""
}

func newCorrelationID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func errorCodeFromStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		return "error"
	}
}
