package currency

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	db     *database.PostgresDB
	logger *zap.Logger
}

type Config struct {
	DB     *database.PostgresDB
	Logger *zap.Logger
}

func NewModule(cfg Config) (*Module, error) {
	return &Module{
		db:     cfg.DB,
		logger: cfg.Logger,
	}, nil
}

func (m *Module) RegisterRoutes(r chi.Router) {
	r.Route("/currencies", func(r chi.Router) {
		r.Get("/", m.listCurrencies)
		r.Post("/", m.createCurrency)
		r.Get("/{code}", m.getCurrency)
		r.Put("/{code}", m.updateCurrency)
		r.Post("/{code}/activate", m.activateCurrency)
		r.Post("/{code}/deactivate", m.deactivateCurrency)
		r.Post("/convert", m.convertCurrency)

		r.Route("/exchange-rates", func(r chi.Router) {
			r.Get("/", m.listExchangeRates)
			r.Post("/", m.createExchangeRate)
			r.Get("/{id}", m.getExchangeRate)
			r.Put("/{id}", m.updateExchangeRate)
			r.Delete("/{id}", m.deleteExchangeRate)
		})
	})
}

type Currency struct {
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Symbol        string    `json:"symbol"`
	DecimalPlaces int       `json:"decimal_places"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
}

type ExchangeRate struct {
	ID            string    `json:"id"`
	FromCurrency  string    `json:"from_currency"`
	ToCurrency    string    `json:"to_currency"`
	Rate          string    `json:"rate"`
	EffectiveDate string    `json:"effective_date"`
	RateType      string    `json:"rate_type"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedBy     *string   `json:"created_by,omitempty"`
}

func (m *Module) listCurrencies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT code, name, symbol, decimal_places, is_active, created_at
		FROM currencies
		ORDER BY code
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query currencies", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to fetch currencies")
		return
	}
	defer rows.Close()

	var currencies []Currency
	for rows.Next() {
		var c Currency
		if err := rows.Scan(&c.Code, &c.Name, &c.Symbol, &c.DecimalPlaces, &c.IsActive, &c.CreatedAt); err != nil {
			m.logger.Error("Failed to scan currency", zap.Error(err))
			continue
		}
		currencies = append(currencies, c)
	}

	if err := rows.Err(); err != nil {
		m.logger.Error("Error iterating currencies", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to fetch currencies")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"currencies": currencies, "total": len(currencies)})
}

func (m *Module) createCurrency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Code          string `json:"code"`
		Name          string `json:"name"`
		Symbol        string `json:"symbol"`
		DecimalPlaces int    `json:"decimal_places"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.DecimalPlaces == 0 {
		req.DecimalPlaces = 2
	}

	query := `
		INSERT INTO currencies (code, name, symbol, decimal_places, is_active, created_at)
		VALUES ($1, $2, $3, $4, TRUE, NOW())
		RETURNING code, name, symbol, decimal_places, is_active, created_at
	`

	var c Currency
	err := m.db.QueryRowContext(ctx, query, req.Code, req.Name, req.Symbol, req.DecimalPlaces).
		Scan(&c.Code, &c.Name, &c.Symbol, &c.DecimalPlaces, &c.IsActive, &c.CreatedAt)
	if err != nil {
		m.logger.Error("Failed to create currency", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to create currency")
		return
	}

	respondJSON(w, http.StatusCreated, c)
}

func (m *Module) getCurrency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := chi.URLParam(r, "code")

	query := `
		SELECT code, name, symbol, decimal_places, is_active, created_at
		FROM currencies
		WHERE code = $1
	`

	var c Currency
	err := m.db.QueryRowContext(ctx, query, code).
		Scan(&c.Code, &c.Name, &c.Symbol, &c.DecimalPlaces, &c.IsActive, &c.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Currency not found")
			return
		}
		m.logger.Error("Failed to get currency", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to fetch currency")
		return
	}

	respondJSON(w, http.StatusOK, c)
}

func (m *Module) updateCurrency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := chi.URLParam(r, "code")

	var req struct {
		Name          *string `json:"name,omitempty"`
		Symbol        *string `json:"symbol,omitempty"`
		DecimalPlaces *int    `json:"decimal_places,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Build dynamic update query
	query := `UPDATE currencies SET `
	args := []any{}
	argIdx := 1
	updates := []string{}

	if req.Name != nil {
		updates = append(updates, "name = $"+string(rune('0'+argIdx)))
		args = append(args, *req.Name)
		argIdx++
	}
	if req.Symbol != nil {
		updates = append(updates, "symbol = $"+string(rune('0'+argIdx)))
		args = append(args, *req.Symbol)
		argIdx++
	}
	if req.DecimalPlaces != nil {
		updates = append(updates, "decimal_places = $"+string(rune('0'+argIdx)))
		args = append(args, *req.DecimalPlaces)
		argIdx++
	}

	if len(updates) == 0 {
		respondError(w, http.StatusBadRequest, "No fields to update")
		return
	}

	for i, u := range updates {
		if i > 0 {
			query += ", "
		}
		query += u
	}
	query += " WHERE code = $" + string(rune('0'+argIdx)) + " RETURNING code, name, symbol, decimal_places, is_active, created_at"
	args = append(args, code)

	var c Currency
	err := m.db.QueryRowContext(ctx, query, args...).
		Scan(&c.Code, &c.Name, &c.Symbol, &c.DecimalPlaces, &c.IsActive, &c.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Currency not found")
			return
		}
		m.logger.Error("Failed to update currency", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to update currency")
		return
	}

	respondJSON(w, http.StatusOK, c)
}

func (m *Module) activateCurrency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := chi.URLParam(r, "code")

	_, err := m.db.ExecContext(ctx, "UPDATE currencies SET is_active = TRUE WHERE code = $1", code)
	if err != nil {
		m.logger.Error("Failed to activate currency", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to activate currency")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

func (m *Module) deactivateCurrency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := chi.URLParam(r, "code")

	_, err := m.db.ExecContext(ctx, "UPDATE currencies SET is_active = FALSE WHERE code = $1", code)
	if err != nil {
		m.logger.Error("Failed to deactivate currency", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to deactivate currency")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}

func (m *Module) convertCurrency(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		Amount       string `json:"amount"`
		FromCurrency string `json:"from_currency"`
		ToCurrency   string `json:"to_currency"`
		Date         string `json:"date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get the exchange rate
	query := `
		SELECT rate, effective_date
		FROM exchange_rates
		WHERE from_currency = $1 AND to_currency = $2
		AND effective_date <= COALESCE($3::date, CURRENT_DATE)
		ORDER BY effective_date DESC
		LIMIT 1
	`

	var rate string
	var effectiveDate time.Time
	var dateParam any
	if req.Date != "" {
		dateParam = req.Date
	}

	err := m.db.QueryRowContext(ctx, query, req.FromCurrency, req.ToCurrency, dateParam).
		Scan(&rate, &effectiveDate)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Exchange rate not found")
			return
		}
		m.logger.Error("Failed to get exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to convert currency")
		return
	}

	// Simple conversion (in production, use proper decimal math)
	respondJSON(w, http.StatusOK, map[string]any{
		"original_amount":  req.Amount,
		"from_currency":    req.FromCurrency,
		"converted_amount": req.Amount, // Placeholder - real math would go here
		"to_currency":      req.ToCurrency,
		"exchange_rate":    rate,
		"rate_date":        effectiveDate.Format("2006-01-02"),
	})
}

func (m *Module) listExchangeRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query := `
		SELECT id, from_currency, to_currency, rate::text, rate_type, effective_date, created_at, created_by
		FROM exchange_rates
		ORDER BY effective_date DESC
		LIMIT 100
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query exchange rates", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to fetch exchange rates")
		return
	}
	defer rows.Close()

	var rates []ExchangeRate
	for rows.Next() {
		var er ExchangeRate
		var effectiveDate time.Time
		if err := rows.Scan(&er.ID, &er.FromCurrency, &er.ToCurrency, &er.Rate, &er.RateType, &effectiveDate, &er.CreatedAt, &er.CreatedBy); err != nil {
			m.logger.Error("Failed to scan exchange rate", zap.Error(err))
			continue
		}
		er.EffectiveDate = effectiveDate.Format("2006-01-02")
		rates = append(rates, er)
	}

	respondJSON(w, http.StatusOK, map[string]any{"rates": rates, "total": len(rates)})
}

func (m *Module) createExchangeRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		FromCurrency  string `json:"from_currency"`
		ToCurrency    string `json:"to_currency"`
		Rate          string `json:"rate"`
		EffectiveDate string `json:"effective_date"`
		RateType      string `json:"rate_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.RateType == "" {
		req.RateType = "spot"
	}

	// Generate a ULID for the ID
	id := generateULID()

	query := `
		INSERT INTO exchange_rates (id, from_currency, to_currency, rate, rate_type, effective_date, created_at)
		VALUES ($1, $2, $3, $4::decimal, $5, $6::date, NOW())
		RETURNING id, from_currency, to_currency, rate::text, rate_type, effective_date, created_at
	`

	var er ExchangeRate
	var effectiveDate time.Time
	err := m.db.QueryRowContext(ctx, query, id, req.FromCurrency, req.ToCurrency, req.Rate, req.RateType, req.EffectiveDate).
		Scan(&er.ID, &er.FromCurrency, &er.ToCurrency, &er.Rate, &er.RateType, &effectiveDate, &er.CreatedAt)
	if err != nil {
		m.logger.Error("Failed to create exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to create exchange rate")
		return
	}
	er.EffectiveDate = effectiveDate.Format("2006-01-02")

	respondJSON(w, http.StatusCreated, er)
}

func (m *Module) getExchangeRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	query := `
		SELECT id, from_currency, to_currency, rate::text, rate_type, effective_date, created_at, created_by
		FROM exchange_rates
		WHERE id = $1
	`

	var er ExchangeRate
	var effectiveDate time.Time
	err := m.db.QueryRowContext(ctx, query, id).
		Scan(&er.ID, &er.FromCurrency, &er.ToCurrency, &er.Rate, &er.RateType, &effectiveDate, &er.CreatedAt, &er.CreatedBy)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Exchange rate not found")
			return
		}
		m.logger.Error("Failed to get exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to fetch exchange rate")
		return
	}
	er.EffectiveDate = effectiveDate.Format("2006-01-02")

	respondJSON(w, http.StatusOK, er)
}

func (m *Module) updateExchangeRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var req struct {
		Rate     *string `json:"rate,omitempty"`
		RateType *string `json:"rate_type,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	query := `
		UPDATE exchange_rates
		SET rate = COALESCE($2::decimal, rate),
		    rate_type = COALESCE($3, rate_type)
		WHERE id = $1
		RETURNING id, from_currency, to_currency, rate::text, rate_type, effective_date, created_at
	`

	var er ExchangeRate
	var effectiveDate time.Time
	err := m.db.QueryRowContext(ctx, query, id, req.Rate, req.RateType).
		Scan(&er.ID, &er.FromCurrency, &er.ToCurrency, &er.Rate, &er.RateType, &effectiveDate, &er.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			respondError(w, http.StatusNotFound, "Exchange rate not found")
			return
		}
		m.logger.Error("Failed to update exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to update exchange rate")
		return
	}
	er.EffectiveDate = effectiveDate.Format("2006-01-02")

	respondJSON(w, http.StatusOK, er)
}

func (m *Module) deleteExchangeRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	_, err := m.db.ExecContext(ctx, "DELETE FROM exchange_rates WHERE id = $1", id)
	if err != nil {
		m.logger.Error("Failed to delete exchange rate", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to delete exchange rate")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func generateULID() string {
	// Simple ULID-like ID generator (in production use github.com/oklog/ulid)
	const chars = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	now := time.Now().UnixMilli()

	// Timestamp part (10 chars)
	ts := make([]byte, 10)
	for i := 9; i >= 0; i-- {
		ts[i] = chars[now%32]
		now /= 32
	}

	// Random part (16 chars)
	rand := make([]byte, 16)
	for i := range rand {
		rand[i] = chars[time.Now().UnixNano()%32]
	}

	return string(ts) + string(rand)
}
