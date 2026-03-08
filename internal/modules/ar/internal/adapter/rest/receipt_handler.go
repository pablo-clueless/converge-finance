package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
	"converge-finance.com/m/internal/modules/ar/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ReceiptHandler struct {
	receiptService *service.ReceiptService
	logger         *zap.Logger
}

func NewReceiptHandler(receiptService *service.ReceiptService, logger *zap.Logger) *ReceiptHandler {
	return &ReceiptHandler{
		receiptService: receiptService,
		logger:         logger,
	}
}

type CreateReceiptRequest struct {
	CustomerID      string                      `json:"customer_id"`
	ReceiptDate     string                      `json:"receipt_date"`
	ReceiptMethod   string                      `json:"receipt_method"`
	Currency        string                      `json:"currency"`
	Amount          float64                     `json:"amount"`
	CheckNumber     string                      `json:"check_number,omitempty"`
	ReferenceNumber string                      `json:"reference_number,omitempty"`
	BankAccountID   string                      `json:"bank_account_id,omitempty"`
	Memo            string                      `json:"memo,omitempty"`
	Notes           string                      `json:"notes,omitempty"`
	Applications    []ReceiptApplicationRequest `json:"applications,omitempty"`
}

type ReceiptApplicationRequest struct {
	InvoiceID     string  `json:"invoice_id"`
	Amount        float64 `json:"amount"`
	DiscountTaken float64 `json:"discount_taken,omitempty"`
}

type ReceiptResponse struct {
	ID              string                       `json:"id"`
	CustomerID      string                       `json:"customer_id"`
	CustomerName    string                       `json:"customer_name,omitempty"`
	ReceiptNumber   string                       `json:"receipt_number"`
	CheckNumber     string                       `json:"check_number,omitempty"`
	ReferenceNumber string                       `json:"reference_number,omitempty"`
	ReceiptDate     string                       `json:"receipt_date"`
	Status          string                       `json:"status"`
	ReceiptMethod   string                       `json:"receipt_method"`
	Currency        string                       `json:"currency"`
	Amount          float64                      `json:"amount"`
	AppliedAmount   float64                      `json:"applied_amount"`
	UnappliedAmount float64                      `json:"unapplied_amount"`
	BankReference   string                       `json:"bank_reference,omitempty"`
	Memo            string                       `json:"memo,omitempty"`
	Notes           string                       `json:"notes,omitempty"`
	Applications    []ReceiptApplicationResponse `json:"applications,omitempty"`
	CreatedAt       string                       `json:"created_at"`
	UpdatedAt       string                       `json:"updated_at"`
}

type ReceiptApplicationResponse struct {
	ID            string  `json:"id"`
	InvoiceID     string  `json:"invoice_id"`
	InvoiceNumber string  `json:"invoice_number,omitempty"`
	Amount        float64 `json:"amount"`
	DiscountTaken float64 `json:"discount_taken"`
}

func (h *ReceiptHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	receiptDate, err := time.Parse("2006-01-02", req.ReceiptDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid receipt date format")
		return
	}

	var bankAccountID *common.ID
	if req.BankAccountID != "" {
		bid := common.ID(req.BankAccountID)
		bankAccountID = &bid
	}

	applications := make([]service.ReceiptApplicationRequest, len(req.Applications))
	for i, app := range req.Applications {
		applications[i] = service.ReceiptApplicationRequest{
			InvoiceID:     common.ID(app.InvoiceID),
			Amount:        app.Amount,
			DiscountTaken: app.DiscountTaken,
		}
	}

	svcReq := service.CreateReceiptRequest{
		EntityID:        common.ID(claims.EntityID),
		CustomerID:      common.ID(req.CustomerID),
		ReceiptDate:     receiptDate,
		ReceiptMethod:   domain.ReceiptMethod(req.ReceiptMethod),
		Currency:        req.Currency,
		Amount:          req.Amount,
		CheckNumber:     req.CheckNumber,
		ReferenceNumber: req.ReferenceNumber,
		BankAccountID:   bankAccountID,
		Memo:            req.Memo,
		Notes:           req.Notes,
		Applications:    applications,
		CreatedBy:       common.ID(claims.UserID),
	}

	receipt, err := h.receiptService.CreateReceipt(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("Failed to create receipt", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toReceiptResponse(receipt))
}

func (h *ReceiptHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	receipt, err := h.receiptService.GetReceipt(r.Context(), common.ID(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "Receipt not found")
		return
	}

	respondJSON(w, http.StatusOK, toReceiptResponse(receipt))
}

func (h *ReceiptHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	filter := domain.ReceiptFilter{
		EntityID: common.ID(claims.EntityID),
		Limit:    50,
		Offset:   0,
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.ReceiptStatus(status)
		filter.Status = &s
	}

	if customerID := r.URL.Query().Get("customer_id"); customerID != "" {
		cid := common.ID(customerID)
		filter.CustomerID = &cid
	}

	if r.URL.Query().Get("unapplied") == "true" {
		filter.Unapplied = true
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = search
	}

	receipts, total, err := h.receiptService.ListReceipts(r.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list receipts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ReceiptResponse, len(receipts))
	for i, rcp := range receipts {
		response[i] = toReceiptResponse(&rcp)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"receipts": response,
		"total":    total,
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	})
}

func (h *ReceiptHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.receiptService.ConfirmReceipt(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to confirm receipt", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ReceiptHandler) Reverse(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.receiptService.ReverseReceipt(r.Context(), common.ID(id), req.Reason, common.ID(claims.UserID)); err != nil {
		h.logger.Error("Failed to reverse receipt", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ReceiptHandler) Void(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.receiptService.VoidReceipt(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to void receipt", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ReceiptHandler) GetUnapplied(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	receipts, err := h.receiptService.GetUnappliedReceipts(r.Context(), common.ID(claims.EntityID))
	if err != nil {
		h.logger.Error("Failed to get unapplied receipts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ReceiptResponse, len(receipts))
	for i, rcp := range receipts {
		response[i] = toReceiptResponse(&rcp)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ReceiptHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	endDate := startDate.AddDate(0, 1, -1)

	if start := r.URL.Query().Get("start_date"); start != "" {
		if parsed, err := time.Parse("2006-01-02", start); err == nil {
			startDate = parsed
		}
	}

	if end := r.URL.Query().Get("end_date"); end != "" {
		if parsed, err := time.Parse("2006-01-02", end); err == nil {
			endDate = parsed
		}
	}

	summary, err := h.receiptService.GetReceiptsSummary(r.Context(), common.ID(claims.EntityID), startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get receipts summary", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"start_date":       startDate.Format("2006-01-02"),
		"end_date":         endDate.Format("2006-01-02"),
		"total_receipts":   summary.TotalReceipts,
		"confirmed_count":  summary.ConfirmedCount,
		"applied_count":    summary.AppliedCount,
		"reversed_count":   summary.ReversedCount,
		"total_amount":     summary.TotalAmount.Amount.InexactFloat64(),
		"confirmed_amount": summary.ConfirmedAmount.Amount.InexactFloat64(),
		"applied_amount":   summary.AppliedAmount.Amount.InexactFloat64(),
		"unapplied_amount": summary.UnappliedAmount.Amount.InexactFloat64(),
		"total_discounts":  summary.TotalDiscounts.Amount.InexactFloat64(),
	})
}

func toReceiptResponse(rcp *domain.Receipt) ReceiptResponse {
	resp := ReceiptResponse{
		ID:              rcp.ID.String(),
		CustomerID:      rcp.CustomerID.String(),
		ReceiptNumber:   rcp.ReceiptNumber,
		CheckNumber:     rcp.CheckNumber,
		ReferenceNumber: rcp.ReferenceNumber,
		ReceiptDate:     rcp.ReceiptDate.Format("2006-01-02"),
		Status:          string(rcp.Status),
		ReceiptMethod:   string(rcp.ReceiptMethod),
		Currency:        rcp.Currency.Code,
		Amount:          rcp.Amount.Amount.InexactFloat64(),
		AppliedAmount:   rcp.AppliedAmount.Amount.InexactFloat64(),
		UnappliedAmount: rcp.UnappliedAmount.Amount.InexactFloat64(),
		BankReference:   rcp.BankReference,
		Memo:            rcp.Memo,
		Notes:           rcp.Notes,
		CreatedAt:       rcp.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       rcp.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if rcp.Customer != nil {
		resp.CustomerName = rcp.Customer.Name
	}

	if len(rcp.Applications) > 0 {
		resp.Applications = make([]ReceiptApplicationResponse, len(rcp.Applications))
		for i, app := range rcp.Applications {
			resp.Applications[i] = ReceiptApplicationResponse{
				ID:            app.ID.String(),
				InvoiceID:     app.InvoiceID.String(),
				Amount:        app.Amount.Amount.InexactFloat64(),
				DiscountTaken: app.DiscountTaken.Amount.InexactFloat64(),
			}
			if app.Invoice != nil {
				resp.Applications[i].InvoiceNumber = app.Invoice.InvoiceNumber
			}
		}
	}

	return resp
}
