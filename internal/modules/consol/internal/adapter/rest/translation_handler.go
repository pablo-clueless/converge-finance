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

type TranslationHandler struct {
	*Handler
	translationService *service.TranslationService
}

func NewTranslationHandler(
	logger *zap.Logger,
	translationService *service.TranslationService,
) *TranslationHandler {
	return &TranslationHandler{
		Handler:            NewHandler(logger),
		translationService: translationService,
	}
}

type CreateExchangeRateRequest struct {
	FromCurrency   string   `json:"from_currency"`
	ToCurrency     string   `json:"to_currency"`
	RateDate       string   `json:"rate_date"`
	ClosingRate    float64  `json:"closing_rate"`
	AverageRate    *float64 `json:"average_rate,omitempty"`
	HistoricalRate *float64 `json:"historical_rate,omitempty"`
	Source         string   `json:"source,omitempty"`
}

type UpdateExchangeRateRequest struct {
	ClosingRate    *float64 `json:"closing_rate,omitempty"`
	AverageRate    *float64 `json:"average_rate,omitempty"`
	HistoricalRate *float64 `json:"historical_rate,omitempty"`
}

type ExchangeRateResponse struct {
	ID             common.ID `json:"id"`
	FromCurrency   string    `json:"from_currency"`
	ToCurrency     string    `json:"to_currency"`
	RateDate       string    `json:"rate_date"`
	ClosingRate    float64   `json:"closing_rate"`
	AverageRate    *float64  `json:"average_rate,omitempty"`
	HistoricalRate *float64  `json:"historical_rate,omitempty"`
	Source         string    `json:"source,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TranslateAmountRequest struct {
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	ToCurrency string  `json:"to_currency"`
	Date       string  `json:"date"`
	RateType   string  `json:"rate_type"`
}

type TranslateAmountResponse struct {
	OriginalAmount   money.Money `json:"original_amount"`
	TranslatedAmount money.Money `json:"translated_amount"`
	ExchangeRate     float64     `json:"exchange_rate"`
	RateType         string      `json:"rate_type"`
}

func (h *TranslationHandler) CreateRate(w http.ResponseWriter, r *http.Request) {
	var req CreateExchangeRateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rateDate, err := time.Parse("2006-01-02", req.RateDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid rate date format")
		return
	}

	fromCurrency := money.MustGetCurrency(req.FromCurrency)
	toCurrency := money.MustGetCurrency(req.ToCurrency)

	rate, err := h.translationService.CreateExchangeRate(
		r.Context(),
		fromCurrency,
		toCurrency,
		rateDate,
		req.ClosingRate,
		req.AverageRate,
		req.HistoricalRate,
		req.Source,
	)
	if err != nil {
		h.logger.Error("failed to create exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toExchangeRateResponse(rate))
}

func (h *TranslationHandler) UpdateRate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid rate ID")
		return
	}

	var req UpdateExchangeRateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rate, err := h.translationService.UpdateExchangeRate(
		r.Context(),
		id,
		req.ClosingRate,
		req.AverageRate,
		req.HistoricalRate,
	)
	if err != nil {
		h.logger.Error("failed to update exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toExchangeRateResponse(rate))
}

func (h *TranslationHandler) DeleteRate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid rate ID")
		return
	}

	if err := h.translationService.DeleteExchangeRate(r.Context(), id); err != nil {
		h.logger.Error("failed to delete exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "exchange rate deleted"})
}

func (h *TranslationHandler) GetRate(w http.ResponseWriter, r *http.Request) {
	fromCurrency := r.URL.Query().Get("from_currency")
	toCurrency := r.URL.Query().Get("to_currency")
	dateStr := r.URL.Query().Get("date")

	if fromCurrency == "" || toCurrency == "" {
		respondError(w, http.StatusBadRequest, "from_currency and to_currency are required")
		return
	}

	date := time.Now()
	if dateStr != "" {
		var err error
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid date format")
			return
		}
	}

	from := money.MustGetCurrency(fromCurrency)
	to := money.MustGetCurrency(toCurrency)

	rate, err := h.translationService.GetExchangeRate(r.Context(), from, to, date)
	if err != nil {
		respondError(w, http.StatusNotFound, "exchange rate not found")
		return
	}

	respondJSON(w, http.StatusOK, toExchangeRateResponse(rate))
}

func (h *TranslationHandler) ListRates(w http.ResponseWriter, r *http.Request) {
	fromCurrency := r.URL.Query().Get("from_currency")
	toCurrency := r.URL.Query().Get("to_currency")
	dateFrom := getDateQuery(r, "date_from")
	dateTo := getDateQuery(r, "date_to")
	limit := getIntQuery(r, "limit", 100)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.ExchangeRateFilter{
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Limit:    limit,
		Offset:   offset,
	}

	if fromCurrency != "" {
		from := money.MustGetCurrency(fromCurrency)
		filter.FromCurrency = &from
	}

	if toCurrency != "" {
		to := money.MustGetCurrency(toCurrency)
		filter.ToCurrency = &to
	}

	rates, err := h.translationService.ListExchangeRates(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list exchange rates", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ExchangeRateResponse, len(rates))
	for i, rate := range rates {
		response[i] = toExchangeRateResponse(&rate)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *TranslationHandler) TranslateAmount(w http.ResponseWriter, r *http.Request) {
	var req TranslateAmountRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid date format")
		return
	}

	rateType := domain.RateType(req.RateType)
	if !rateType.IsValid() {
		rateType = domain.RateTypeClosing
	}

	fromCurrency := money.MustGetCurrency(req.Currency)
	toCurrency := money.MustGetCurrency(req.ToCurrency)
	originalAmount := money.New(req.Amount, req.Currency)

	translatedAmount, exchangeRate, err := h.translationService.TranslateAmount(
		r.Context(),
		originalAmount,
		toCurrency,
		date,
		rateType,
	)
	if err != nil {
		h.logger.Error("failed to translate amount", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = fromCurrency

	respondJSON(w, http.StatusOK, TranslateAmountResponse{
		OriginalAmount:   originalAmount,
		TranslatedAmount: translatedAmount,
		ExchangeRate:     exchangeRate,
		RateType:         string(rateType),
	})
}

func toExchangeRateResponse(rate *domain.ExchangeRate) ExchangeRateResponse {
	return ExchangeRateResponse{
		ID:             rate.ID,
		FromCurrency:   rate.FromCurrency.Code,
		ToCurrency:     rate.ToCurrency.Code,
		RateDate:       rate.RateDate.Format("2006-01-02"),
		ClosingRate:    rate.ClosingRate,
		AverageRate:    rate.AverageRate,
		HistoricalRate: rate.HistoricalRate,
		Source:         rate.Source,
		CreatedAt:      rate.CreatedAt,
		UpdatedAt:      rate.UpdatedAt,
	}
}
