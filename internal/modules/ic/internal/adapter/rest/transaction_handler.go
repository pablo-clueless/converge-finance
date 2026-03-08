package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
	"converge-finance.com/m/internal/modules/ic/internal/service"
	"github.com/shopspring/decimal"
)

type TransactionHandler struct {
	*Handler
	txService *service.ICTransactionService
}

func NewTransactionHandler(h *Handler, txService *service.ICTransactionService) *TransactionHandler {
	return &TransactionHandler{
		Handler:   h,
		txService: txService,
	}
}

type CreateTransactionRequest struct {
	FromEntityID    string                   `json:"from_entity_id"`
	ToEntityID      string                   `json:"to_entity_id"`
	TransactionType string                   `json:"transaction_type"`
	TransactionDate string                   `json:"transaction_date"`
	DueDate         string                   `json:"due_date,omitempty"`
	Amount          string                   `json:"amount"`
	Currency        string                   `json:"currency"`
	Description     string                   `json:"description"`
	Reference       string                   `json:"reference,omitempty"`
	Lines           []TransactionLineRequest `json:"lines,omitempty"`
}

type TransactionLineRequest struct {
	Description    string `json:"description"`
	Quantity       string `json:"quantity,omitempty"`
	UnitPrice      string `json:"unit_price,omitempty"`
	Amount         string `json:"amount"`
	CostCenterCode string `json:"cost_center_code,omitempty"`
	ProjectCode    string `json:"project_code,omitempty"`
}

type TransactionResponse struct {
	ID                string     `json:"id"`
	TransactionNumber string     `json:"transaction_number"`
	TransactionType   string     `json:"transaction_type"`
	FromEntityID      string     `json:"from_entity_id"`
	ToEntityID        string     `json:"to_entity_id"`
	TransactionDate   string     `json:"transaction_date"`
	DueDate           string     `json:"due_date,omitempty"`
	Amount            string     `json:"amount"`
	Currency          string     `json:"currency"`
	Description       string     `json:"description"`
	Reference         string     `json:"reference,omitempty"`
	Status            string     `json:"status"`
	FromJournalEntry  string     `json:"from_journal_entry_id,omitempty"`
	ToJournalEntry    string     `json:"to_journal_entry_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	PostedAt          *time.Time `json:"posted_at,omitempty"`
	ReconciledAt      *time.Time `json:"reconciled_at,omitempty"`
}

func toTransactionResponse(tx *domain.ICTransaction) TransactionResponse {
	resp := TransactionResponse{
		ID:                tx.ID.String(),
		TransactionNumber: tx.TransactionNumber,
		TransactionType:   string(tx.TransactionType),
		FromEntityID:      tx.FromEntityID.String(),
		ToEntityID:        tx.ToEntityID.String(),
		TransactionDate:   tx.TransactionDate.Format("2006-01-02"),
		Amount:            tx.Amount.Amount.String(),
		Currency:          tx.Currency.Code,
		Description:       tx.Description,
		Reference:         tx.Reference,
		Status:            string(tx.Status),
		CreatedAt:         tx.CreatedAt,
		PostedAt:          tx.PostedAt,
		ReconciledAt:      tx.ReconciledAt,
	}
	if tx.DueDate != nil {
		resp.DueDate = tx.DueDate.Format("2006-01-02")
	}
	if tx.FromJournalEntryID != nil {
		resp.FromJournalEntry = tx.FromJournalEntryID.String()
	}
	if tx.ToJournalEntryID != nil {
		resp.ToJournalEntry = tx.ToJournalEntryID.String()
	}
	return resp
}

func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)

	var fromEntityID, toEntityID *common.ID
	if id := r.URL.Query().Get("from_entity_id"); id != "" {
		parsed, _ := common.Parse(id)
		fromEntityID = &parsed
	}
	if id := r.URL.Query().Get("to_entity_id"); id != "" {
		parsed, _ := common.Parse(id)
		toEntityID = &parsed
	}

	var txType *domain.TransactionType
	if t := r.URL.Query().Get("transaction_type"); t != "" {
		tt := domain.TransactionType(t)
		txType = &tt
	}

	var status *domain.TransactionStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.TransactionStatus(s)
		status = &st
	}

	filter := domain.ICTransactionFilter{
		EntityID:        &entityID,
		FromEntityID:    fromEntityID,
		ToEntityID:      toEntityID,
		TransactionType: txType,
		Status:          status,
		Limit:           getIntQuery(r, "limit", 50),
		Offset:          getIntQuery(r, "offset", 0),
	}

	txs, total, err := h.txService.ListTransactions(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []TransactionResponse
	for _, tx := range txs {
		responses = append(responses, toTransactionResponse(&tx))
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(responses)) < total,
	})
}

func (h *TransactionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fromEntityID, err := common.Parse(req.FromEntityID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid from_entity_id")
		return
	}

	toEntityID, err := common.Parse(req.ToEntityID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid to_entity_id")
		return
	}

	txDate, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid transaction_date")
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid amount")
		return
	}

	currency := money.MustGetCurrency(req.Currency)

	createReq := service.CreateTransactionRequest{
		FromEntityID:    fromEntityID,
		ToEntityID:      toEntityID,
		TransactionType: domain.TransactionType(req.TransactionType),
		TransactionDate: txDate,
		Amount:          money.NewFromDecimal(amount, currency),
		Description:     req.Description,
		Reference:       req.Reference,
	}

	if req.DueDate != "" {
		dueDate, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid due_date")
			return
		}
		createReq.DueDate = &dueDate
	}

	for _, line := range req.Lines {
		lineAmount, _ := decimal.NewFromString(line.Amount)
		lineQty := decimal.NewFromInt(1)
		if line.Quantity != "" {
			lineQty, _ = decimal.NewFromString(line.Quantity)
		}
		linePrice := decimal.Zero
		if line.UnitPrice != "" {
			linePrice, _ = decimal.NewFromString(line.UnitPrice)
		}

		createReq.Lines = append(createReq.Lines, service.TransactionLineRequest{
			Description:    line.Description,
			Quantity:       lineQty,
			UnitPrice:      linePrice,
			Amount:         money.NewFromDecimal(lineAmount, currency),
			CostCenterCode: line.CostCenterCode,
			ProjectCode:    line.ProjectCode,
		})
	}

	tx, err := h.txService.CreateTransaction(r.Context(), createReq)
	if err != nil {
		if ve, ok := err.(*common.ValidationError); ok {
			respondValidationError(w, ve)
			return
		}
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toTransactionResponse(tx))
}

func (h *TransactionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	tx, err := h.txService.GetTransaction(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "transaction not found")
		return
	}

	respondJSON(w, http.StatusOK, toTransactionResponse(tx))
}

func (h *TransactionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.txService.SubmitTransaction(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, _ := h.txService.GetTransaction(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransactionResponse(tx))
}

func (h *TransactionHandler) Post(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.txService.PostTransaction(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, _ := h.txService.GetTransaction(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransactionResponse(tx))
}

func (h *TransactionHandler) Reconcile(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.txService.ReconcileTransaction(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, _ := h.txService.GetTransaction(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransactionResponse(tx))
}

func (h *TransactionHandler) Dispute(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		req.Reason = ""
	}

	if err := h.txService.DisputeTransaction(r.Context(), id, req.Reason); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, _ := h.txService.GetTransaction(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransactionResponse(tx))
}

func (h *TransactionHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.txService.ResolveDispute(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	tx, _ := h.txService.GetTransaction(r.Context(), id)
	respondJSON(w, http.StatusOK, toTransactionResponse(tx))
}

func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.txService.DeleteTransaction(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "transaction deleted"})
}
