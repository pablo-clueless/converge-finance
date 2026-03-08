package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type PeriodHandler struct {
	*Handler
	periodRepo  repository.PeriodRepository
	auditLogger *audit.Logger
}

func NewPeriodHandler(
	logger *zap.Logger,
	periodRepo repository.PeriodRepository,
	auditLogger *audit.Logger,
) *PeriodHandler {
	return &PeriodHandler{
		Handler:     NewHandler(logger),
		periodRepo:  periodRepo,
		auditLogger: auditLogger,
	}
}

func (h *PeriodHandler) RegisterRoutes(r chi.Router) {
	r.Route("/fiscal-years", func(r chi.Router) {
		r.Get("/", h.ListFiscalYears)
		r.Post("/", h.CreateFiscalYear)
		r.Get("/current", h.GetCurrentFiscalYear)
		r.Get("/{id}", h.GetFiscalYear)
		r.Post("/{id}/close", h.CloseFiscalYear)
	})

	r.Route("/periods", func(r chi.Router) {
		r.Get("/", h.ListPeriods)
		r.Get("/open", h.ListOpenPeriods)
		r.Get("/for-date", h.GetPeriodForDate)
		r.Get("/{id}", h.GetPeriod)
		r.Post("/{id}/open", h.OpenPeriod)
		r.Post("/{id}/close", h.ClosePeriod)
		r.Post("/{id}/reopen", h.ReopenPeriod)
	})
}

type CreateFiscalYearRequest struct {
	YearCode  string `json:"year_code"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

type FiscalYearResponse struct {
	ID        string           `json:"id"`
	EntityID  string           `json:"entity_id"`
	YearCode  string           `json:"year_code"`
	StartDate string           `json:"start_date"`
	EndDate   string           `json:"end_date"`
	Status    string           `json:"status"`
	Periods   []PeriodResponse `json:"periods,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type PeriodResponse struct {
	ID           string    `json:"id"`
	EntityID     string    `json:"entity_id"`
	FiscalYearID string    `json:"fiscal_year_id"`
	PeriodNumber int       `json:"period_number"`
	PeriodName   string    `json:"period_name"`
	StartDate    string    `json:"start_date"`
	EndDate      string    `json:"end_date"`
	Status       string    `json:"status"`
	IsAdjustment bool      `json:"is_adjustment"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toFiscalYearResponse(fy *domain.FiscalYear) FiscalYearResponse {
	resp := FiscalYearResponse{
		ID:        fy.ID.String(),
		EntityID:  fy.EntityID.String(),
		YearCode:  fy.YearCode,
		StartDate: fy.StartDate.Format("2006-01-02"),
		EndDate:   fy.EndDate.Format("2006-01-02"),
		Status:    string(fy.Status),
		CreatedAt: fy.CreatedAt,
		UpdatedAt: fy.UpdatedAt,
	}

	if len(fy.Periods) > 0 {
		resp.Periods = make([]PeriodResponse, len(fy.Periods))
		for i, p := range fy.Periods {
			resp.Periods[i] = toPeriodResponse(&p)
		}
	}

	return resp
}

func toPeriodResponse(p *domain.FiscalPeriod) PeriodResponse {
	return PeriodResponse{
		ID:           p.ID.String(),
		EntityID:     p.EntityID.String(),
		FiscalYearID: p.FiscalYearID.String(),
		PeriodNumber: p.PeriodNumber,
		PeriodName:   p.PeriodName,
		StartDate:    p.StartDate.Format("2006-01-02"),
		EndDate:      p.EndDate.Format("2006-01-02"),
		Status:       string(p.Status),
		IsAdjustment: p.IsAdjustment,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func (h *PeriodHandler) ListFiscalYears(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	fiscalYears, err := h.periodRepo.ListYears(ctx, entityID)
	if err != nil {
		h.logger.Error("Failed to list fiscal years", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to list fiscal years")
		return
	}

	responses := make([]FiscalYearResponse, len(fiscalYears))
	for i, fy := range fiscalYears {
		responses[i] = toFiscalYearResponse(&fy)
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *PeriodHandler) GetFiscalYear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid fiscal year ID")
		return
	}

	fiscalYear, err := h.periodRepo.GetYearByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Fiscal year not found")
			return
		}
		h.logger.Error("Failed to get fiscal year", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get fiscal year")
		return
	}

	periods, err := h.periodRepo.GetPeriodsForYear(ctx, fiscalYear.ID)
	if err != nil {
		h.logger.Error("Failed to get periods for fiscal year", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get fiscal year")
		return
	}
	fiscalYear.Periods = periods

	respondJSON(w, http.StatusOK, toFiscalYearResponse(fiscalYear))
}

func (h *PeriodHandler) GetCurrentFiscalYear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	fiscalYear, err := h.periodRepo.GetCurrentYear(ctx, entityID)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "No current fiscal year found")
			return
		}
		h.logger.Error("Failed to get current fiscal year", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get current fiscal year")
		return
	}

	periods, err := h.periodRepo.GetPeriodsForYear(ctx, fiscalYear.ID)
	if err != nil {
		h.logger.Error("Failed to get periods for fiscal year", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get current fiscal year")
		return
	}
	fiscalYear.Periods = periods

	respondJSON(w, http.StatusOK, toFiscalYearResponse(fiscalYear))
}

func (h *PeriodHandler) CreateFiscalYear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	var req CreateFiscalYearRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid start date format (use YYYY-MM-DD)")
		return
	}

	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid end date format (use YYYY-MM-DD)")
		return
	}

	existing, _ := h.periodRepo.GetYearByCode(ctx, entityID, req.YearCode)
	if existing != nil {
		respondError(w, http.StatusConflict, "Fiscal year with this code already exists")
		return
	}

	fiscalYear, err := domain.NewFiscalYear(entityID, req.YearCode, startDate, endDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := fiscalYear.GenerateMonthlyPeriods(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.periodRepo.CreateYear(ctx, fiscalYear); err != nil {
		h.logger.Error("Failed to create fiscal year", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to create fiscal year")
		return
	}

	for i := range fiscalYear.Periods {
		if err := h.periodRepo.CreatePeriod(ctx, &fiscalYear.Periods[i]); err != nil {
			h.logger.Error("Failed to create period", zap.Error(err))
			respondError(w, http.StatusInternalServerError, "Failed to create fiscal year periods")
			return
		}
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogCreate(ctx, "gl.fiscal_year", fiscalYear.ID, map[string]any{
			"year_code":  fiscalYear.YearCode,
			"start_date": fiscalYear.StartDate,
			"end_date":   fiscalYear.EndDate,
		})
	}

	respondJSON(w, http.StatusCreated, toFiscalYearResponse(fiscalYear))
}

func (h *PeriodHandler) CloseFiscalYear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid fiscal year ID")
		return
	}

	fiscalYear, err := h.periodRepo.GetYearByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Fiscal year not found")
			return
		}
		h.logger.Error("Failed to get fiscal year", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to close fiscal year")
		return
	}

	periods, err := h.periodRepo.GetPeriodsForYear(ctx, fiscalYear.ID)
	if err != nil {
		h.logger.Error("Failed to get periods", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to close fiscal year")
		return
	}
	fiscalYear.Periods = periods

	if err := fiscalYear.Close(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.periodRepo.UpdateYear(ctx, fiscalYear); err != nil {
		h.logger.Error("Failed to close fiscal year", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to close fiscal year")
		return
	}

	if h.auditLogger != nil {
		h.auditLogger.LogAction(ctx, "gl.fiscal_year", fiscalYear.ID, "closed", nil)
	}

	respondJSON(w, http.StatusOK, toFiscalYearResponse(fiscalYear))
}

func (h *PeriodHandler) ListPeriods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	fiscalYearIDStr := r.URL.Query().Get("fiscal_year_id")
	if fiscalYearIDStr == "" {
		respondError(w, http.StatusBadRequest, "fiscal_year_id query parameter is required")
		return
	}

	fiscalYearID, err := common.Parse(fiscalYearIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid fiscal_year_id")
		return
	}

	periods, err := h.periodRepo.GetPeriodsForYear(ctx, fiscalYearID)
	if err != nil {
		h.logger.Error("Failed to list periods", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to list periods")
		return
	}

	responses := make([]PeriodResponse, len(periods))
	for i, p := range periods {
		responses[i] = toPeriodResponse(&p)
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *PeriodHandler) ListOpenPeriods(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	periods, err := h.periodRepo.GetOpenPeriods(ctx, entityID)
	if err != nil {
		h.logger.Error("Failed to list open periods", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to list open periods")
		return
	}

	responses := make([]PeriodResponse, len(periods))
	for i, p := range periods {
		responses[i] = toPeriodResponse(&p)
	}

	respondJSON(w, http.StatusOK, responses)
}

func (h *PeriodHandler) GetPeriodForDate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		respondError(w, http.StatusBadRequest, "date query parameter is required")
		return
	}

	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid date format (use YYYY-MM-DD)")
		return
	}

	period, err := h.periodRepo.GetPeriodForDate(ctx, entityID, dateStr)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "No period found for the specified date")
			return
		}
		h.logger.Error("Failed to get period for date", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get period for date")
		return
	}

	respondJSON(w, http.StatusOK, toPeriodResponse(period))
}

func (h *PeriodHandler) GetPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid period ID")
		return
	}

	period, err := h.periodRepo.GetPeriodByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Period not found")
			return
		}
		h.logger.Error("Failed to get period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get period")
		return
	}

	respondJSON(w, http.StatusOK, toPeriodResponse(period))
}

func (h *PeriodHandler) OpenPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid period ID")
		return
	}

	period, err := h.periodRepo.GetPeriodByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Period not found")
			return
		}
		h.logger.Error("Failed to get period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to open period")
		return
	}

	if err := period.Open(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.periodRepo.UpdatePeriod(ctx, period); err != nil {
		h.logger.Error("Failed to open period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to open period")
		return
	}

	if h.auditLogger != nil {
		h.auditLogger.LogAction(ctx, "gl.period", period.ID, "opened", nil)
	}

	respondJSON(w, http.StatusOK, toPeriodResponse(period))
}

func (h *PeriodHandler) ClosePeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid period ID")
		return
	}

	period, err := h.periodRepo.GetPeriodByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Period not found")
			return
		}
		h.logger.Error("Failed to get period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to close period")
		return
	}

	if err := period.Close(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.periodRepo.UpdatePeriod(ctx, period); err != nil {
		h.logger.Error("Failed to close period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to close period")
		return
	}

	if h.auditLogger != nil {
		h.auditLogger.LogAction(ctx, "gl.period", period.ID, "closed", nil)
	}

	respondJSON(w, http.StatusOK, toPeriodResponse(period))
}

func (h *PeriodHandler) ReopenPeriod(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid period ID")
		return
	}

	period, err := h.periodRepo.GetPeriodByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Period not found")
			return
		}
		h.logger.Error("Failed to get period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to reopen period")
		return
	}

	if err := period.Reopen(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.periodRepo.UpdatePeriod(ctx, period); err != nil {
		h.logger.Error("Failed to reopen period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to reopen period")
		return
	}

	if h.auditLogger != nil {
		h.auditLogger.LogAction(ctx, "gl.period", period.ID, "reopened", nil)
	}

	respondJSON(w, http.StatusOK, toPeriodResponse(period))
}
