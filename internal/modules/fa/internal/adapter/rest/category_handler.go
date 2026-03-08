package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/shopspring/decimal"
)

type CategoryHandler struct {
	*Handler
	categoryRepo repository.CategoryRepository
	auditLogger  *audit.Logger
}

func NewCategoryHandler(
	h *Handler,
	categoryRepo repository.CategoryRepository,
	auditLogger *audit.Logger,
) *CategoryHandler {
	return &CategoryHandler{
		Handler:      h,
		categoryRepo: categoryRepo,
		auditLogger:  auditLogger,
	}
}

type CreateCategoryRequest struct {
	Code                        string  `json:"code"`
	Name                        string  `json:"name"`
	Description                 string  `json:"description,omitempty"`
	DepreciationMethod          string  `json:"depreciation_method"`
	DefaultUsefulLifeYears      int     `json:"default_useful_life_years"`
	DefaultSalvagePercent       float64 `json:"default_salvage_percent"`
	AssetAccountID              string  `json:"asset_account_id,omitempty"`
	AccumDepreciationAccountID  string  `json:"accum_depreciation_account_id,omitempty"`
	DepreciationExpenseAccountID string `json:"depreciation_expense_account_id,omitempty"`
	GainLossAccountID           string  `json:"gain_loss_account_id,omitempty"`
}

type CategoryResponse struct {
	ID                          string    `json:"id"`
	EntityID                    string    `json:"entity_id"`
	Code                        string    `json:"code"`
	Name                        string    `json:"name"`
	Description                 string    `json:"description,omitempty"`
	DepreciationMethod          string    `json:"depreciation_method"`
	DefaultUsefulLifeYears      int       `json:"default_useful_life_years"`
	DefaultSalvagePercent       float64   `json:"default_salvage_percent"`
	AssetAccountID              string    `json:"asset_account_id,omitempty"`
	AccumDepreciationAccountID  string    `json:"accum_depreciation_account_id,omitempty"`
	DepreciationExpenseAccountID string   `json:"depreciation_expense_account_id,omitempty"`
	GainLossAccountID           string    `json:"gain_loss_account_id,omitempty"`
	IsActive                    bool      `json:"is_active"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

func toCategoryResponse(c *domain.AssetCategory) CategoryResponse {
	resp := CategoryResponse{
		ID:                     c.ID.String(),
		EntityID:               c.EntityID.String(),
		Code:                   c.Code,
		Name:                   c.Name,
		Description:            c.Description,
		DepreciationMethod:     string(c.DepreciationMethod),
		DefaultUsefulLifeYears: c.DefaultUsefulLifeYears,
		DefaultSalvagePercent:  c.DefaultSalvagePercent.InexactFloat64(),
		IsActive:               c.IsActive,
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
	}
	if c.AssetAccountID != nil {
		resp.AssetAccountID = c.AssetAccountID.String()
	}
	if c.AccumDepreciationAccountID != nil {
		resp.AccumDepreciationAccountID = c.AccumDepreciationAccountID.String()
	}
	if c.DepreciationExpenseAccountID != nil {
		resp.DepreciationExpenseAccountID = c.DepreciationExpenseAccountID.String()
	}
	if c.GainLossAccountID != nil {
		resp.GainLossAccountID = c.GainLossAccountID.String()
	}
	return resp
}

func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	filter := domain.AssetCategoryFilter{
		EntityID: entityID,
		IsActive: getBoolQuery(r, "is_active"),
		Search:   r.URL.Query().Get("search"),
		Limit:    getIntQuery(r, "limit", 50),
		Offset:   getIntQuery(r, "offset", 0),
	}

	categories, err := h.categoryRepo.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, err := h.categoryRepo.Count(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []CategoryResponse
	for _, c := range categories {
		responses = append(responses, toCategoryResponse(&c))
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(responses)) < total,
	})
}

func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	userID := getUserID(r)

	var req CreateCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	category, err := domain.NewAssetCategory(entityID, req.Code, req.Name, userID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	category.Description = req.Description
	category.DepreciationMethod = domain.DepreciationMethod(req.DepreciationMethod)
	category.DefaultUsefulLifeYears = req.DefaultUsefulLifeYears
	category.DefaultSalvagePercent = decimal.NewFromFloat(req.DefaultSalvagePercent)

	if req.AssetAccountID != "" {
		id, _ := common.Parse(req.AssetAccountID)
		category.AssetAccountID = &id
	}
	if req.AccumDepreciationAccountID != "" {
		id, _ := common.Parse(req.AccumDepreciationAccountID)
		category.AccumDepreciationAccountID = &id
	}
	if req.DepreciationExpenseAccountID != "" {
		id, _ := common.Parse(req.DepreciationExpenseAccountID)
		category.DepreciationExpenseAccountID = &id
	}
	if req.GainLossAccountID != "" {
		id, _ := common.Parse(req.GainLossAccountID)
		category.GainLossAccountID = &id
	}

	if err := category.Validate(); err != nil {
		if ve, ok := err.(*common.ValidationError); ok {
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.categoryRepo.Create(r.Context(), category); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toCategoryResponse(category))
}

func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	category, err := h.categoryRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "category not found")
		return
	}

	respondJSON(w, http.StatusOK, toCategoryResponse(category))
}

func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	category, err := h.categoryRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "category not found")
		return
	}

	var req CreateCategoryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	category.Name = req.Name
	category.Description = req.Description
	category.DepreciationMethod = domain.DepreciationMethod(req.DepreciationMethod)
	category.DefaultUsefulLifeYears = req.DefaultUsefulLifeYears
	category.DefaultSalvagePercent = decimal.NewFromFloat(req.DefaultSalvagePercent)
	category.UpdatedAt = time.Now()

	if req.AssetAccountID != "" {
		pid, _ := common.Parse(req.AssetAccountID)
		category.AssetAccountID = &pid
	}
	if req.AccumDepreciationAccountID != "" {
		pid, _ := common.Parse(req.AccumDepreciationAccountID)
		category.AccumDepreciationAccountID = &pid
	}
	if req.DepreciationExpenseAccountID != "" {
		pid, _ := common.Parse(req.DepreciationExpenseAccountID)
		category.DepreciationExpenseAccountID = &pid
	}
	if req.GainLossAccountID != "" {
		pid, _ := common.Parse(req.GainLossAccountID)
		category.GainLossAccountID = &pid
	}

	if err := h.categoryRepo.Update(r.Context(), category); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toCategoryResponse(category))
}

func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.categoryRepo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *CategoryHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	category, err := h.categoryRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "category not found")
		return
	}

	category.Activate()

	if err := h.categoryRepo.Update(r.Context(), category); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toCategoryResponse(category))
}

func (h *CategoryHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	category, err := h.categoryRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "category not found")
		return
	}

	category.Deactivate()

	if err := h.categoryRepo.Update(r.Context(), category); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toCategoryResponse(category))
}
