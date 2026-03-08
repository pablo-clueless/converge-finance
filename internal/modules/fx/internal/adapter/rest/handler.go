package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fx/internal/domain"
	"converge-finance.com/m/internal/modules/fx/internal/repository"
	"converge-finance.com/m/internal/modules/fx/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type TriangulationHandler struct {
	service *service.TriangulationService
	logger  *zap.Logger
}

func NewTriangulationHandler(svc *service.TriangulationService, logger *zap.Logger) *TriangulationHandler {
	return &TriangulationHandler{
		service: svc,
		logger:  logger,
	}
}

type ConvertRequest struct {
	FromCurrency  string  `json:"from_currency"`
	ToCurrency    string  `json:"to_currency"`
	Amount        float64 `json:"amount"`
	Date          string  `json:"date,omitempty"`
	RateType      string  `json:"rate_type,omitempty"`
	ReferenceType string  `json:"reference_type,omitempty"`
	ReferenceID   string  `json:"reference_id,omitempty"`
}

type ConvertResponse struct {
	FromCurrency   string                     `json:"from_currency"`
	ToCurrency     string                     `json:"to_currency"`
	OriginalAmount float64                    `json:"original_amount"`
	ResultAmount   float64                    `json:"result_amount"`
	EffectiveRate  float64                    `json:"effective_rate"`
	Legs           []domain.TriangulationLeg  `json:"legs"`
	LegsCount      int                        `json:"legs_count"`
	Method         domain.TriangulationMethod `json:"method"`
	ConversionDate string                     `json:"conversion_date"`
	RateType       string                     `json:"rate_type"`
}

func (h *TriangulationHandler) Convert(w http.ResponseWriter, r *http.Request) {
	var req ConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	fromCurrency, err := money.GetCurrency(req.FromCurrency)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid from_currency")
		return
	}

	toCurrency, err := money.GetCurrency(req.ToCurrency)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid to_currency")
		return
	}

	date := time.Now()
	if req.Date != "" {
		date, err = time.Parse("2006-01-02", req.Date)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid date format (use YYYY-MM-DD)")
			return
		}
	}

	rateType := money.RateTypeSpot
	if req.RateType != "" {
		rateType = money.RateType(req.RateType)
	}

	amount := money.New(req.Amount, fromCurrency.Code)

	var refID common.ID
	if req.ReferenceID != "" {
		refID = common.ID(req.ReferenceID)
	}

	result, err := h.service.Convert(r.Context(), service.ConvertRequest{
		EntityID:      entityID,
		Amount:        amount,
		ToCurrency:    toCurrency,
		Date:          date,
		RateType:      rateType,
		ReferenceType: req.ReferenceType,
		ReferenceID:   refID,
		CreatedBy:     userID,
	})
	if err != nil {
		h.logger.Error("conversion failed", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := ConvertResponse{
		FromCurrency:   result.FromCurrency.Code,
		ToCurrency:     result.ToCurrency.Code,
		OriginalAmount: result.OriginalAmount.Amount.InexactFloat64(),
		ResultAmount:   result.ResultAmount.Amount.InexactFloat64(),
		EffectiveRate:  result.EffectiveRate.InexactFloat64(),
		Legs:           result.Legs,
		LegsCount:      len(result.Legs),
		Method:         result.Method,
		ConversionDate: result.ConversionDate.Format("2006-01-02"),
		RateType:       string(result.RateType),
	}

	h.writeJSON(w, http.StatusOK, resp)
}

func (h *TriangulationHandler) GetPath(w http.ResponseWriter, r *http.Request) {
	fromCode := r.URL.Query().Get("from")
	toCode := r.URL.Query().Get("to")
	dateStr := r.URL.Query().Get("date")
	rateTypeStr := r.URL.Query().Get("rate_type")

	if fromCode == "" || toCode == "" {
		h.writeError(w, http.StatusBadRequest, "from and to currencies are required")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	from, err := money.GetCurrency(fromCode)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid from currency")
		return
	}

	to, err := money.GetCurrency(toCode)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid to currency")
		return
	}

	date := time.Now()
	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid date format")
			return
		}
	}

	rateType := money.RateTypeSpot
	if rateTypeStr != "" {
		rateType = money.RateType(rateTypeStr)
	}

	path, err := h.service.FindConversionPath(r.Context(), entityID, from, to, date, rateType)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, path)
}

func (h *TriangulationHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	config, err := h.service.GetConfig(r.Context(), entityID)
	if err == domain.ErrConfigNotFound {
		h.writeJSON(w, http.StatusOK, map[string]any{
			"exists": false,
			"defaults": map[string]any{
				"fallback_currencies": []string{"USD", "EUR"},
				"max_legs":            3,
				"allow_inverse_rates": true,
			},
		})
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"exists": true,
		"config": config,
	})
}

type ConfigRequest struct {
	BaseCurrency       string   `json:"base_currency"`
	FallbackCurrencies []string `json:"fallback_currencies"`
	MaxLegs            int      `json:"max_legs"`
	AllowInverseRates  bool     `json:"allow_inverse_rates"`
	RateTolerance      float64  `json:"rate_tolerance"`
}

func (h *TriangulationHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	baseCurrency, err := money.GetCurrency(req.BaseCurrency)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid base_currency")
		return
	}

	config := domain.NewTriangulationConfig(entityID, baseCurrency, userID)
	config.FallbackCurrencies = req.FallbackCurrencies
	if req.MaxLegs >= 2 && req.MaxLegs <= 5 {
		config.MaxLegs = req.MaxLegs
	}
	config.AllowInverseRates = req.AllowInverseRates
	config.RateTolerance = decimal.NewFromFloat(req.RateTolerance)

	if err := h.service.CreateOrUpdateConfig(r.Context(), config); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, config)
}

type PairConfigRequest struct {
	FromCurrency    string  `json:"from_currency"`
	ToCurrency      string  `json:"to_currency"`
	PreferredMethod string  `json:"preferred_method"`
	ViaCurrency     string  `json:"via_currency,omitempty"`
	SpreadMarkup    float64 `json:"spread_markup"`
	Priority        int     `json:"priority"`
}

func (h *TriangulationHandler) CreatePairConfig(w http.ResponseWriter, r *http.Request) {
	var req PairConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	fromCurrency, err := money.GetCurrency(req.FromCurrency)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid from_currency")
		return
	}

	toCurrency, err := money.GetCurrency(req.ToCurrency)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid to_currency")
		return
	}

	config, err := domain.NewCurrencyPairConfig(entityID, fromCurrency, toCurrency, domain.TriangulationMethod(req.PreferredMethod), userID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.ViaCurrency != "" {
		viaCurrency, err := money.GetCurrency(req.ViaCurrency)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid via_currency")
			return
		}
		if err := config.SetViaCurrency(viaCurrency); err != nil {
			h.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := config.SetSpreadMarkup(decimal.NewFromFloat(req.SpreadMarkup)); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config.Priority = req.Priority

	if err := h.service.SetCurrencyPairConfig(r.Context(), config); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, config)
}

func (h *TriangulationHandler) ListPairConfigs(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	configs, err := h.service.ListCurrencyPairConfigs(r.Context(), entityID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, configs)
}

func (h *TriangulationHandler) GetConversionLog(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	log, err := h.service.GetConversionLog(r.Context(), id)
	if err == domain.ErrLogNotFound {
		h.writeError(w, http.StatusNotFound, "log not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, log)
}

func (h *TriangulationHandler) ListConversionLogs(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.TriangulationLogFilter{
		EntityID:     entityID,
		FromCurrency: r.URL.Query().Get("from_currency"),
		ToCurrency:   r.URL.Query().Get("to_currency"),
		Limit:        50,
		Offset:       0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	if dateFromStr := r.URL.Query().Get("date_from"); dateFromStr != "" {
		if dateFrom, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filter.DateFrom = &dateFrom
		}
	}

	if dateToStr := r.URL.Query().Get("date_to"); dateToStr != "" {
		if dateTo, err := time.Parse("2006-01-02", dateToStr); err == nil {
			filter.DateTo = &dateTo
		}
	}

	logs, total, err := h.service.ListConversionLogs(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   logs,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *TriangulationHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *TriangulationHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

type RevaluationHandler struct {
	service *service.RevaluationService
	logger  *zap.Logger
}

func NewRevaluationHandler(svc *service.RevaluationService, logger *zap.Logger) *RevaluationHandler {
	return &RevaluationHandler{
		service: svc,
		logger:  logger,
	}
}

type AccountFXConfigRequest struct {
	AccountID                string `json:"account_id"`
	FXTreatment              string `json:"fx_treatment"`
	RevaluationGainAccountID string `json:"revaluation_gain_account_id,omitempty"`
	RevaluationLossAccountID string `json:"revaluation_loss_account_id,omitempty"`
}

func (h *RevaluationHandler) ConfigureAccountFX(w http.ResponseWriter, r *http.Request) {
	var req AccountFXConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	config, err := h.service.ConfigureAccountFX(r.Context(), service.ConfigureAccountFXRequest{
		EntityID:                 entityID,
		AccountID:                common.ID(req.AccountID),
		FXTreatment:              domain.AccountFXTreatment(req.FXTreatment),
		RevaluationGainAccountID: common.ID(req.RevaluationGainAccountID),
		RevaluationLossAccountID: common.ID(req.RevaluationLossAccountID),
	})
	if err != nil {
		h.logger.Error("failed to configure account FX", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, config)
}

func (h *RevaluationHandler) ListAccountFXConfigs(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	treatmentStr := r.URL.Query().Get("treatment")
	var treatment *domain.AccountFXTreatment
	if treatmentStr != "" {
		t := domain.AccountFXTreatment(treatmentStr)
		treatment = &t
	}

	configs, err := h.service.ListAccountFXConfigs(r.Context(), entityID, treatment)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, configs)
}

type CreateRevaluationRunRequest struct {
	FiscalPeriodID     string `json:"fiscal_period_id"`
	RevaluationDate    string `json:"revaluation_date"`
	RateDate           string `json:"rate_date"`
	FunctionalCurrency string `json:"functional_currency"`
}

func (h *RevaluationHandler) CreateRevaluationRun(w http.ResponseWriter, r *http.Request) {
	var req CreateRevaluationRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	revaluationDate, err := time.Parse("2006-01-02", req.RevaluationDate)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid revaluation_date format (use YYYY-MM-DD)")
		return
	}

	rateDate, err := time.Parse("2006-01-02", req.RateDate)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid rate_date format (use YYYY-MM-DD)")
		return
	}

	functionalCurrency, err := money.GetCurrency(req.FunctionalCurrency)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid functional_currency")
		return
	}

	run, err := h.service.CreateRevaluationRun(r.Context(), service.CreateRevaluationRunRequest{
		EntityID:           entityID,
		FiscalPeriodID:     common.ID(req.FiscalPeriodID),
		RevaluationDate:    revaluationDate,
		RateDate:           rateDate,
		FunctionalCurrency: functionalCurrency,
		CreatedBy:          userID,
	})
	if err != nil {
		h.logger.Error("failed to create revaluation run", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, run)
}

func (h *RevaluationHandler) GetRevaluationRun(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	run, err := h.service.GetRevaluationRun(r.Context(), id)
	if err == domain.ErrRevaluationRunNotFound {
		h.writeError(w, http.StatusNotFound, "revaluation run not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, run)
}

func (h *RevaluationHandler) ListRevaluationRuns(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.RevaluationRunFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if periodID := r.URL.Query().Get("period_id"); periodID != "" {
		filter.FiscalPeriodID = common.ID(periodID)
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := domain.RevaluationStatus(statusStr)
		filter.Status = &status
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	runs, total, err := h.service.ListRevaluationRuns(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   runs,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *RevaluationHandler) SubmitForApproval(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	run, err := h.service.SubmitForApproval(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to submit revaluation for approval", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, run)
}

func (h *RevaluationHandler) ApproveRevaluation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)
	approverID := common.ID(auth.GetUserIDFromContext(r.Context()))

	run, err := h.service.ApproveRevaluation(r.Context(), id, approverID)
	if err != nil {
		h.logger.Error("failed to approve revaluation", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, run)
}

type PostRevaluationRequest struct {
	JournalEntryID string `json:"journal_entry_id"`
}

func (h *RevaluationHandler) PostRevaluation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	var req PostRevaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	posterID := common.ID(auth.GetUserIDFromContext(r.Context()))

	run, err := h.service.PostRevaluation(r.Context(), service.PostRevaluationRequest{
		RunID:          id,
		PosterID:       posterID,
		JournalEntryID: common.ID(req.JournalEntryID),
	})
	if err != nil {
		h.logger.Error("failed to post revaluation", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, run)
}

type ReverseRevaluationRequest struct {
	ReversalJournalID string `json:"reversal_journal_id"`
}

func (h *RevaluationHandler) ReverseRevaluation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	var req ReverseRevaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	reversedBy := common.ID(auth.GetUserIDFromContext(r.Context()))

	run, err := h.service.ReverseRevaluation(r.Context(), service.ReverseRevaluationRequest{
		RunID:             id,
		ReversedBy:        reversedBy,
		ReversalJournalID: common.ID(req.ReversalJournalID),
	})
	if err != nil {
		h.logger.Error("failed to reverse revaluation", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, run)
}

func (h *RevaluationHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *RevaluationHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
