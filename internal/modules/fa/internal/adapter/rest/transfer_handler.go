package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/modules/fa/internal/service"
	"converge-finance.com/m/internal/platform/audit"
)

type TransferHandler struct {
	*Handler
	transferRepo repository.TransferRepository
	assetService *service.AssetService
	auditLogger  *audit.Logger
}

func NewTransferHandler(
	h *Handler,
	transferRepo repository.TransferRepository,
	assetService *service.AssetService,
	auditLogger *audit.Logger,
) *TransferHandler {
	return &TransferHandler{
		Handler:      h,
		transferRepo: transferRepo,
		assetService: assetService,
		auditLogger:  auditLogger,
	}
}

type CreateTransferRequest struct {
	AssetID          string `json:"asset_id"`
	TransferDate     string `json:"transfer_date"`
	EffectiveDate    string `json:"effective_date"`
	ToLocationCode   string `json:"to_location_code,omitempty"`
	ToLocationName   string `json:"to_location_name,omitempty"`
	ToDepartmentCode string `json:"to_department_code,omitempty"`
	ToDepartmentName string `json:"to_department_name,omitempty"`
	ToCustodianID    string `json:"to_custodian_id,omitempty"`
	ToCustodianName  string `json:"to_custodian_name,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type TransferResponse struct {
	ID                 string     `json:"id"`
	EntityID           string     `json:"entity_id"`
	TransferNumber     string     `json:"transfer_number"`
	AssetID            string     `json:"asset_id"`
	TransferDate       string     `json:"transfer_date"`
	EffectiveDate      string     `json:"effective_date"`
	FromLocationCode   string     `json:"from_location_code,omitempty"`
	FromLocationName   string     `json:"from_location_name,omitempty"`
	ToLocationCode     string     `json:"to_location_code,omitempty"`
	ToLocationName     string     `json:"to_location_name,omitempty"`
	FromDepartmentCode string     `json:"from_department_code,omitempty"`
	FromDepartmentName string     `json:"from_department_name,omitempty"`
	ToDepartmentCode   string     `json:"to_department_code,omitempty"`
	ToDepartmentName   string     `json:"to_department_name,omitempty"`
	FromCustodianName  string     `json:"from_custodian_name,omitempty"`
	ToCustodianName    string     `json:"to_custodian_name,omitempty"`
	Reason             string     `json:"reason,omitempty"`
	Status             string     `json:"status"`
	ApprovedAt         *time.Time `json:"approved_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

func toTransferResponse(t *domain.AssetTransfer) TransferResponse {
	return TransferResponse{
		ID:                 t.ID.String(),
		EntityID:           t.EntityID.String(),
		TransferNumber:     t.TransferNumber,
		AssetID:            t.AssetID.String(),
		TransferDate:       t.TransferDate.Format("2006-01-02"),
		EffectiveDate:      t.EffectiveDate.Format("2006-01-02"),
		FromLocationCode:   t.FromLocationCode,
		FromLocationName:   t.FromLocationName,
		ToLocationCode:     t.ToLocationCode,
		ToLocationName:     t.ToLocationName,
		FromDepartmentCode: t.FromDepartmentCode,
		FromDepartmentName: t.FromDepartmentName,
		ToDepartmentCode:   t.ToDepartmentCode,
		ToDepartmentName:   t.ToDepartmentName,
		FromCustodianName:  t.FromCustodianName,
		ToCustodianName:    t.ToCustodianName,
		Reason:             t.Reason,
		Status:             string(t.Status),
		ApprovedAt:         t.ApprovedAt,
		CompletedAt:        t.CompletedAt,
		CreatedAt:          t.CreatedAt,
	}
}

func (h *TransferHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var assetID *common.ID
	if aid := r.URL.Query().Get("asset_id"); aid != "" {
		id, _ := common.Parse(aid)
		assetID = &id
	}

	var status *domain.TransferStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.TransferStatus(s)
		status = &st
	}

	filter := domain.AssetTransferFilter{
		EntityID: entityID,
		AssetID:  assetID,
		Status:   status,
		Limit:    getIntQuery(r, "limit", 50),
		Offset:   getIntQuery(r, "offset", 0),
	}

	transfers, err := h.transferRepo.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, err := h.transferRepo.Count(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []TransferResponse
	for _, t := range transfers {
		responses = append(responses, toTransferResponse(&t))
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(responses)) < total,
	})
}

func (h *TransferHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTransferRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	assetID, err := common.Parse(req.AssetID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid asset ID")
		return
	}

	transferDate, err := time.Parse("2006-01-02", req.TransferDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid transfer date")
		return
	}

	effectiveDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid effective date")
		return
	}

	transferReq := service.TransferRequest{
		AssetID:          assetID,
		TransferDate:     transferDate,
		EffectiveDate:    effectiveDate,
		ToLocationCode:   req.ToLocationCode,
		ToLocationName:   req.ToLocationName,
		ToDepartmentCode: req.ToDepartmentCode,
		ToDepartmentName: req.ToDepartmentName,
		ToCustodianName:  req.ToCustodianName,
		Reason:           req.Reason,
	}

	if req.ToCustodianID != "" {
		id, _ := common.Parse(req.ToCustodianID)
		transferReq.ToCustodianID = &id
	}

	transfer, err := h.assetService.CreateTransfer(r.Context(), transferReq)
	if err != nil {
		if ve, ok := err.(*common.ValidationError); ok {
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toTransferResponse(transfer))
}

func (h *TransferHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	transfer, err := h.transferRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "transfer not found")
		return
	}

	respondJSON(w, http.StatusOK, toTransferResponse(transfer))
}

func (h *TransferHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.assetService.ApproveTransfer(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	transfer, _ := h.transferRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransferResponse(transfer))
}

func (h *TransferHandler) Complete(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.assetService.CompleteTransfer(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	transfer, _ := h.transferRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransferResponse(transfer))
}

func (h *TransferHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.assetService.CancelTransfer(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	transfer, _ := h.transferRepo.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransferResponse(transfer))
}
