package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
	"converge-finance.com/m/internal/modules/ic/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
)

type MappingHandler struct {
	*Handler
	mappingRepo repository.AccountMappingRepository
	auditLogger *audit.Logger
}

func NewMappingHandler(h *Handler, mappingRepo repository.AccountMappingRepository, auditLogger *audit.Logger) *MappingHandler {
	return &MappingHandler{
		Handler:     h,
		mappingRepo: mappingRepo,
		auditLogger: auditLogger,
	}
}

type CreateMappingRequest struct {
	FromEntityID         string `json:"from_entity_id"`
	ToEntityID           string `json:"to_entity_id"`
	TransactionType      string `json:"transaction_type"`
	FromDueToAccountID   string `json:"from_due_to_account_id,omitempty"`
	FromDueFromAccountID string `json:"from_due_from_account_id,omitempty"`
	FromRevenueAccountID string `json:"from_revenue_account_id,omitempty"`
	FromExpenseAccountID string `json:"from_expense_account_id,omitempty"`
	ToDueToAccountID     string `json:"to_due_to_account_id,omitempty"`
	ToDueFromAccountID   string `json:"to_due_from_account_id,omitempty"`
	ToRevenueAccountID   string `json:"to_revenue_account_id,omitempty"`
	ToExpenseAccountID   string `json:"to_expense_account_id,omitempty"`
	Description          string `json:"description,omitempty"`
}

type MappingResponse struct {
	ID                   string    `json:"id"`
	FromEntityID         string    `json:"from_entity_id"`
	ToEntityID           string    `json:"to_entity_id"`
	TransactionType      string    `json:"transaction_type"`
	FromDueToAccountID   string    `json:"from_due_to_account_id,omitempty"`
	FromDueFromAccountID string    `json:"from_due_from_account_id,omitempty"`
	FromRevenueAccountID string    `json:"from_revenue_account_id,omitempty"`
	FromExpenseAccountID string    `json:"from_expense_account_id,omitempty"`
	ToDueToAccountID     string    `json:"to_due_to_account_id,omitempty"`
	ToDueFromAccountID   string    `json:"to_due_from_account_id,omitempty"`
	ToRevenueAccountID   string    `json:"to_revenue_account_id,omitempty"`
	ToExpenseAccountID   string    `json:"to_expense_account_id,omitempty"`
	Description          string    `json:"description,omitempty"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func toMappingResponse(m *domain.AccountMapping) MappingResponse {
	resp := MappingResponse{
		ID:              m.ID.String(),
		FromEntityID:    m.FromEntityID.String(),
		ToEntityID:      m.ToEntityID.String(),
		TransactionType: string(m.TransactionType),
		Description:     m.Description,
		IsActive:        m.IsActive,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
	if m.FromDueToAccountID != nil {
		resp.FromDueToAccountID = m.FromDueToAccountID.String()
	}
	if m.FromDueFromAccountID != nil {
		resp.FromDueFromAccountID = m.FromDueFromAccountID.String()
	}
	if m.FromRevenueAccountID != nil {
		resp.FromRevenueAccountID = m.FromRevenueAccountID.String()
	}
	if m.FromExpenseAccountID != nil {
		resp.FromExpenseAccountID = m.FromExpenseAccountID.String()
	}
	if m.ToDueToAccountID != nil {
		resp.ToDueToAccountID = m.ToDueToAccountID.String()
	}
	if m.ToDueFromAccountID != nil {
		resp.ToDueFromAccountID = m.ToDueFromAccountID.String()
	}
	if m.ToRevenueAccountID != nil {
		resp.ToRevenueAccountID = m.ToRevenueAccountID.String()
	}
	if m.ToExpenseAccountID != nil {
		resp.ToExpenseAccountID = m.ToExpenseAccountID.String()
	}
	return resp
}

func (h *MappingHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)

	mappings, err := h.mappingRepo.GetAllForEntity(r.Context(), entityID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []MappingResponse
	for _, m := range mappings {
		responses = append(responses, toMappingResponse(&m))
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *MappingHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateMappingRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fromEntityID, err := common.Parse(req.FromEntityID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid from_entity_id")
		return
	}

	toEntityID, err := common.Parse(req.ToEntityID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid to_entity_id")
		return
	}

	mapping, err := domain.NewAccountMapping(fromEntityID, toEntityID, domain.TransactionType(req.TransactionType))
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	mapping.Description = req.Description

	// Parse and set account IDs
	var fromDueTo, fromDueFrom, fromRevenue, fromExpense *common.ID
	var toDueTo, toDueFrom, toRevenue, toExpense *common.ID

	if req.FromDueToAccountID != "" {
		id, _ := common.Parse(req.FromDueToAccountID)
		fromDueTo = &id
	}
	if req.FromDueFromAccountID != "" {
		id, _ := common.Parse(req.FromDueFromAccountID)
		fromDueFrom = &id
	}
	if req.FromRevenueAccountID != "" {
		id, _ := common.Parse(req.FromRevenueAccountID)
		fromRevenue = &id
	}
	if req.FromExpenseAccountID != "" {
		id, _ := common.Parse(req.FromExpenseAccountID)
		fromExpense = &id
	}
	if req.ToDueToAccountID != "" {
		id, _ := common.Parse(req.ToDueToAccountID)
		toDueTo = &id
	}
	if req.ToDueFromAccountID != "" {
		id, _ := common.Parse(req.ToDueFromAccountID)
		toDueFrom = &id
	}
	if req.ToRevenueAccountID != "" {
		id, _ := common.Parse(req.ToRevenueAccountID)
		toRevenue = &id
	}
	if req.ToExpenseAccountID != "" {
		id, _ := common.Parse(req.ToExpenseAccountID)
		toExpense = &id
	}

	mapping.SetFromAccounts(fromDueTo, fromDueFrom, fromRevenue, fromExpense)
	mapping.SetToAccounts(toDueTo, toDueFrom, toRevenue, toExpense)

	if err := h.mappingRepo.Create(r.Context(), mapping); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toMappingResponse(mapping))
}

func (h *MappingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	mapping, err := h.mappingRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "mapping not found")
		return
	}

	respondJSON(w, http.StatusOK, toMappingResponse(mapping))
}

func (h *MappingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	mapping, err := h.mappingRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "mapping not found")
		return
	}

	var req CreateMappingRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mapping.Description = req.Description

	// Parse and set account IDs
	var fromDueTo, fromDueFrom, fromRevenue, fromExpense *common.ID
	var toDueTo, toDueFrom, toRevenue, toExpense *common.ID

	if req.FromDueToAccountID != "" {
		id, _ := common.Parse(req.FromDueToAccountID)
		fromDueTo = &id
	}
	if req.FromDueFromAccountID != "" {
		id, _ := common.Parse(req.FromDueFromAccountID)
		fromDueFrom = &id
	}
	if req.FromRevenueAccountID != "" {
		id, _ := common.Parse(req.FromRevenueAccountID)
		fromRevenue = &id
	}
	if req.FromExpenseAccountID != "" {
		id, _ := common.Parse(req.FromExpenseAccountID)
		fromExpense = &id
	}
	if req.ToDueToAccountID != "" {
		id, _ := common.Parse(req.ToDueToAccountID)
		toDueTo = &id
	}
	if req.ToDueFromAccountID != "" {
		id, _ := common.Parse(req.ToDueFromAccountID)
		toDueFrom = &id
	}
	if req.ToRevenueAccountID != "" {
		id, _ := common.Parse(req.ToRevenueAccountID)
		toRevenue = &id
	}
	if req.ToExpenseAccountID != "" {
		id, _ := common.Parse(req.ToExpenseAccountID)
		toExpense = &id
	}

	mapping.SetFromAccounts(fromDueTo, fromDueFrom, fromRevenue, fromExpense)
	mapping.SetToAccounts(toDueTo, toDueFrom, toRevenue, toExpense)

	if err := h.mappingRepo.Update(r.Context(), mapping); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toMappingResponse(mapping))
}

func (h *MappingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.mappingRepo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "mapping deleted"})
}
