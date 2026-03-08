package user

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
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
	r.Route("/users", func(r chi.Router) {
		r.Get("/", m.listUsers)
		r.Post("/", m.createUser)
		r.Get("/me", m.getCurrentUser)
		r.Put("/me", m.updateCurrentUser)
		r.Get("/me/preferences", m.getPreferences)
		r.Put("/me/preferences", m.updatePreferences)
		r.Get("/{id}", m.getUser)
		r.Put("/{id}", m.updateUser)
		r.Delete("/{id}", m.deleteUser)
		r.Post("/{id}/activate", m.activateUser)
		r.Post("/{id}/deactivate", m.deactivateUser)
		r.Get("/{id}/roles", m.getUserRoles)
		r.Put("/{id}/roles", m.updateUserRoles)
		r.Get("/{id}/entity-access", m.getEntityAccess)
		r.Put("/{id}/entity-access", m.updateEntityAccess)
	})

	r.Route("/roles", func(r chi.Router) {
		r.Get("/", m.listRoles)
		r.Post("/", m.createRole)
		r.Get("/{id}", m.getRole)
		r.Put("/{id}", m.updateRole)
		r.Delete("/{id}", m.deleteRole)
	})
}

type User struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	FirstName   string     `json:"first_name"`
	LastName    string     `json:"last_name"`
	DisplayName string     `json:"display_name"`
	IsActive    bool       `json:"is_active"`
	IsSystem    bool       `json:"is_system"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type UserPreferences struct {
	Locale               string `json:"locale"`
	Timezone             string `json:"timezone"`
	DateFormat           string `json:"date_format"`
	NumberFormat         string `json:"number_format"`
	DefaultEntityID      string `json:"default_entity_id"`
	Theme                string `json:"theme"`
	NotificationsEnabled bool   `json:"notifications_enabled"`
	EmailNotifications   bool   `json:"email_notifications"`
}

type Role struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	IsSystem    bool      `json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserEntityAccess struct {
	EntityID   string   `json:"entity_id"`
	EntityCode string   `json:"entity_code"`
	EntityName string   `json:"entity_name"`
	Roles      []string `json:"roles"`
	IsDefault  bool     `json:"is_default"`
}

func (m *Module) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT id, email, first_name, last_name, is_active, is_system, last_login_at, created_at, updated_at
		FROM users
		ORDER BY email
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query users", zap.Error(err))
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.IsActive, &u.IsSystem, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt); err != nil {
			m.logger.Error("Failed to scan user", zap.Error(err))
			continue
		}
		u.DisplayName = u.FirstName + " " + u.LastName
		users = append(users, u)
	}

	if users == nil {
		users = []User{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"users": users, "total": len(users)})
}

func (m *Module) createUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Email     string `json:"email"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		m.logger.Error("Failed to hash password", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	id := ulid.Make().String()

	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, email, first_name, last_name, is_active, is_system, last_login_at, created_at, updated_at
	`

	var user User
	err = m.db.QueryRowContext(ctx, query, id, req.Email, string(hashedPassword), req.FirstName, req.LastName).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.IsActive, &user.IsSystem, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		m.logger.Error("Failed to create user", zap.Error(err))
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}
	user.DisplayName = user.FirstName + " " + user.LastName

	respondJSON(w, http.StatusCreated, user)
}

func (m *Module) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.GetUserIDFromContext(ctx)

	query := `
		SELECT id, email, first_name, last_name, is_active, is_system, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user User
	err := m.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.IsActive, &user.IsSystem, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to get current user", zap.Error(err))
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return
	}
	user.DisplayName = user.FirstName + " " + user.LastName

	respondJSON(w, http.StatusOK, user)
}

func (m *Module) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.GetUserIDFromContext(ctx)

	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE users
		SET first_name = $2, last_name = $3
		WHERE id = $1
		RETURNING id, email, first_name, last_name, is_active, is_system, last_login_at, created_at, updated_at
	`

	var user User
	err := m.db.QueryRowContext(ctx, query, userID, req.FirstName, req.LastName).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.IsActive, &user.IsSystem, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to update user", zap.Error(err))
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}
	user.DisplayName = user.FirstName + " " + user.LastName

	respondJSON(w, http.StatusOK, user)
}

func (m *Module) getPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.GetUserIDFromContext(ctx)

	// Get default entity for user
	query := `
		SELECT uea.entity_id
		FROM user_entity_access uea
		WHERE uea.user_id = $1 AND uea.is_default = TRUE
		LIMIT 1
	`

	var defaultEntityID string
	_ = m.db.QueryRowContext(ctx, query, userID).Scan(&defaultEntityID)

	prefs := UserPreferences{
		Locale:               "en-US",
		Timezone:             "America/New_York",
		DateFormat:           "MM/DD/YYYY",
		NumberFormat:         "en-US",
		DefaultEntityID:      defaultEntityID,
		Theme:                "light",
		NotificationsEnabled: true,
		EmailNotifications:   true,
	}
	respondJSON(w, http.StatusOK, prefs)
}

func (m *Module) updatePreferences(w http.ResponseWriter, r *http.Request) {
	var prefs UserPreferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, prefs)
}

func (m *Module) getUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `
		SELECT id, email, first_name, last_name, is_active, is_system, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user User
	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.IsActive, &user.IsSystem, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to get user", zap.Error(err))
		http.Error(w, "Failed to fetch user", http.StatusInternalServerError)
		return
	}
	user.DisplayName = user.FirstName + " " + user.LastName

	respondJSON(w, http.StatusOK, user)
}

func (m *Module) updateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var req struct {
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE users
		SET email = $2, first_name = $3, last_name = $4
		WHERE id = $1
		RETURNING id, email, first_name, last_name, is_active, is_system, last_login_at, created_at, updated_at
	`

	var user User
	err := m.db.QueryRowContext(ctx, query, id, req.Email, req.FirstName, req.LastName).Scan(
		&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.IsActive, &user.IsSystem, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to update user", zap.Error(err))
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}
	user.DisplayName = user.FirstName + " " + user.LastName

	respondJSON(w, http.StatusOK, user)
}

func (m *Module) deleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `DELETE FROM users WHERE id = $1 AND is_system = FALSE`
	result, err := m.db.ExecContext(ctx, query, id)
	if err != nil {
		m.logger.Error("Failed to delete user", zap.Error(err))
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "User not found or is a system user", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (m *Module) activateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `UPDATE users SET is_active = TRUE WHERE id = $1 RETURNING id`
	var userID string
	err := m.db.QueryRowContext(ctx, query, id).Scan(&userID)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to activate user", zap.Error(err))
		http.Error(w, "Failed to activate user", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "activated", "id": userID})
}

func (m *Module) deactivateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `UPDATE users SET is_active = FALSE WHERE id = $1 AND is_system = FALSE RETURNING id`
	var userID string
	err := m.db.QueryRowContext(ctx, query, id).Scan(&userID)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found or is a system user", http.StatusNotFound)
		return
	}
	if err != nil {
		m.logger.Error("Failed to deactivate user", zap.Error(err))
		http.Error(w, "Failed to deactivate user", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deactivated", "id": userID})
}

func (m *Module) getUserRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `
		SELECT DISTINCT unnest(uea.roles) as role
		FROM user_entity_access uea
		WHERE uea.user_id = $1
	`

	rows, err := m.db.QueryContext(ctx, query, id)
	if err != nil {
		m.logger.Error("Failed to query user roles", zap.Error(err))
		http.Error(w, "Failed to fetch roles", http.StatusInternalServerError)
		return
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			continue
		}
		roles = append(roles, role)
	}

	if roles == nil {
		roles = []string{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

func (m *Module) updateUserRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var req struct {
		EntityID string   `json:"entity_id"`
		Roles    []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE user_entity_access
		SET roles = $3
		WHERE user_id = $1 AND entity_id = $2
		RETURNING roles
	`

	var roles pq.StringArray
	err := m.db.QueryRowContext(ctx, query, id, req.EntityID, pq.Array(req.Roles)).Scan(&roles)
	if err == sql.ErrNoRows {
		// Insert new access record
		insertQuery := `
			INSERT INTO user_entity_access (id, user_id, entity_id, roles)
			VALUES ($1, $2, $3, $4)
			RETURNING roles
		`
		accessID := ulid.Make().String()
		err = m.db.QueryRowContext(ctx, insertQuery, accessID, id, req.EntityID, pq.Array(req.Roles)).Scan(&roles)
	}
	if err != nil {
		m.logger.Error("Failed to update user roles", zap.Error(err))
		http.Error(w, "Failed to update roles", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"roles": []string(roles)})
}

func (m *Module) getEntityAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `
		SELECT uea.entity_id, e.code, e.name, uea.roles, uea.is_default
		FROM user_entity_access uea
		JOIN entities e ON e.id = uea.entity_id
		WHERE uea.user_id = $1
		ORDER BY e.code
	`

	rows, err := m.db.QueryContext(ctx, query, id)
	if err != nil {
		m.logger.Error("Failed to query entity access", zap.Error(err))
		http.Error(w, "Failed to fetch entity access", http.StatusInternalServerError)
		return
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var access []UserEntityAccess
	for rows.Next() {
		var a UserEntityAccess
		var roles pq.StringArray
		if err := rows.Scan(&a.EntityID, &a.EntityCode, &a.EntityName, &roles, &a.IsDefault); err != nil {
			m.logger.Error("Failed to scan entity access", zap.Error(err))
			continue
		}
		a.Roles = []string(roles)
		access = append(access, a)
	}

	if access == nil {
		access = []UserEntityAccess{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"entities": access})
}

func (m *Module) updateEntityAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var req struct {
		EntityID  string   `json:"entity_id"`
		Roles     []string `json:"roles"`
		IsDefault bool     `json:"is_default"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// If setting as default, clear other defaults first
	if req.IsDefault {
		_, _ = m.db.ExecContext(ctx, "UPDATE user_entity_access SET is_default = FALSE WHERE user_id = $1", id)
	}

	query := `
		INSERT INTO user_entity_access (id, user_id, entity_id, roles, is_default)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, entity_id) DO UPDATE SET roles = $4, is_default = $5
		RETURNING entity_id
	`

	accessID := ulid.Make().String()
	var entityID string
	err := m.db.QueryRowContext(ctx, query, accessID, id, req.EntityID, pq.Array(req.Roles), req.IsDefault).Scan(&entityID)
	if err != nil {
		m.logger.Error("Failed to update entity access", zap.Error(err))
		http.Error(w, "Failed to update entity access", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "updated", "entity_id": entityID})
}

func (m *Module) listRoles(w http.ResponseWriter, r *http.Request) {
	// Roles are stored as text arrays in user_entity_access, return predefined system roles
	roles := []Role{
		{ID: "01HQ001", Name: "admin", Description: "Full system access", Permissions: []string{"*"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "01HQ002", Name: "accountant", Description: "Accounting operations", Permissions: []string{"gl:*", "ap:read", "ar:read"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "01HQ003", Name: "viewer", Description: "Read-only access", Permissions: []string{"*:read"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "01HQ004", Name: "ap_clerk", Description: "Accounts Payable operations", Permissions: []string{"ap:*", "gl:read"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "01HQ005", Name: "ar_clerk", Description: "Accounts Receivable operations", Permissions: []string{"ar:*", "gl:read"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}
	respondJSON(w, http.StatusOK, map[string]any{"roles": roles, "total": len(roles)})
}

func (m *Module) createRole(w http.ResponseWriter, r *http.Request) {
	// Roles are predefined system roles in this implementation
	http.Error(w, "Custom role creation not supported", http.StatusNotImplemented)
}

func (m *Module) getRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	roles := map[string]Role{
		"01HQ001": {ID: "01HQ001", Name: "admin", Description: "Full system access", Permissions: []string{"*"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"01HQ002": {ID: "01HQ002", Name: "accountant", Description: "Accounting operations", Permissions: []string{"gl:*", "ap:read", "ar:read"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		"01HQ003": {ID: "01HQ003", Name: "viewer", Description: "Read-only access", Permissions: []string{"*:read"}, IsSystem: true, CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	if role, ok := roles[id]; ok {
		respondJSON(w, http.StatusOK, role)
		return
	}

	http.Error(w, "Role not found", http.StatusNotFound)
}

func (m *Module) updateRole(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "System role modification not supported", http.StatusNotImplemented)
}

func (m *Module) deleteRole(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "System role deletion not supported", http.StatusNotImplemented)
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
