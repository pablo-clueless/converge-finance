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

type DepreciationHandler struct {
	*Handler
	depRepo   repository.DepreciationRepository
	depEngine *service.DepreciationEngine
	auditLogger *audit.Logger
}

func NewDepreciationHandler(
	h *Handler,
	depRepo repository.DepreciationRepository,
	depEngine *service.DepreciationEngine,
	auditLogger *audit.Logger,
) *DepreciationHandler {
	return &DepreciationHandler{
		Handler:     h,
		depRepo:     depRepo,
		depEngine:   depEngine,
		auditLogger: auditLogger,
	}
}

type DepreciationPreviewRequest struct {
	PeriodEndDate string `json:"period_end_date"`
}

type DepreciationPreviewResponse struct {
	AssetID            string `json:"asset_id"`
	AssetCode          string `json:"asset_code"`
	AssetName          string `json:"asset_name"`
	OpeningBookValue   string `json:"opening_book_value"`
	DepreciationAmount string `json:"depreciation_amount"`
	ClosingBookValue   string `json:"closing_book_value"`
	Method             string `json:"method"`
	CalculationBasis   string `json:"calculation_basis"`
}

type RunDepreciationRequest struct {
	FiscalPeriodID string `json:"fiscal_period_id"`
	PeriodEndDate  string `json:"period_end_date"`
	Currency       string `json:"currency"`
}

type DepreciationRunResponse struct {
	ID                string     `json:"id"`
	EntityID          string     `json:"entity_id"`
	RunNumber         string     `json:"run_number"`
	FiscalPeriodID    string     `json:"fiscal_period_id"`
	DepreciationDate  string     `json:"depreciation_date"`
	AssetCount        int        `json:"asset_count"`
	TotalDepreciation string     `json:"total_depreciation"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	JournalEntryID    string     `json:"journal_entry_id,omitempty"`
	PostedAt          *time.Time `json:"posted_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func toDepreciationRunResponse(r *domain.DepreciationRun) DepreciationRunResponse {
	resp := DepreciationRunResponse{
		ID:                r.ID.String(),
		EntityID:          r.EntityID.String(),
		RunNumber:         r.RunNumber,
		FiscalPeriodID:    r.FiscalPeriodID.String(),
		DepreciationDate:  r.DepreciationDate.Format("2006-01-02"),
		AssetCount:        r.AssetCount,
		TotalDepreciation: r.TotalDepreciation.Amount.String(),
		Currency:          r.Currency.Code,
		Status:            string(r.Status),
		PostedAt:          r.PostedAt,
		CreatedAt:         r.CreatedAt,
	}
	if r.JournalEntryID != nil {
		resp.JournalEntryID = r.JournalEntryID.String()
	}
	return resp
}

func (h *DepreciationHandler) Preview(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var req DepreciationPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	periodEndDate, err := time.Parse("2006-01-02", req.PeriodEndDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid period end date")
		return
	}

	previews, err := h.depEngine.PreviewDepreciation(r.Context(), entityID, periodEndDate)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []DepreciationPreviewResponse
	for _, p := range previews {
		responses = append(responses, DepreciationPreviewResponse{
			AssetID:            p.Asset.ID.String(),
			AssetCode:          p.Asset.AssetCode,
			AssetName:          p.Asset.AssetName,
			OpeningBookValue:   p.OpeningBookValue.Amount.String(),
			DepreciationAmount: p.DepreciationAmount.Amount.String(),
			ClosingBookValue:   p.ClosingBookValue.Amount.String(),
			Method:             string(p.Method),
			CalculationBasis:   p.CalculationBasis,
		})
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *DepreciationHandler) Run(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var req RunDepreciationRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal period ID")
		return
	}

	periodEndDate, err := time.Parse("2006-01-02", req.PeriodEndDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid period end date")
		return
	}

	currency := money.MustGetCurrency(req.Currency)

	run, err := h.depEngine.RunMonthlyDepreciation(r.Context(), entityID, fiscalPeriodID, periodEndDate, currency)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toDepreciationRunResponse(run))
}

func (h *DepreciationHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var status *domain.DepreciationRunStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.DepreciationRunStatus(s)
		status = &st
	}

	filter := domain.DepreciationRunFilter{
		EntityID: entityID,
		Status:   status,
		Limit:    getIntQuery(r, "limit", 50),
		Offset:   getIntQuery(r, "offset", 0),
	}

	runs, err := h.depRepo.ListRuns(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	total, err := h.depRepo.CountRuns(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []DepreciationRunResponse
	for _, run := range runs {
		responses = append(responses, toDepreciationRunResponse(&run))
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(responses)) < total,
	})
}

func (h *DepreciationHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	run, err := h.depRepo.GetRunByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "depreciation run not found")
		return
	}

	entries, err := h.depRepo.GetEntriesByRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	run.Entries = entries

	type DepreciationEntryResponse struct {
		ID                 string `json:"id"`
		AssetID            string `json:"asset_id"`
		OpeningBookValue   string `json:"opening_book_value"`
		DepreciationAmount string `json:"depreciation_amount"`
		ClosingBookValue   string `json:"closing_book_value"`
		Method             string `json:"method"`
		CalculationBasis   string `json:"calculation_basis"`
	}

	type RunDetailResponse struct {
		DepreciationRunResponse
		Entries []DepreciationEntryResponse `json:"entries"`
	}

	var entryResponses []DepreciationEntryResponse
	for _, e := range entries {
		entryResponses = append(entryResponses, DepreciationEntryResponse{
			ID:                 e.ID.String(),
			AssetID:            e.AssetID.String(),
			OpeningBookValue:   e.OpeningBookValue.Amount.String(),
			DepreciationAmount: e.DepreciationAmount.Amount.String(),
			ClosingBookValue:   e.ClosingBookValue.Amount.String(),
			Method:             string(e.DepreciationMethod),
			CalculationBasis:   e.CalculationBasis,
		})
	}

	resp := RunDetailResponse{
		DepreciationRunResponse: toDepreciationRunResponse(run),
		Entries:                 entryResponses,
	}

	respondJSON(w, http.StatusOK, resp)
}

func (h *DepreciationHandler) PostRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.depEngine.PostDepreciationRun(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	run, _ := h.depRepo.GetRunByID(r.Context(), id)
	respondJSON(w, http.StatusOK, toDepreciationRunResponse(run))
}

func (h *DepreciationHandler) ReverseRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	run, err := h.depEngine.ReverseDepreciationRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toDepreciationRunResponse(run))
}
