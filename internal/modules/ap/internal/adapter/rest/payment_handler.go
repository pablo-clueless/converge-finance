package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
	"converge-finance.com/m/internal/modules/ap/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type PaymentHandler struct {
	paymentService *service.PaymentService
	logger         *zap.Logger
}

func NewPaymentHandler(paymentService *service.PaymentService, logger *zap.Logger) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		logger:         logger,
	}
}

type CreatePaymentRequest struct {
	VendorID      string                     `json:"vendor_id"`
	PaymentDate   string                     `json:"payment_date"`
	PaymentMethod string                     `json:"payment_method"`
	Currency      string                     `json:"currency"`
	BankAccountID string                     `json:"bank_account_id,omitempty"`
	Memo          string                     `json:"memo,omitempty"`
	Notes         string                     `json:"notes,omitempty"`
	Allocations   []PaymentAllocationRequest `json:"allocations"`
}

type PaymentAllocationRequest struct {
	InvoiceID     string  `json:"invoice_id"`
	Amount        float64 `json:"amount"`
	DiscountTaken float64 `json:"discount_taken,omitempty"`
}

type PaymentResponse struct {
	ID            string                      `json:"id"`
	VendorID      string                      `json:"vendor_id"`
	VendorName    string                      `json:"vendor_name,omitempty"`
	PaymentNumber string                      `json:"payment_number"`
	CheckNumber   string                      `json:"check_number,omitempty"`
	PaymentDate   string                      `json:"payment_date"`
	Status        string                      `json:"status"`
	PaymentType   string                      `json:"payment_type"`
	PaymentMethod string                      `json:"payment_method"`
	Currency      string                      `json:"currency"`
	Amount        float64                     `json:"amount"`
	DiscountTaken float64                     `json:"discount_taken"`
	BankReference string                      `json:"bank_reference,omitempty"`
	Memo          string                      `json:"memo,omitempty"`
	Notes         string                      `json:"notes,omitempty"`
	Allocations   []PaymentAllocationResponse `json:"allocations,omitempty"`
	CreatedAt     string                      `json:"created_at"`
	UpdatedAt     string                      `json:"updated_at"`
}

type PaymentAllocationResponse struct {
	ID            string  `json:"id"`
	InvoiceID     string  `json:"invoice_id"`
	InvoiceNumber string  `json:"invoice_number,omitempty"`
	Amount        float64 `json:"amount"`
	DiscountTaken float64 `json:"discount_taken"`
}

func (h *PaymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paymentDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid payment date format")
		return
	}

	var bankAccountID *common.ID
	if req.BankAccountID != "" {
		bid := common.ID(req.BankAccountID)
		bankAccountID = &bid
	}

	allocations := make([]service.PaymentAllocationRequest, len(req.Allocations))
	for i, alloc := range req.Allocations {
		allocations[i] = service.PaymentAllocationRequest{
			InvoiceID:     common.ID(alloc.InvoiceID),
			Amount:        alloc.Amount,
			DiscountTaken: alloc.DiscountTaken,
		}
	}

	svcReq := service.CreatePaymentRequest{
		EntityID:      common.ID(claims.EntityID),
		VendorID:      common.ID(req.VendorID),
		PaymentDate:   paymentDate,
		PaymentMethod: domain.PaymentMethod(req.PaymentMethod),
		Currency:      req.Currency,
		BankAccountID: bankAccountID,
		Memo:          req.Memo,
		Notes:         req.Notes,
		Allocations:   allocations,
		CreatedBy:     common.ID(claims.UserID),
	}

	payment, err := h.paymentService.CreatePayment(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("Failed to create payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toPaymentResponse(payment))
}

func (h *PaymentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	payment, err := h.paymentService.GetPayment(r.Context(), common.ID(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "Payment not found")
		return
	}

	respondJSON(w, http.StatusOK, toPaymentResponse(payment))
}

func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	filter := domain.PaymentFilter{
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
		s := domain.PaymentStatus(status)
		filter.Status = &s
	}

	if vendorID := r.URL.Query().Get("vendor_id"); vendorID != "" {
		vid := common.ID(vendorID)
		filter.VendorID = &vid
	}

	if method := r.URL.Query().Get("payment_method"); method != "" {
		pm := domain.PaymentMethod(method)
		filter.PaymentMethod = &pm
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = search
	}

	payments, total, err := h.paymentService.ListPayments(r.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list payments", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]PaymentResponse, len(payments))
	for i, pay := range payments {
		response[i] = toPaymentResponse(&pay)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"payments": response,
		"total":    total,
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	})
}

func (h *PaymentHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.paymentService.SubmitPayment(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to submit payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := h.paymentService.ApprovePayment(r.Context(), common.ID(id), common.ID(claims.UserID)); err != nil {
		h.logger.Error("Failed to approve payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.paymentService.RejectPayment(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to reject payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) Schedule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		ScheduledDate string `json:"scheduled_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	scheduledDate, err := time.Parse("2006-01-02", req.ScheduledDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid scheduled date format")
		return
	}

	if err := h.paymentService.SchedulePayment(r.Context(), common.ID(id), scheduledDate); err != nil {
		h.logger.Error("Failed to schedule payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) Process(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.paymentService.ProcessPayment(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to process payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) Complete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		BankReference string `json:"bank_reference"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.paymentService.CompletePayment(r.Context(), common.ID(id), req.BankReference); err != nil {
		h.logger.Error("Failed to complete payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) Fail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.paymentService.FailPayment(r.Context(), common.ID(id), req.Reason); err != nil {
		h.logger.Error("Failed to fail payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) Void(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.paymentService.VoidPayment(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to void payment", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PaymentHandler) GetScheduled(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	beforeDate := time.Now().AddDate(0, 0, 7)
	if before := r.URL.Query().Get("before"); before != "" {
		if parsed, err := time.Parse("2006-01-02", before); err == nil {
			beforeDate = parsed
		}
	}

	payments, err := h.paymentService.GetScheduledPayments(r.Context(), common.ID(claims.EntityID), beforeDate)
	if err != nil {
		h.logger.Error("Failed to get scheduled payments", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]PaymentResponse, len(payments))
	for i, pay := range payments {
		response[i] = toPaymentResponse(&pay)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *PaymentHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
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

	summary, err := h.paymentService.GetPaymentsSummary(r.Context(), common.ID(claims.EntityID), startDate, endDate)
	if err != nil {
		h.logger.Error("Failed to get payments summary", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"start_date":       startDate.Format("2006-01-02"),
		"end_date":         endDate.Format("2006-01-02"),
		"total_payments":   summary.TotalPayments,
		"completed_count":  summary.CompletedCount,
		"pending_count":    summary.PendingCount,
		"scheduled_count":  summary.ScheduledCount,
		"total_amount":     summary.TotalAmount.Amount.InexactFloat64(),
		"completed_amount": summary.CompletedAmount.Amount.InexactFloat64(),
		"pending_amount":   summary.PendingAmount.Amount.InexactFloat64(),
		"scheduled_amount": summary.ScheduledAmount.Amount.InexactFloat64(),
		"discounts_taken":  summary.TotalDiscountsTaken.Amount.InexactFloat64(),
		"check_amount":     summary.CheckAmount.Amount.InexactFloat64(),
		"ach_amount":       summary.ACHAmount.Amount.InexactFloat64(),
		"wire_amount":      summary.WireAmount.Amount.InexactFloat64(),
	})
}

func toPaymentResponse(pay *domain.Payment) PaymentResponse {
	resp := PaymentResponse{
		ID:            pay.ID.String(),
		VendorID:      pay.VendorID.String(),
		PaymentNumber: pay.PaymentNumber,
		CheckNumber:   pay.CheckNumber,
		PaymentDate:   pay.PaymentDate.Format("2006-01-02"),
		Status:        string(pay.Status),
		PaymentType:   string(pay.PaymentType),
		PaymentMethod: string(pay.PaymentMethod),
		Currency:      pay.Currency.Code,
		Amount:        pay.Amount.Amount.InexactFloat64(),
		DiscountTaken: pay.DiscountTaken.Amount.InexactFloat64(),
		BankReference: pay.BankReference,
		Memo:          pay.Memo,
		Notes:         pay.Notes,
		CreatedAt:     pay.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     pay.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if pay.Vendor != nil {
		resp.VendorName = pay.Vendor.Name
	}

	if len(pay.Allocations) > 0 {
		resp.Allocations = make([]PaymentAllocationResponse, len(pay.Allocations))
		for i, alloc := range pay.Allocations {
			resp.Allocations[i] = PaymentAllocationResponse{
				ID:            alloc.ID.String(),
				InvoiceID:     alloc.InvoiceID.String(),
				Amount:        alloc.Amount.Amount.InexactFloat64(),
				DiscountTaken: alloc.DiscountTaken.Amount.InexactFloat64(),
			}
			if alloc.Invoice != nil {
				resp.Allocations[i].InvoiceNumber = alloc.Invoice.InvoiceNumber
			}
		}
	}

	return resp
}
