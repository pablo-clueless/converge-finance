package rest

import (
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"converge-finance.com/m/internal/modules/gl/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type JournalHandler struct {
	*Handler
	journalRepo   repository.JournalRepository
	periodRepo    repository.PeriodRepository
	postingEngine *service.PostingEngine
	auditLogger   *audit.Logger
}

func NewJournalHandler(
	logger *zap.Logger,
	journalRepo repository.JournalRepository,
	periodRepo repository.PeriodRepository,
	postingEngine *service.PostingEngine,
	auditLogger *audit.Logger,
) *JournalHandler {
	return &JournalHandler{
		Handler:       NewHandler(logger),
		journalRepo:   journalRepo,
		periodRepo:    periodRepo,
		postingEngine: postingEngine,
		auditLogger:   auditLogger,
	}
}

func (h *JournalHandler) RegisterRoutes(r chi.Router) {
	r.Route("/journals", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
		r.Put("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)
		r.Post("/{id}/submit", h.Submit)
		r.Post("/{id}/post", h.Post)
		r.Post("/{id}/reverse", h.Reverse)
		r.Post("/{id}/lines", h.AddLine)
		r.Delete("/{id}/lines/{lineNumber}", h.RemoveLine)
	})
}

type CreateJournalEntryRequest struct {
	EntryDate   string                     `json:"entry_date"`
	Description string                     `json:"description"`
	Currency    string                     `json:"currency"`
	Lines       []CreateJournalLineRequest `json:"lines"`
}

type CreateJournalLineRequest struct {
	AccountID   string `json:"account_id"`
	Description string `json:"description,omitempty"`
	Debit       string `json:"debit,omitempty"`
	Credit      string `json:"credit,omitempty"`
}

type UpdateJournalEntryRequest struct {
	Description *string `json:"description,omitempty"`
}

type AddJournalLineRequest struct {
	AccountID   string `json:"account_id"`
	Description string `json:"description,omitempty"`
	Debit       string `json:"debit,omitempty"`
	Credit      string `json:"credit,omitempty"`
}

type ReverseJournalEntryRequest struct {
	ReversalDate string `json:"reversal_date"`
}

type JournalEntryResponse struct {
	ID              string                `json:"id"`
	EntityID        string                `json:"entity_id"`
	EntryNumber     string                `json:"entry_number"`
	FiscalPeriodID  string                `json:"fiscal_period_id"`
	EntryDate       string                `json:"entry_date"`
	PostingDate     *string               `json:"posting_date,omitempty"`
	Description     string                `json:"description"`
	Source          string                `json:"source"`
	SourceReference string                `json:"source_reference,omitempty"`
	Status          string                `json:"status"`
	Currency        string                `json:"currency"`
	ExchangeRate    string                `json:"exchange_rate"`
	TotalDebit      string                `json:"total_debit"`
	TotalCredit     string                `json:"total_credit"`
	IsBalanced      bool                  `json:"is_balanced"`
	IsReversing     bool                  `json:"is_reversing"`
	ReversalOfID    *string               `json:"reversal_of_id,omitempty"`
	ReversedByID    *string               `json:"reversed_by_id,omitempty"`
	Lines           []JournalLineResponse `json:"lines"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	PostedAt        *time.Time            `json:"posted_at,omitempty"`
}

type JournalLineResponse struct {
	ID           string `json:"id"`
	LineNumber   int    `json:"line_number"`
	AccountID    string `json:"account_id"`
	Description  string `json:"description,omitempty"`
	DebitAmount  string `json:"debit_amount"`
	CreditAmount string `json:"credit_amount"`
	BaseDebit    string `json:"base_debit"`
	BaseCredit   string `json:"base_credit"`
}

func toJournalEntryResponse(je *domain.JournalEntry) JournalEntryResponse {
	resp := JournalEntryResponse{
		ID:              je.ID.String(),
		EntityID:        je.EntityID.String(),
		EntryNumber:     je.EntryNumber,
		FiscalPeriodID:  je.FiscalPeriodID.String(),
		EntryDate:       je.EntryDate.Format("2006-01-02"),
		Description:     je.Description,
		Source:          string(je.Source),
		SourceReference: je.SourceReference,
		Status:          string(je.Status),
		Currency:        je.Currency.Code,
		ExchangeRate:    je.ExchangeRate.String(),
		TotalDebit:      je.TotalDebits().Amount.String(),
		TotalCredit:     je.TotalCredits().Amount.String(),
		IsBalanced:      je.IsBalanced(),
		IsReversing:     je.IsReversing,
		Lines:           make([]JournalLineResponse, len(je.Lines)),
		CreatedAt:       je.CreatedAt,
		UpdatedAt:       je.UpdatedAt,
		PostedAt:        je.PostedAt,
	}

	if je.PostingDate != nil {
		postingDate := je.PostingDate.Format("2006-01-02")
		resp.PostingDate = &postingDate
	}

	if je.ReversalOfID != nil {
		reversalOfID := je.ReversalOfID.String()
		resp.ReversalOfID = &reversalOfID
	}

	if je.ReversedByID != nil {
		reversedByID := je.ReversedByID.String()
		resp.ReversedByID = &reversedByID
	}

	for i, line := range je.Lines {
		resp.Lines[i] = JournalLineResponse{
			ID:           line.ID.String(),
			LineNumber:   line.LineNumber,
			AccountID:    line.AccountID.String(),
			Description:  line.Description,
			DebitAmount:  line.DebitAmount.Amount.String(),
			CreditAmount: line.CreditAmount.Amount.String(),
			BaseDebit:    line.BaseDebit.Amount.String(),
			BaseCredit:   line.BaseCredit.Amount.String(),
		}
	}

	return resp
}

func (h *JournalHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	filter := domain.JournalEntryFilter{
		EntityID: entityID,
		Limit:    getIntQuery(r, "limit", 50),
		Offset:   getIntQuery(r, "offset", 0),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.JournalEntryStatus(status)
		filter.Status = &s
	}

	if periodID := r.URL.Query().Get("period_id"); periodID != "" {
		id, err := common.Parse(periodID)
		if err == nil {
			filter.FiscalPeriodID = &id
		}
	}

	entries, err := h.journalRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list journal entries", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to list journal entries")
		return
	}

	total, err := h.journalRepo.Count(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to count journal entries", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to count journal entries")
		return
	}

	responses := make([]JournalEntryResponse, len(entries))
	for i, e := range entries {
		responses[i] = toJournalEntryResponse(&e)
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(entries)) < total,
	})
}

func (h *JournalHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	entry, err := h.journalRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to get journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get journal entry")
		return
	}

	respondJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

func (h *JournalHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)
	userID := getUserID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	var req CreateJournalEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	entryDate, err := time.Parse("2006-01-02", req.EntryDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid entry date format (use YYYY-MM-DD)")
		return
	}

	period, err := h.periodRepo.GetPeriodForDate(ctx, entityID, req.EntryDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "No fiscal period found for the entry date")
		return
	}

	if !period.CanPost() {
		respondError(w, http.StatusBadRequest, "Fiscal period is not open for posting")
		return
	}

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid currency code")
		return
	}

	entryNumber, err := h.journalRepo.GetNextEntryNumber(ctx, entityID, "JE")
	if err != nil {
		h.logger.Error("Failed to generate entry number", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to create journal entry")
		return
	}

	entry, err := domain.NewJournalEntry(
		entityID,
		entryNumber,
		period.ID,
		entryDate,
		req.Description,
		currency,
		userID,
	)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	for _, lineReq := range req.Lines {
		accountID, err := common.Parse(lineReq.AccountID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid account ID in line")
			return
		}

		var debit, credit money.Money

		if lineReq.Debit != "" {
			amount, err := decimal.NewFromString(lineReq.Debit)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid debit amount")
				return
			}
			debit = money.NewFromDecimal(amount, currency)
		} else {
			debit = money.Zero(currency)
		}

		if lineReq.Credit != "" {
			amount, err := decimal.NewFromString(lineReq.Credit)
			if err != nil {
				respondError(w, http.StatusBadRequest, "Invalid credit amount")
				return
			}
			credit = money.NewFromDecimal(amount, currency)
		} else {
			credit = money.Zero(currency)
		}

		if err := entry.AddLine(accountID, lineReq.Description, debit, credit); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := entry.Validate(); err != nil {
		if ve, ok := err.(*common.ValidationError); ok {
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.journalRepo.Create(ctx, entry); err != nil {
		h.logger.Error("Failed to create journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to create journal entry")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogCreate(ctx, "gl.journal_entry", entry.ID, map[string]any{
			"entry_number": entry.EntryNumber,
			"description":  entry.Description,
			"total_debit":  entry.TotalDebits().String(),
			"total_credit": entry.TotalCredits().String(),
		})
	}

	respondJSON(w, http.StatusCreated, toJournalEntryResponse(entry))
}

func (h *JournalHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	var req UpdateJournalEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	entry, err := h.journalRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to get journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to update journal entry")
		return
	}

	if !entry.CanModify() {
		respondError(w, http.StatusBadRequest, "Journal entry cannot be modified in current status")
		return
	}

	if req.Description != nil {
		entry.Description = *req.Description
	}
	entry.UpdatedAt = time.Now()

	if err := h.journalRepo.Update(ctx, entry); err != nil {
		h.logger.Error("Failed to update journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to update journal entry")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogUpdate(ctx, "gl.journal_entry", entry.ID, map[string]any{
			"description": entry.Description,
		})
	}

	respondJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

func (h *JournalHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	entry, err := h.journalRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to get journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to delete journal entry")
		return
	}

	if entry.Status != domain.JournalEntryStatusDraft {
		respondError(w, http.StatusBadRequest, "Only draft entries can be deleted")
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "Journal entry deleted"})
}

func (h *JournalHandler) Submit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	entry, err := h.journalRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to get journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to submit journal entry")
		return
	}

	if err := entry.Submit(); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.journalRepo.Update(ctx, entry); err != nil {
		h.logger.Error("Failed to submit journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to submit journal entry")
		return
	}

	if h.auditLogger != nil {
		_ = h.auditLogger.LogAction(ctx, "gl.journal_entry", entry.ID, "submitted", nil)
	}

	respondJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

func (h *JournalHandler) Post(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	if err := h.postingEngine.PostEntry(ctx, id); err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to post journal entry", zap.Error(err))
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := h.journalRepo.GetByID(ctx, id)
	if err != nil {
		h.logger.Error("Failed to get posted journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Entry posted but failed to retrieve")
		return
	}

	respondJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

func (h *JournalHandler) Reverse(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	var req ReverseJournalEntryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	reversal, err := h.postingEngine.ReverseEntry(ctx, id, req.ReversalDate)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to reverse journal entry", zap.Error(err))
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toJournalEntryResponse(reversal))
}

func (h *JournalHandler) AddLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	var req AddJournalLineRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	entry, err := h.journalRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to get journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to add line")
		return
	}

	if !entry.CanModify() {
		respondError(w, http.StatusBadRequest, "Journal entry cannot be modified")
		return
	}

	accountID, err := common.Parse(req.AccountID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	var debit, credit money.Money

	if req.Debit != "" {
		amount, err := decimal.NewFromString(req.Debit)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid debit amount")
			return
		}
		debit = money.NewFromDecimal(amount, entry.Currency)
	} else {
		debit = money.Zero(entry.Currency)
	}

	if req.Credit != "" {
		amount, err := decimal.NewFromString(req.Credit)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid credit amount")
			return
		}
		credit = money.NewFromDecimal(amount, entry.Currency)
	} else {
		credit = money.Zero(entry.Currency)
	}

	if err := entry.AddLine(accountID, req.Description, debit, credit); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.journalRepo.Update(ctx, entry); err != nil {
		h.logger.Error("Failed to update journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to add line")
		return
	}

	respondJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}

func (h *JournalHandler) RemoveLine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid journal entry ID")
		return
	}

	lineNumber := getIntQuery(r, "lineNumber", 0)
	if lineNumber == 0 {

		lineNumberStr := chi.URLParam(r, "lineNumber")
		if lineNumberStr != "" {
			if n, err := strconv.Atoi(lineNumberStr); err == nil {
				lineNumber = n
			}
		}
	}

	if lineNumber <= 0 {
		respondError(w, http.StatusBadRequest, "Invalid line number")
		return
	}

	entry, err := h.journalRepo.GetByID(ctx, id)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Journal entry not found")
			return
		}
		h.logger.Error("Failed to get journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to remove line")
		return
	}

	if err := entry.RemoveLine(lineNumber); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.journalRepo.Update(ctx, entry); err != nil {
		h.logger.Error("Failed to update journal entry", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to remove line")
		return
	}

	respondJSON(w, http.StatusOK, toJournalEntryResponse(entry))
}
