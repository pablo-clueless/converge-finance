package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type AccountHandler struct {
	*Handler
	accountRepo repository.AccountRepository
	auditLogger *audit.Logger
}

func NewAccountHandler(
	logger *zap.Logger,
	accountRepo repository.AccountRepository,
	auditLogger *audit.Logger,
) *AccountHandler {
	return &AccountHandler{
		Handler:     NewHandler(logger),
		accountRepo: accountRepo,
		auditLogger: auditLogger,
	}
}

func (h *AccountHandler) RegisterRoutes(r chi.Router) {
	r.Route("/accounts", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/tree", h.GetTree)
		r.Get("/{id}", h.Get)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/activate", h.Activate)
		r.Post("/{id}/deactivate", h.Deactivate)
	})
}

type CreateAccountRequest struct {
	ParentID    *string `json:"parent_id,omitempty"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Subtype     string  `json:"subtype,omitempty"`
	Currency    string  `json:"currency"`
	IsControl   bool    `json:"is_control"`
	Description string  `json:"description,omitempty"`
}

type UpdateAccountRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Subtype     *string `json:"subtype,omitempty"`
}

type AccountResponse struct {
	ID            string            `json:"id"`
	EntityID      string            `json:"entity_id"`
	ParentID      *string           `json:"parent_id,omitempty"`
	Code          string            `json:"code"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Subtype       string            `json:"subtype,omitempty"`
	Currency      string            `json:"currency"`
	IsControl     bool              `json:"is_control"`
	IsPosting     bool              `json:"is_posting"`
	IsActive      bool              `json:"is_active"`
	NormalBalance string            `json:"normal_balance"`
	Description   string            `json:"description,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	Children      []AccountResponse `json:"children,omitempty"`
}

func toAccountResponse(a *domain.Account) AccountResponse {
	resp := AccountResponse{
		ID:            a.ID.String(),
		EntityID:      a.EntityID.String(),
		Code:          a.Code,
		Name:          a.Name,
		Type:          string(a.Type),
		Subtype:       string(a.Subtype),
		Currency:      a.Currency.Code,
		IsControl:     a.IsControl,
		IsPosting:     a.IsPosting,
		IsActive:      a.IsActive,
		NormalBalance: string(a.NormalBalance),
		Description:   a.Description,
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
	}

	if a.ParentID != nil {
		parentID := a.ParentID.String()
		resp.ParentID = &parentID
	}

	if len(a.Children) > 0 {
		resp.Children = make([]AccountResponse, len(a.Children))
		for i, child := range a.Children {
			resp.Children[i] = toAccountResponse(&child)
		}
	}

	return resp
}

func (h *AccountHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	filter := domain.AccountFilter{
		EntityID:  entityID,
		Type:      (*domain.AccountType)(getStringQuery(r, "type")),
		IsActive:  getBoolQuery(r, "is_active"),
		IsPosting: getBoolQuery(r, "is_posting"),
		IsControl: getBoolQuery(r, "is_control"),
		Limit:     getIntQuery(r, "limit", 100),
		Offset:    getIntQuery(r, "offset", 0),
	}

	if parentID := r.URL.Query().Get("parent_id"); parentID != "" {
		id, err := common.Parse(parentID)
		if err == nil {
			filter.ParentID = &id
		}
	}

	accounts, err := h.accountRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list accounts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to list accounts")
		return
	}

	total, err := h.accountRepo.Count(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to count accounts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to count accounts")
		return
	}

	responses := make([]AccountResponse, len(accounts))
	for i, a := range accounts {
		responses[i] = toAccountResponse(&a)
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(accounts)) < total,
	})
}

func (h *AccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	account, err := h.accountRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Account not found")
			return
		}
		h.logger.Error("Failed to get account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get account")
		return
	}

	respondJSON(w, http.StatusOK, toAccountResponse(account))
}

func (h *AccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	var req CreateAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid currency code")
		return
	}

	account, err := domain.NewAccount(
		entityID,
		req.Code,
		req.Name,
		domain.AccountType(req.Type),
		currency,
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	account.Description = req.Description
	if req.Subtype != "" {
		account.Subtype = domain.AccountSubtype(req.Subtype)
	}
	if req.IsControl {
		account.MakeControl()
	}

	if req.ParentID != nil {
		parentID, err := common.Parse(*req.ParentID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid parent ID")
			return
		}
		if err := account.SetParent(parentID); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := account.Validate(); err != nil {
		if ve, ok := err.(*common.ValidationError); ok {
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	exists, err := h.accountRepo.ExistsByCode(ctx, entityID, req.Code)
	if err != nil {
		h.logger.Error("Failed to check account code", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}
	if exists {
		respondError(w, http.StatusConflict, "Account code already exists")
		return
	}

	if err := h.accountRepo.Create(ctx, account); err != nil {
		h.logger.Error("Failed to create account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to create account")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogCreate(ctx, "gl.account", account.ID, map[string]any{
			"code": account.Code,
			"name": account.Name,
			"type": account.Type,
		})
	}

	respondJSON(w, http.StatusCreated, toAccountResponse(account))
}

func (h *AccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	var req UpdateAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	account, err := h.accountRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Account not found")
			return
		}
		h.logger.Error("Failed to get account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to update account")
		return
	}

	if req.Name != nil {
		account.Name = *req.Name
	}
	if req.Description != nil {
		account.Description = *req.Description
	}
	if req.Subtype != nil {
		account.Subtype = domain.AccountSubtype(*req.Subtype)
	}
	account.UpdatedAt = time.Now()

	if err := account.Validate(); err != nil {
		if ve, ok := err.(*common.ValidationError); ok {
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.accountRepo.Update(ctx, account); err != nil {
		h.logger.Error("Failed to update account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to update account")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogUpdate(ctx, "gl.account", account.ID, map[string]any{
			"name":        account.Name,
			"description": account.Description,
		})
	}

	respondJSON(w, http.StatusOK, toAccountResponse(account))
}

func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	account, err := h.accountRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Account not found")
			return
		}
		h.logger.Error("Failed to get account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to delete account")
		return
	}

	if err := h.accountRepo.Delete(ctx, id); err != nil {
		h.logger.Error("Failed to delete account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to delete account")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogDelete(ctx, "gl.account", account.ID, map[string]any{
			"code": account.Code,
			"name": account.Name,
		})
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "Account deleted"})
}

func (h *AccountHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	accounts, err := h.accountRepo.GetTree(ctx, entityID)
	if err != nil {
		h.logger.Error("Failed to get account tree", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get account tree")
		return
	}

	responses := make([]AccountResponse, len(accounts))
	for i, a := range accounts {
		responses[i] = toAccountResponse(&a)
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *AccountHandler) Activate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	account, err := h.accountRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Account not found")
			return
		}
		h.logger.Error("Failed to get account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to activate account")
		return
	}

	account.Activate()

	if err := h.accountRepo.Update(ctx, account); err != nil {
		h.logger.Error("Failed to activate account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to activate account")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogAction(ctx, "gl.account", account.ID, "activated", nil)
	}

	respondJSON(w, http.StatusOK, toAccountResponse(account))
}

func (h *AccountHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	account, err := h.accountRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Account not found")
			return
		}
		h.logger.Error("Failed to get account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to deactivate account")
		return
	}

	if err := account.Deactivate(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.accountRepo.Update(ctx, account); err != nil {
		h.logger.Error("Failed to deactivate account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to deactivate account")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogAction(ctx, "gl.account", account.ID, "deactivated", nil)
	}

	respondJSON(w, http.StatusOK, toAccountResponse(account))
}
