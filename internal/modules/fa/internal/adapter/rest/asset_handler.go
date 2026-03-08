package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/modules/fa/internal/service"
	"converge-finance.com/m/internal/platform/audit"
)

type AssetHandler struct {
	*Handler
	assetRepo    repository.AssetRepository
	categoryRepo repository.CategoryRepository
	assetService *service.AssetService
	auditLogger  *audit.Logger
}

func NewAssetHandler(
	h *Handler,
	assetRepo repository.AssetRepository,
	categoryRepo repository.CategoryRepository,
	assetService *service.AssetService,
	auditLogger *audit.Logger,
) *AssetHandler {
	return &AssetHandler{
		Handler:      h,
		assetRepo:    assetRepo,
		categoryRepo: categoryRepo,
		assetService: assetService,
		auditLogger:  auditLogger,
	}
}

type CreateAssetRequest struct {
	CategoryID         string  `json:"category_id"`
	AssetCode          string  `json:"asset_code"`
	AssetName          string  `json:"asset_name"`
	Description        string  `json:"description,omitempty"`
	SerialNumber       string  `json:"serial_number,omitempty"`
	AcquisitionDate    string  `json:"acquisition_date"`
	AcquisitionCost    string  `json:"acquisition_cost"`
	Currency           string  `json:"currency"`
	DepreciationMethod string  `json:"depreciation_method"`
	UsefulLifeYears    int     `json:"useful_life_years"`
	UsefulLifeUnits    *int    `json:"useful_life_units,omitempty"`
	SalvageValue       string  `json:"salvage_value"`
	VendorID           string  `json:"vendor_id,omitempty"`
	PONumber           string  `json:"po_number,omitempty"`
	LocationCode       string  `json:"location_code,omitempty"`
	LocationName       string  `json:"location_name,omitempty"`
	DepartmentCode     string  `json:"department_code,omitempty"`
	DepartmentName     string  `json:"department_name,omitempty"`
	CustodianName      string  `json:"custodian_name,omitempty"`
}

type AssetResponse struct {
	ID                      string     `json:"id"`
	EntityID                string     `json:"entity_id"`
	CategoryID              string     `json:"category_id"`
	CategoryName            string     `json:"category_name,omitempty"`
	AssetCode               string     `json:"asset_code"`
	AssetName               string     `json:"asset_name"`
	Description             string     `json:"description,omitempty"`
	SerialNumber            string     `json:"serial_number,omitempty"`
	AcquisitionDate         string     `json:"acquisition_date"`
	AcquisitionCost         string     `json:"acquisition_cost"`
	Currency                string     `json:"currency"`
	DepreciationMethod      string     `json:"depreciation_method"`
	UsefulLifeYears         int        `json:"useful_life_years"`
	SalvageValue            string     `json:"salvage_value"`
	AccumulatedDepreciation string     `json:"accumulated_depreciation"`
	BookValue               string     `json:"book_value"`
	DepreciationStartDate   string     `json:"depreciation_start_date,omitempty"`
	LastDepreciationDate    string     `json:"last_depreciation_date,omitempty"`
	LocationCode            string     `json:"location_code,omitempty"`
	LocationName            string     `json:"location_name,omitempty"`
	DepartmentCode          string     `json:"department_code,omitempty"`
	DepartmentName          string     `json:"department_name,omitempty"`
	CustodianName           string     `json:"custodian_name,omitempty"`
	Status                  string     `json:"status"`
	ActivatedAt             *time.Time `json:"activated_at,omitempty"`
	DisposedAt              *time.Time `json:"disposed_at,omitempty"`
	IsFullyDepreciated      bool       `json:"is_fully_depreciated"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func toAssetResponse(a *domain.Asset) AssetResponse {
	resp := AssetResponse{
		ID:                      a.ID.String(),
		EntityID:                a.EntityID.String(),
		CategoryID:              a.CategoryID.String(),
		AssetCode:               a.AssetCode,
		AssetName:               a.AssetName,
		Description:             a.Description,
		SerialNumber:            a.SerialNumber,
		AcquisitionDate:         a.AcquisitionDate.Format("2006-01-02"),
		AcquisitionCost:         a.AcquisitionCost.Amount.String(),
		Currency:                a.Currency.Code,
		DepreciationMethod:      string(a.DepreciationMethod),
		UsefulLifeYears:         a.UsefulLifeYears,
		SalvageValue:            a.SalvageValue.Amount.String(),
		AccumulatedDepreciation: a.AccumulatedDepreciation.Amount.String(),
		BookValue:               a.BookValue.Amount.String(),
		LocationCode:            a.LocationCode,
		LocationName:            a.LocationName,
		DepartmentCode:          a.DepartmentCode,
		DepartmentName:          a.DepartmentName,
		CustodianName:           a.CustodianName,
		Status:                  string(a.Status),
		ActivatedAt:             a.ActivatedAt,
		DisposedAt:              a.DisposedAt,
		IsFullyDepreciated:      a.IsFullyDepreciated(),
		CreatedAt:               a.CreatedAt,
		UpdatedAt:               a.UpdatedAt,
	}
	if a.DepreciationStartDate != nil {
		resp.DepreciationStartDate = a.DepreciationStartDate.Format("2006-01-02")
	}
	if a.LastDepreciationDate != nil {
		resp.LastDepreciationDate = a.LastDepreciationDate.Format("2006-01-02")
	}
	if a.Category != nil {
		resp.CategoryName = a.Category.Name
	}
	return resp
}

func (h *AssetHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var categoryID *common.ID
	if cid := r.URL.Query().Get("category_id"); cid != "" {
		id, _ := common.Parse(cid)
		categoryID = &id
	}

	var status *domain.AssetStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.AssetStatus(s)
		status = &st
	}

	filter := domain.AssetFilter{
		EntityID:   entityID,
		CategoryID: categoryID,
		Status:     status,
		Search:     r.URL.Query().Get("search"),
		Limit:      getIntQuery(r, "limit", 50),
		Offset:     getIntQuery(r, "offset", 0),
	}

	assets, err := h.assetRepo.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, err := h.assetRepo.Count(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []AssetResponse
	for _, a := range assets {
		responses = append(responses, toAssetResponse(&a))
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(responses)) < total,
	})
}

func (h *AssetHandler) Create(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)

	var req CreateAssetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	categoryID, err := common.Parse(req.CategoryID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid category ID")
		return
	}

	acquisitionDate, err := time.Parse("2006-01-02", req.AcquisitionDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid acquisition date")
		return
	}

	acquisitionCost, err := money.NewFromString(req.AcquisitionCost, req.Currency)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid acquisition cost")
		return
	}

	salvageValue, err := money.NewFromString(req.SalvageValue, req.Currency)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid salvage value")
		return
	}

	createReq := service.CreateAssetRequest{
		EntityID:           entityID,
		CategoryID:         categoryID,
		AssetCode:          req.AssetCode,
		AssetName:          req.AssetName,
		Description:        req.Description,
		SerialNumber:       req.SerialNumber,
		AcquisitionDate:    acquisitionDate,
		AcquisitionCost:    acquisitionCost,
		DepreciationMethod: domain.DepreciationMethod(req.DepreciationMethod),
		UsefulLifeYears:    req.UsefulLifeYears,
		UsefulLifeUnits:    req.UsefulLifeUnits,
		SalvageValue:       salvageValue,
		PONumber:           req.PONumber,
		LocationCode:       req.LocationCode,
		LocationName:       req.LocationName,
		DepartmentCode:     req.DepartmentCode,
		DepartmentName:     req.DepartmentName,
		CustodianName:      req.CustodianName,
	}

	if req.VendorID != "" {
		vid, _ := common.Parse(req.VendorID)
		createReq.VendorID = &vid
	}

	asset, err := h.assetService.CreateAsset(r.Context(), createReq)
	if err != nil {
		if ve, ok := err.(*common.ValidationError); ok {
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toAssetResponse(asset))
}

func (h *AssetHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	asset, err := h.assetRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "asset not found")
		return
	}

	category, _ := h.categoryRepo.GetByID(r.Context(), asset.CategoryID)
	asset.Category = category

	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

func (h *AssetHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	asset, err := h.assetRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "asset not found")
		return
	}

	if asset.Status != domain.AssetStatusDraft {
		respondError(w, http.StatusBadRequest, "can only update draft assets")
		return
	}

	var req CreateAssetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	asset.AssetName = req.AssetName
	asset.Description = req.Description
	asset.SerialNumber = req.SerialNumber
	asset.LocationCode = req.LocationCode
	asset.LocationName = req.LocationName
	asset.DepartmentCode = req.DepartmentCode
	asset.DepartmentName = req.DepartmentName
	asset.CustodianName = req.CustodianName
	asset.UpdatedAt = time.Now()

	if err := h.assetRepo.Update(r.Context(), asset); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

func (h *AssetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	asset, err := h.assetRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "asset not found")
		return
	}

	if asset.Status != domain.AssetStatusDraft {
		respondError(w, http.StatusBadRequest, "can only delete draft assets")
		return
	}

	if err := h.assetRepo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true})
}

type ActivateRequest struct {
	DepreciationStartDate string `json:"depreciation_start_date"`
}

func (h *AssetHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var req ActivateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	startDate, err := time.Parse("2006-01-02", req.DepreciationStartDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid depreciation start date")
		return
	}

	if err := h.assetService.ActivateAsset(r.Context(), id, startDate); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	asset, _ := h.assetRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

type SuspendRequest struct {
	Reason string `json:"reason"`
}

func (h *AssetHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var req SuspendRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.assetService.SuspendAsset(r.Context(), id, req.Reason); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	asset, _ := h.assetRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

func (h *AssetHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.assetService.ReactivateAsset(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	asset, _ := h.assetRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

type DisposeRequest struct {
	DisposalType string `json:"disposal_type"`
	Proceeds     string `json:"proceeds"`
	Cost         string `json:"cost"`
	Notes        string `json:"notes,omitempty"`
}

func (h *AssetHandler) Dispose(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	asset, err := h.assetRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "asset not found")
		return
	}

	var req DisposeRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	proceeds, err := money.NewFromString(req.Proceeds, asset.Currency.Code)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid proceeds amount")
		return
	}

	cost, err := money.NewFromString(req.Cost, asset.Currency.Code)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid cost amount")
		return
	}

	dispReq := service.DisposalRequest{
		AssetID:      id,
		DisposalType: domain.DisposalType(req.DisposalType),
		Proceeds:     proceeds,
		Cost:         cost,
		Notes:        req.Notes,
	}

	if err := h.assetService.DisposeAsset(r.Context(), dispReq); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	asset, _ = h.assetRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

type WriteOffRequest struct {
	Notes string `json:"notes,omitempty"`
}

func (h *AssetHandler) WriteOff(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var req WriteOffRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.assetService.WriteOffAsset(r.Context(), id, req.Notes); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	asset, _ := h.assetRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

type RecordUnitsRequest struct {
	Units int `json:"units"`
}

func (h *AssetHandler) RecordUnits(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var req RecordUnitsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.assetService.RecordUnits(r.Context(), id, req.Units); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	asset, _ := h.assetRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toAssetResponse(asset))
}

func (h *AssetHandler) GetDepreciationSchedule(w http.ResponseWriter, r *http.Request) {
	respondError(w, http.StatusNotImplemented, "not implemented")
}

func (h *AssetHandler) AssetRegister(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	filter := domain.AssetFilter{
		EntityID: entityID,
		Limit:    getIntQuery(r, "limit", 1000),
		Offset:   getIntQuery(r, "offset", 0),
	}

	assets, err := h.assetRepo.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []AssetResponse
	for _, a := range assets {
		responses = append(responses, toAssetResponse(&a))
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *AssetHandler) BookValueReport(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	status := domain.AssetStatusActive
	filter := domain.AssetFilter{
		EntityID: entityID,
		Status:   &status,
		Limit:    getIntQuery(r, "limit", 1000),
		Offset:   getIntQuery(r, "offset", 0),
	}

	assets, err := h.assetRepo.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []AssetResponse
	for _, a := range assets {
		responses = append(responses, toAssetResponse(&a))
	}

	respondJSON(w, http.StatusOK, responses)
}
