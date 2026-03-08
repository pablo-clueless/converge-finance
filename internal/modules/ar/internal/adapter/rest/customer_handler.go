package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
	"converge-finance.com/m/internal/modules/ar/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type CustomerHandler struct {
	customerService *service.CustomerService
	logger          *zap.Logger
}

func NewCustomerHandler(customerService *service.CustomerService, logger *zap.Logger) *CustomerHandler {
	return &CustomerHandler{
		customerService: customerService,
		logger:          logger,
	}
}

type CreateCustomerRequest struct {
	CustomerCode   string  `json:"customer_code"`
	Name           string  `json:"name"`
	LegalName      string  `json:"legal_name,omitempty"`
	CustomerType   string  `json:"customer_type"`
	TaxID          string  `json:"tax_id,omitempty"`
	Email          string  `json:"email,omitempty"`
	Phone          string  `json:"phone,omitempty"`
	Website        string  `json:"website,omitempty"`
	Currency       string  `json:"currency"`
	PaymentTerms   string  `json:"payment_terms,omitempty"`
	CreditLimit    float64 `json:"credit_limit,omitempty"`
	DunningEnabled bool    `json:"dunning_enabled"`
	Notes          string  `json:"notes,omitempty"`
}

type CustomerResponse struct {
	ID              string  `json:"id"`
	CustomerCode    string  `json:"customer_code"`
	Name            string  `json:"name"`
	LegalName       string  `json:"legal_name,omitempty"`
	CustomerType    string  `json:"customer_type"`
	TaxID           string  `json:"tax_id,omitempty"`
	Email           string  `json:"email,omitempty"`
	Phone           string  `json:"phone,omitempty"`
	Website         string  `json:"website,omitempty"`
	Status          string  `json:"status"`
	Currency        string  `json:"currency"`
	PaymentTerms    string  `json:"payment_terms"`
	CreditLimit     float64 `json:"credit_limit"`
	CurrentBalance  float64 `json:"current_balance"`
	AvailableCredit float64 `json:"available_credit"`
	OnCreditHold    bool    `json:"on_credit_hold"`
	DunningEnabled  bool    `json:"dunning_enabled"`
	Notes           string  `json:"notes,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func (h *CustomerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paymentTerms := domain.PaymentTermsNet30
	if req.PaymentTerms != "" {
		paymentTerms = domain.PaymentTerms(req.PaymentTerms)
	}

	customerType := domain.CustomerTypeBusiness
	if req.CustomerType != "" {
		customerType = domain.CustomerType(req.CustomerType)
	}

	svcReq := service.CreateCustomerRequest{
		EntityID:       common.ID(claims.EntityID),
		CustomerCode:   req.CustomerCode,
		Name:           req.Name,
		LegalName:      req.LegalName,
		CustomerType:   customerType,
		TaxID:          req.TaxID,
		Email:          req.Email,
		Phone:          req.Phone,
		Website:        req.Website,
		Currency:       req.Currency,
		PaymentTerms:   paymentTerms,
		CreditLimit:    req.CreditLimit,
		DunningEnabled: req.DunningEnabled,
		Notes:          req.Notes,
		CreatedBy:      common.ID(claims.UserID),
	}

	customer, err := h.customerService.CreateCustomer(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("Failed to create customer", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toCustomerResponse(customer))
}

func (h *CustomerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	customer, err := h.customerService.GetCustomer(r.Context(), common.ID(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "Customer not found")
		return
	}

	respondJSON(w, http.StatusOK, toCustomerResponse(customer))
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	filter := domain.CustomerFilter{
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
		s := domain.CustomerStatus(status)
		filter.Status = &s
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = search
	}

	if r.URL.Query().Get("on_credit_hold") == "true" {
		hold := true
		filter.OnCreditHold = &hold
	}

	customers, total, err := h.customerService.ListCustomers(r.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list customers", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]CustomerResponse, len(customers))
	for i, c := range customers {
		response[i] = toCustomerResponse(&c)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"customers": response,
		"total":     total,
		"limit":     filter.Limit,
		"offset":    filter.Offset,
	})
}

func (h *CustomerHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var paymentTerms *domain.PaymentTerms
	if req.PaymentTerms != "" {
		pt := domain.PaymentTerms(req.PaymentTerms)
		paymentTerms = &pt
	}

	svcReq := service.UpdateCustomerRequest{
		ID:             common.ID(id),
		Name:           &req.Name,
		LegalName:      &req.LegalName,
		TaxID:          &req.TaxID,
		Email:          &req.Email,
		Phone:          &req.Phone,
		Website:        &req.Website,
		PaymentTerms:   paymentTerms,
		CreditLimit:    &req.CreditLimit,
		DunningEnabled: &req.DunningEnabled,
		Notes:          &req.Notes,
	}

	customer, err := h.customerService.UpdateCustomer(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("Failed to update customer", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toCustomerResponse(customer))
}

func (h *CustomerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.customerService.DeleteCustomer(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to delete customer", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomerHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.customerService.ActivateCustomer(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to activate customer", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomerHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.customerService.DeactivateCustomer(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to deactivate customer", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomerHandler) Suspend(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.customerService.SuspendCustomer(r.Context(), common.ID(id), req.Reason); err != nil {
		h.logger.Error("Failed to suspend customer", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomerHandler) ReleaseCreditHold(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := h.customerService.ReleaseCreditHold(r.Context(), common.ID(id), common.ID(claims.UserID)); err != nil {
		h.logger.Error("Failed to release credit hold", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CustomerHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	balance, err := h.customerService.GetCustomerBalance(r.Context(), common.ID(id))
	if err != nil {
		h.logger.Error("Failed to get customer balance", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"customer_id":      balance.CustomerID.String(),
		"total_invoiced":   balance.TotalInvoiced.Amount.InexactFloat64(),
		"total_received":   balance.TotalReceived.Amount.InexactFloat64(),
		"current_balance":  balance.CurrentBalance.Amount.InexactFloat64(),
		"overdue_balance":  balance.OverdueBalance.Amount.InexactFloat64(),
		"available_credit": balance.AvailableCredit.Amount.InexactFloat64(),
	})
}

func (h *CustomerHandler) Search(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "Search query is required")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	customers, err := h.customerService.SearchCustomers(r.Context(), common.ID(claims.EntityID), query, limit)
	if err != nil {
		h.logger.Error("Failed to search customers", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]CustomerResponse, len(customers))
	for i, c := range customers {
		response[i] = toCustomerResponse(&c)
	}

	respondJSON(w, http.StatusOK, response)
}

func toCustomerResponse(c *domain.Customer) CustomerResponse {
	return CustomerResponse{
		ID:              c.ID.String(),
		CustomerCode:    c.CustomerCode,
		Name:            c.Name,
		LegalName:       c.LegalName,
		CustomerType:    string(c.CustomerType),
		TaxID:           c.TaxID,
		Email:           c.Email,
		Phone:           c.Phone,
		Website:         c.Website,
		Status:          string(c.Status),
		Currency:        c.Currency.Code,
		PaymentTerms:    string(c.PaymentTerms),
		CreditLimit:     c.CreditLimit.Amount.InexactFloat64(),
		CurrentBalance:  c.CurrentBalance.Amount.InexactFloat64(),
		AvailableCredit: c.AvailableCredit.Amount.InexactFloat64(),
		OnCreditHold:    c.OnCreditHold,
		DunningEnabled:  c.DunningEnabled,
		Notes:           c.Notes,
		CreatedAt:       c.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
