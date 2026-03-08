package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Handler struct {
	logger *zap.Logger
}

func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{logger: logger}
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

func respondValidationError(w http.ResponseWriter, err *common.ValidationError) {
	respondJSON(w, http.StatusBadRequest, ValidationErrorResponse{
		Error:  "validation_error",
		Errors: err.Errors,
	})
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type ValidationErrorResponse struct {
	Error  string              `json:"error"`
	Errors []common.FieldError `json:"errors"`
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type PaginatedResponse struct {
	Data    any   `json:"data"`
	Total   int64 `json:"total"`
	Limit   int   `json:"limit"`
	Offset  int   `json:"offset"`
	HasMore bool  `json:"has_more"`
}

func getIDParam(r *http.Request, name string) (common.ID, error) {
	param := chi.URLParam(r, name)
	return common.Parse(param)
}

func getEntityID(r *http.Request) common.ID {
	entityID := auth.GetEntityIDFromContext(r.Context())
	if entityID != "" {
		return common.ID(entityID)
	}
	return common.ID(r.Header.Get("X-Entity-ID"))
}

func getUserID(r *http.Request) common.ID {
	userID := auth.GetUserIDFromContext(r.Context())
	return common.ID(userID)
}

func getIntQuery(r *http.Request, name string, defaultValue int) int {
	value := r.URL.Query().Get(name)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}

func getBoolQuery(r *http.Request, name string) *bool {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil
	}
	boolValue := value == "true" || value == "1"
	return &boolValue
}

func getStringQuery(r *http.Request, name string) *string {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil
	}
	return &value
}

func getDateQuery(r *http.Request, name string) *time.Time {
	value := r.URL.Query().Get(name)
	if value == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}
	return &t
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
