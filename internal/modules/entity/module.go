package entity

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
)

type Module struct {
	db     *database.PostgresDB
	logger *zap.Logger
}

type Config struct {
	DB     *database.PostgresDB
	Logger *zap.Logger
}

func NewModule(cfg Config) (*Module, error) {
	return &Module{
		db:     cfg.DB,
		logger: cfg.Logger,
	}, nil
}

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Route("/entities", func(r chi.Router) {
		r.Get("/", m.listEntities)
		r.Post("/", m.createEntity)
		r.Get("/hierarchy", m.getHierarchy)
		r.Get("/{id}", m.getEntity)
		r.Put("/{id}", m.updateEntity)
		r.Delete("/{id}", m.deleteEntity)
		r.Post("/{id}/activate", m.activateEntity)
		r.Post("/{id}/suspend", m.suspendEntity)
		r.Get("/{id}/settings", m.getSettings)
		r.Put("/{id}/settings", m.updateSettings)
	})
}

type Entity struct {
	ID                 string          `json:"id"`
	Code               string          `json:"code"`
	Name               string          `json:"name"`
	BaseCurrency       string          `json:"base_currency"`
	FiscalYearEndMonth int             `json:"fiscal_year_end_month"`
	IsActive           bool            `json:"is_active"`
	Settings           json.RawMessage `json:"settings"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type EntitySettings struct {
	EntityID           string          `json:"entity_id"`
	FiscalYearEndMonth int             `json:"fiscal_year_end_month"`
	Settings           json.RawMessage `json:"settings"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type EntityHierarchyNode struct {
	Entity   Entity                `json:"entity"`
	Children []EntityHierarchyNode `json:"children"`
}

func (m *Module) listEntities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT id, code, name, base_currency, fiscal_year_end_month, is_active, settings, created_at, updated_at
		FROM entities
		ORDER BY code
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query entities", zap.Error(err))
		http.Error(w, "Failed to fetch entities", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Code, &e.Name, &e.BaseCurrency, &e.FiscalYearEndMonth, &e.IsActive, &e.Settings, &e.CreatedAt, &e.UpdatedAt); err != nil {
			m.logger.Error("Failed to scan entity", zap.Error(err))
			continue
		}
		entities = append(entities, e)
	}

	if entities == nil {
		entities = []Entity{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"entities": entities, "total": len(entities)})
}

func (m *Module) createEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Code               string          `json:"code"`
		Name               string          `json:"name"`
		BaseCurrency       string          `json:"base_currency"`
		FiscalYearEndMonth int             `json:"fiscal_year_end_month"`
		Settings           json.RawMessage `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FiscalYearEndMonth == 0 {
		req.FiscalYearEndMonth = 12
	}
	if req.BaseCurrency == "" {
		req.BaseCurrency = "USD"
	}
	if req.Settings == nil {
		req.Settings = json.RawMessage("{}")
	}

	id := ulid.Make().String()

	query := `
		INSERT INTO entities (id, code, name, base_currency, fiscal_year_end_month, settings)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, code, name, base_currency, fiscal_year_end_month, is_active, settings, created_at, updated_at
	`

	var entity Entity
	err := m.db.QueryRowContext(ctx, query, id, req.Code, req.Name, req.BaseCurrency, req.FiscalYearEndMonth, req.Settings).Scan(
		&entity.ID, &entity.Code, &entity.Name, &entity.BaseCurrency, &entity.FiscalYearEndMonth, &entity.IsActive, &entity.Settings, &entity.CreatedAt, &entity.UpdatedAt,
	)
	if err != nil {
		m.logger.Error("Failed to create entity", zap.Error(err))
		http.Error(w, "Failed to create entity", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, entity)
}

func (m *Module) getEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `
		SELECT id, code, name, base_currency, fiscal_year_end_month, is_active, settings, created_at, updated_at
		FROM entities
		WHERE id = $1
	`

	var entity Entity
	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&entity.ID, &entity.Code, &entity.Name, &entity.BaseCurrency, &entity.FiscalYearEndMonth, &entity.IsActive, &entity.Settings, &entity.CreatedAt, &entity.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to get entity", zap.Error(err))
		http.Error(w, "Failed to fetch entity", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, entity)
}

func (m *Module) updateEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var req struct {
		Code               string          `json:"code"`
		Name               string          `json:"name"`
		BaseCurrency       string          `json:"base_currency"`
		FiscalYearEndMonth int             `json:"fiscal_year_end_month"`
		Settings           json.RawMessage `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE entities
		SET code = $2, name = $3, base_currency = $4, fiscal_year_end_month = $5, settings = $6
		WHERE id = $1
		RETURNING id, code, name, base_currency, fiscal_year_end_month, is_active, settings, created_at, updated_at
	`

	var entity Entity
	err := m.db.QueryRowContext(ctx, query, id, req.Code, req.Name, req.BaseCurrency, req.FiscalYearEndMonth, req.Settings).Scan(
		&entity.ID, &entity.Code, &entity.Name, &entity.BaseCurrency, &entity.FiscalYearEndMonth, &entity.IsActive, &entity.Settings, &entity.CreatedAt, &entity.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to update entity", zap.Error(err))
		http.Error(w, "Failed to update entity", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, entity)
}

func (m *Module) deleteEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `DELETE FROM entities WHERE id = $1`
	result, err := m.db.ExecContext(ctx, query, id)
	if err != nil {
		m.logger.Error("Failed to delete entity", zap.Error(err))
		http.Error(w, "Failed to delete entity", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (m *Module) activateEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `UPDATE entities SET is_active = TRUE WHERE id = $1 RETURNING id`
	var entityID string
	err := m.db.QueryRowContext(ctx, query, id).Scan(&entityID)
	if err == sql.ErrNoRows {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to activate entity", zap.Error(err))
		http.Error(w, "Failed to activate entity", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "activated", "id": entityID})
}

func (m *Module) suspendEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `UPDATE entities SET is_active = FALSE WHERE id = $1 RETURNING id`
	var entityID string
	err := m.db.QueryRowContext(ctx, query, id).Scan(&entityID)
	if err == sql.ErrNoRows {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to suspend entity", zap.Error(err))
		http.Error(w, "Failed to suspend entity", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "suspended", "id": entityID})
}

func (m *Module) getSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `
		SELECT id, fiscal_year_end_month, settings, updated_at
		FROM entities
		WHERE id = $1
	`

	var settings EntitySettings
	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&settings.EntityID, &settings.FiscalYearEndMonth, &settings.Settings, &settings.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to get entity settings", zap.Error(err))
		http.Error(w, "Failed to fetch settings", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

func (m *Module) updateSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var req struct {
		FiscalYearEndMonth int             `json:"fiscal_year_end_month"`
		Settings           json.RawMessage `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE entities
		SET fiscal_year_end_month = $2, settings = $3
		WHERE id = $1
		RETURNING id, fiscal_year_end_month, settings, updated_at
	`

	var settings EntitySettings
	err := m.db.QueryRowContext(ctx, query, id, req.FiscalYearEndMonth, req.Settings).Scan(
		&settings.EntityID, &settings.FiscalYearEndMonth, &settings.Settings, &settings.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to update entity settings", zap.Error(err))
		http.Error(w, "Failed to update settings", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, settings)
}

func (m *Module) getHierarchy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT id, code, name, base_currency, fiscal_year_end_month, is_active, settings, created_at, updated_at
		FROM entities
		ORDER BY code
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query entities for hierarchy", zap.Error(err))
		http.Error(w, "Failed to fetch hierarchy", http.StatusInternalServerError)
		return
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			m.logger.Error("Failed to close rows", zap.Error(err))
		}
	}()

	var hierarchy []EntityHierarchyNode
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Code, &e.Name, &e.BaseCurrency, &e.FiscalYearEndMonth, &e.IsActive, &e.Settings, &e.CreatedAt, &e.UpdatedAt); err != nil {
			m.logger.Error("Failed to scan entity", zap.Error(err))
			continue
		}
		hierarchy = append(hierarchy, EntityHierarchyNode{
			Entity:   e,
			Children: []EntityHierarchyNode{},
		})
	}

	if hierarchy == nil {
		hierarchy = []EntityHierarchyNode{}
	}

	respondJSON(w, http.StatusOK, hierarchy)
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
