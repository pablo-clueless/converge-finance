package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
	"converge-finance.com/m/internal/modules/consol/internal/service"
	"go.uber.org/zap"
)

type ConsolidationSetHandler struct {
	*Handler
	consolidationService *service.ConsolidationService
}

func NewConsolidationSetHandler(
	logger *zap.Logger,
	consolidationService *service.ConsolidationService,
) *ConsolidationSetHandler {
	return &ConsolidationSetHandler{
		Handler:              NewHandler(logger),
		consolidationService: consolidationService,
	}
}

type CreateConsolidationSetRequest struct {
	SetCode           string `json:"set_code"`
	SetName           string `json:"set_name"`
	Description       string `json:"description,omitempty"`
	ReportingCurrency string `json:"reporting_currency"`
}

type ConsolidationSetResponse struct {
	ID                       common.ID                        `json:"id"`
	SetCode                  string                           `json:"set_code"`
	SetName                  string                           `json:"set_name"`
	Description              string                           `json:"description,omitempty"`
	ParentEntityID           common.ID                        `json:"parent_entity_id"`
	ReportingCurrency        string                           `json:"reporting_currency"`
	DefaultTranslationMethod string                           `json:"default_translation_method"`
	IsActive                 bool                             `json:"is_active"`
	Members                  []ConsolidationSetMemberResponse `json:"members,omitempty"`
	CreatedAt                time.Time                        `json:"created_at"`
	UpdatedAt                time.Time                        `json:"updated_at"`
}

type ConsolidationSetMemberResponse struct {
	ID                  common.ID `json:"id"`
	EntityID            common.ID `json:"entity_id"`
	EntityCode          string    `json:"entity_code,omitempty"`
	EntityName          string    `json:"entity_name,omitempty"`
	OwnershipPercent    float64   `json:"ownership_percent"`
	MinorityPercent     float64   `json:"minority_percent"`
	ConsolidationMethod string    `json:"consolidation_method"`
	TranslationMethod   *string   `json:"translation_method,omitempty"`
	FunctionalCurrency  string    `json:"functional_currency"`
	SequenceNumber      int       `json:"sequence_number"`
	IsActive            bool      `json:"is_active"`
}

func (h *ConsolidationSetHandler) CreateSet(w http.ResponseWriter, r *http.Request) {
	var req CreateConsolidationSetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	currency := money.MustGetCurrency(req.ReportingCurrency)

	set, err := h.consolidationService.CreateConsolidationSet(
		r.Context(),
		req.SetCode,
		req.SetName,
		entityID,
		currency,
	)
	if err != nil {
		h.logger.Error("failed to create consolidation set", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toConsolidationSetResponse(set))
}

func (h *ConsolidationSetHandler) GetSet(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	set, err := h.consolidationService.GetConsolidationSet(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "consolidation set not found")
		return
	}

	respondJSON(w, http.StatusOK, toConsolidationSetResponse(set))
}

func (h *ConsolidationSetHandler) ListSets(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	isActive := getBoolQuery(r, "is_active")
	search := r.URL.Query().Get("search")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.ConsolidationSetFilter{
		ParentEntityID: &entityID,
		IsActive:       isActive,
		Search:         search,
		Limit:          limit,
		Offset:         offset,
	}

	sets, err := h.consolidationService.ListConsolidationSets(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list consolidation sets", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ConsolidationSetResponse, len(sets))
	for i, set := range sets {
		response[i] = toConsolidationSetResponse(&set)
	}

	respondJSON(w, http.StatusOK, response)
}

type AddMemberRequest struct {
	EntityID            common.ID `json:"entity_id"`
	OwnershipPercent    float64   `json:"ownership_percent"`
	ConsolidationMethod string    `json:"consolidation_method"`
	FunctionalCurrency  string    `json:"functional_currency"`
}

func (h *ConsolidationSetHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	setID, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid set ID")
		return
	}

	var req AddMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	consolidationMethod := domain.ConsolidationMethod(req.ConsolidationMethod)
	if !consolidationMethod.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid consolidation method")
		return
	}

	currency := money.MustGetCurrency(req.FunctionalCurrency)

	member, err := h.consolidationService.AddMemberToSet(
		r.Context(),
		setID,
		req.EntityID,
		req.OwnershipPercent,
		consolidationMethod,
		currency,
	)
	if err != nil {
		h.logger.Error("failed to add member", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toMemberResponse(member))
}

func toConsolidationSetResponse(set *domain.ConsolidationSet) ConsolidationSetResponse {
	resp := ConsolidationSetResponse{
		ID:                       set.ID,
		SetCode:                  set.SetCode,
		SetName:                  set.SetName,
		Description:              set.Description,
		ParentEntityID:           set.ParentEntityID,
		ReportingCurrency:        set.ReportingCurrency.Code,
		DefaultTranslationMethod: string(set.DefaultTranslationMethod),
		IsActive:                 set.IsActive,
		CreatedAt:                set.CreatedAt,
		UpdatedAt:                set.UpdatedAt,
	}

	if len(set.Members) > 0 {
		resp.Members = make([]ConsolidationSetMemberResponse, len(set.Members))
		for i, member := range set.Members {
			resp.Members[i] = toMemberResponse(&member)
		}
	}

	return resp
}

func toMemberResponse(member *domain.ConsolidationSetMember) ConsolidationSetMemberResponse {
	resp := ConsolidationSetMemberResponse{
		ID:                  member.ID,
		EntityID:            member.EntityID,
		EntityCode:          member.EntityCode,
		EntityName:          member.EntityName,
		OwnershipPercent:    member.OwnershipPercent,
		MinorityPercent:     member.MinorityPercent(),
		ConsolidationMethod: string(member.ConsolidationMethod),
		FunctionalCurrency:  member.FunctionalCurrency.Code,
		SequenceNumber:      member.SequenceNumber,
		IsActive:            member.IsActive,
	}

	if member.TranslationMethod != nil {
		method := string(*member.TranslationMethod)
		resp.TranslationMethod = &method
	}

	return resp
}
