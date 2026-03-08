package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

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
		_ = json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{
		"error":   http.StatusText(status),
		"message": message,
	})
}

type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
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

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
