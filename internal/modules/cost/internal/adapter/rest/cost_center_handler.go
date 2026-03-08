package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
	"converge-finance.com/m/internal/modules/cost/internal/service"
	"go.uber.org/zap"
)

type CostCenterHandler struct {
	*Handler
	costCenterService *service.CostCenterService
}

func NewCostCenterHandler(logger *zap.Logger, svc *service.CostCenterService) *CostCenterHandler {
	return &CostCenterHandler{Handler: NewHandler(logger), costCenterService: svc}
}

type CreateCostCenterRequest struct {
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	CenterType string     `json:"center_type"`
	ParentID   *common.ID `json:"parent_id,omitempty"`
}

type CostCenterResponse struct {
	ID                      common.ID  `json:"id"`
	EntityID                common.ID  `json:"entity_id"`
	Code                    string     `json:"code"`
	Name                    string     `json:"name"`
	Description             string     `json:"description,omitempty"`
	CenterType              string     `json:"center_type"`
	ParentID                *common.ID `json:"parent_id,omitempty"`
	HierarchyLevel          int        `json:"hierarchy_level"`
	ManagerID               *common.ID `json:"manager_id,omitempty"`
	ManagerName             string     `json:"manager_name,omitempty"`
	DefaultExpenseAccountID *common.ID `json:"default_expense_account_id,omitempty"`
	Headcount               int        `json:"headcount"`
	SquareFootage           float64    `json:"square_footage"`
	IsActive                bool       `json:"is_active"`
	CreatedAt               time.Time  `json:"created_at"`
}

func (h *CostCenterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateCostCenterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	centerType := domain.CenterType(req.CenterType)
	if !centerType.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid center type")
		return
	}

	center, err := h.costCenterService.CreateCostCenter(r.Context(), entityID, req.Code, req.Name, centerType, req.ParentID)
	if err != nil {
		h.logger.Error("failed to create cost center", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toCostCenterResponse(center))
}

func (h *CostCenterHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cost center ID")
		return
	}

	center, err := h.costCenterService.GetCostCenter(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "cost center not found")
		return
	}

	respondJSON(w, http.StatusOK, toCostCenterResponse(center))
}

func (h *CostCenterHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	isActive := getBoolQuery(r, "is_active")
	centerTypeStr := getStringQuery(r, "center_type")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.CostCenterFilter{
		EntityID: &entityID,
		IsActive: isActive,
		Limit:    limit,
		Offset:   offset,
	}

	if centerTypeStr != nil {
		ct := domain.CenterType(*centerTypeStr)
		filter.CenterType = &ct
	}

	centers, err := h.costCenterService.ListCostCenters(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list cost centers", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]CostCenterResponse, len(centers))
	for i, center := range centers {
		response[i] = toCostCenterResponse(&center)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *CostCenterHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cost center ID")
		return
	}

	if err := h.costCenterService.DeactivateCostCenter(r.Context(), id); err != nil {
		h.logger.Error("failed to deactivate cost center", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "cost center deactivated"})
}

func toCostCenterResponse(c *domain.CostCenter) CostCenterResponse {
	return CostCenterResponse{
		ID:                      c.ID,
		EntityID:                c.EntityID,
		Code:                    c.Code,
		Name:                    c.Name,
		Description:             c.Description,
		CenterType:              string(c.CenterType),
		ParentID:                c.ParentID,
		HierarchyLevel:          c.HierarchyLevel,
		ManagerID:               c.ManagerID,
		ManagerName:             c.ManagerName,
		DefaultExpenseAccountID: c.DefaultExpenseAccountID,
		Headcount:               c.Headcount,
		SquareFootage:           c.SquareFootage,
		IsActive:                c.IsActive,
		CreatedAt:               c.CreatedAt,
	}
}
