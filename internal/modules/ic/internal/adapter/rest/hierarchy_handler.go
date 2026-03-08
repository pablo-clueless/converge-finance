package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
	"converge-finance.com/m/internal/modules/ic/internal/repository"
)

type HierarchyHandler struct {
	*Handler
	hierarchyRepo repository.EntityHierarchyRepository
}

func NewHierarchyHandler(h *Handler, hierarchyRepo repository.EntityHierarchyRepository) *HierarchyHandler {
	return &HierarchyHandler{
		Handler:       h,
		hierarchyRepo: hierarchyRepo,
	}
}

type EntityHierarchyResponse struct {
	ID                  string                    `json:"id"`
	Code                string                    `json:"code"`
	Name                string                    `json:"name"`
	BaseCurrency        string                    `json:"base_currency"`
	IsActive            bool                      `json:"is_active"`
	ParentID            string                    `json:"parent_id,omitempty"`
	EntityType          string                    `json:"entity_type"`
	OwnershipPercent    float64                   `json:"ownership_percent"`
	ConsolidationMethod string                    `json:"consolidation_method"`
	HierarchyLevel      int                       `json:"hierarchy_level"`
	HierarchyPath       string                    `json:"hierarchy_path"`
	CreatedAt           time.Time                 `json:"created_at"`
	UpdatedAt           time.Time                 `json:"updated_at"`
	Children            []EntityHierarchyResponse `json:"children,omitempty"`
}

func toHierarchyResponse(e *domain.EntityHierarchy) EntityHierarchyResponse {
	resp := EntityHierarchyResponse{
		ID:                  e.ID.String(),
		Code:                e.Code,
		Name:                e.Name,
		BaseCurrency:        e.BaseCurrency,
		IsActive:            e.IsActive,
		EntityType:          string(e.EntityType),
		OwnershipPercent:    e.OwnershipPercent.InexactFloat64(),
		ConsolidationMethod: string(e.ConsolidationMethod),
		HierarchyLevel:      e.HierarchyLevel,
		HierarchyPath:       e.HierarchyPath,
		CreatedAt:           e.CreatedAt,
		UpdatedAt:           e.UpdatedAt,
	}
	if e.ParentID != nil {
		resp.ParentID = e.ParentID.String()
	}
	for _, child := range e.Children {
		resp.Children = append(resp.Children, toHierarchyResponse(child))
	}
	return resp
}

func (h *HierarchyHandler) GetTree(w http.ResponseWriter, r *http.Request) {
	rootID := getEntityID(r)
	if rootID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	tree, err := h.hierarchyRepo.GetHierarchyTree(r.Context(), rootID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toHierarchyResponse(tree))
}

func (h *HierarchyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	entity, err := h.hierarchyRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "entity not found")
		return
	}

	respondJSON(w, http.StatusOK, toHierarchyResponse(entity))
}

func (h *HierarchyHandler) GetChildren(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	children, err := h.hierarchyRepo.GetChildren(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []EntityHierarchyResponse
	for _, c := range children {
		responses = append(responses, toHierarchyResponse(&c))
	}

	respondJSON(w, http.StatusOK, responses)
}

type UpdateHierarchyRequest struct {
	ParentID            string  `json:"parent_id,omitempty"`
	EntityType          string  `json:"entity_type"`
	OwnershipPercent    float64 `json:"ownership_percent"`
	ConsolidationMethod string  `json:"consolidation_method"`
}

func (h *HierarchyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var req UpdateHierarchyRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var parentID *common.ID
	if req.ParentID != "" {
		pid, err := common.Parse(req.ParentID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid parent_id")
			return
		}
		parentID = &pid
	}

	if err := h.hierarchyRepo.UpdateHierarchy(
		r.Context(),
		id,
		parentID,
		domain.EntityType(req.EntityType),
		req.OwnershipPercent,
		domain.ConsolidationMethod(req.ConsolidationMethod),
	); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	entity, _ := h.hierarchyRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toHierarchyResponse(entity))
}

func (h *HierarchyHandler) GetRoots(w http.ResponseWriter, r *http.Request) {
	roots, err := h.hierarchyRepo.GetRootEntities(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []EntityHierarchyResponse
	for _, root := range roots {
		responses = append(responses, toHierarchyResponse(&root))
	}

	respondJSON(w, http.StatusOK, responses)
}
