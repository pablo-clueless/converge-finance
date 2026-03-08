package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
	"converge-finance.com/m/internal/modules/ar/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"go.uber.org/zap"
)

type InvoiceService struct {
	invoiceRepo  repository.InvoiceRepository
	customerRepo repository.CustomerRepository
	glAPI        gl.API
	auditLogger  *audit.Logger
	logger       *zap.Logger
}

func NewInvoiceService(
	invoiceRepo repository.InvoiceRepository,
	customerRepo repository.CustomerRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *InvoiceService {
	return &InvoiceService{
		invoiceRepo:  invoiceRepo,
		customerRepo: customerRepo,
		glAPI:        glAPI,
		auditLogger:  auditLogger,
		logger:       logger,
	}
}

type CreateInvoiceRequest struct {
	EntityID    common.ID
	CustomerID  common.ID
	InvoiceType domain.InvoiceType
	PONumber    string
	InvoiceDate time.Time
	DueDate     *time.Time
	Currency    string
	Description string
	Notes       string
	Lines       []CreateInvoiceLineRequest
	CreatedBy   common.ID
}

type CreateInvoiceLineRequest struct {
	RevenueAccountID common.ID
	ItemCode         string
	Description      string
	Quantity         float64
	UnitPrice        float64
	DiscountPct      float64
	TaxCode          string
	TaxRate          float64
	ProjectID        *common.ID
	CostCenterID     *common.ID
}

func (s *InvoiceService) CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error) {

	customer, err := s.customerRepo.GetByID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	if !customer.CanInvoice() {
		return nil, fmt.Errorf("customer is not eligible for invoicing")
	}

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}

	dueDate := req.DueDate
	if dueDate == nil {
		calculatedDue := customer.GetDueDate(req.InvoiceDate)
		dueDate = &calculatedDue
	}

	invoiceNumber, err := s.invoiceRepo.GetNextInvoiceNumber(ctx, req.EntityID, "INV")
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice number: %w", err)
	}

	invoiceType := req.InvoiceType
	if invoiceType == "" {
		invoiceType = domain.InvoiceTypeStandard
	}

	invoice, err := domain.NewInvoice(
		req.EntityID,
		req.CustomerID,
		invoiceNumber,
		invoiceType,
		req.InvoiceDate,
		*dueDate,
		currency,
		req.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	invoice.PONumber = req.PONumber
	invoice.Description = req.Description
	invoice.Notes = req.Notes
	invoice.BillToAddress = customer.BillingAddress
	if customer.ShippingAddress != nil {
		invoice.ShipToAddress = customer.ShippingAddress
	}

	for _, lineReq := range req.Lines {
		unitPrice := money.New(lineReq.UnitPrice, currency.Code)
		if err := invoice.AddLine(lineReq.RevenueAccountID, lineReq.Description, lineReq.Quantity, unitPrice); err != nil {
			return nil, fmt.Errorf("failed to add line: %w", err)
		}

		lineIdx := len(invoice.Lines) - 1
		invoice.Lines[lineIdx].ItemCode = lineReq.ItemCode

		if lineReq.DiscountPct > 0 {
			invoice.Lines[lineIdx].DiscountPct = lineReq.DiscountPct
			invoice.Lines[lineIdx].DiscountAmt = invoice.Lines[lineIdx].Amount.MultiplyFloat(lineReq.DiscountPct / 100)
		}

		if lineReq.TaxCode != "" {
			invoice.Lines[lineIdx].TaxCode = lineReq.TaxCode
			invoice.Lines[lineIdx].TaxRate = lineReq.TaxRate
			invoice.Lines[lineIdx].TaxAmount = invoice.Lines[lineIdx].Amount.MultiplyFloat(lineReq.TaxRate / 100)
		}

		if lineReq.ProjectID != nil {
			invoice.Lines[lineIdx].ProjectID = lineReq.ProjectID
		}
		if lineReq.CostCenterID != nil {
			invoice.Lines[lineIdx].CostCenterID = lineReq.CostCenterID
		}
	}

	if err := invoice.Validate(); err != nil {
		return nil, err
	}

	if !customer.IsWithinCreditLimit(invoice.TotalAmount) {
		s.logger.Warn("Invoice exceeds customer credit limit",
			zap.String("customer_id", customer.ID.String()),
			zap.String("invoice_id", invoice.ID.String()),
		)
	}

	if err := s.invoiceRepo.Create(ctx, invoice); err != nil {
		return nil, fmt.Errorf("failed to save invoice: %w", err)
	}

	err = s.auditLogger.Log(ctx, "ar_invoice", invoice.ID, "invoice.created", map[string]any{
		"invoice_number": invoice.InvoiceNumber,
		"customer_id":    customer.ID.String(),
		"total_amount":   invoice.TotalAmount.Amount.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	s.logger.Info("AR Invoice created",
		zap.String("invoice_id", invoice.ID.String()),
		zap.String("invoice_number", invoice.InvoiceNumber),
		zap.String("customer_id", customer.ID.String()),
	)

	return invoice, nil
}

func (s *InvoiceService) GetInvoice(ctx context.Context, id common.ID) (*domain.Invoice, error) {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	customer, err := s.customerRepo.GetByID(ctx, invoice.CustomerID)
	if err == nil {
		invoice.Customer = customer
	}

	return invoice, nil
}

func (s *InvoiceService) ListInvoices(ctx context.Context, filter domain.InvoiceFilter) ([]domain.Invoice, int, error) {
	invoices, err := s.invoiceRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.invoiceRepo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return invoices, count, nil
}

func (s *InvoiceService) SubmitInvoice(ctx context.Context, id common.ID) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if err := invoice.Submit(); err != nil {
		return err
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to submit invoice: %w", err)
	}

	s.logger.Info("Invoice submitted for approval",
		zap.String("invoice_id", id.String()),
	)

	return nil
}

func (s *InvoiceService) ApproveInvoice(ctx context.Context, id common.ID, approvedBy common.ID) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	customer, err := s.customerRepo.GetByID(ctx, invoice.CustomerID)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	if err := s.glAPI.ValidatePeriodOpen(ctx, invoice.EntityID, invoice.InvoiceDate); err != nil {
		return fmt.Errorf("cannot post to closed period: %w", err)
	}

	if err := invoice.Approve(approvedBy); err != nil {
		return err
	}

	journalEntry, err := s.postInvoiceToGL(ctx, invoice, customer)
	if err != nil {
		return fmt.Errorf("failed to post to GL: %w", err)
	}

	invoice.JournalEntryID = &journalEntry.ID
	now := time.Now()
	invoice.PostingDate = &now

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	newBalance := customer.CurrentBalance.MustAdd(invoice.TotalAmount)
	customer.UpdateBalance(newBalance)

	if customer.ShouldTriggerCreditHold() {
		customer.Suspend("Auto credit hold: exceeded threshold")
	}

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		s.logger.Error("Failed to update customer balance", zap.Error(err))
	}

	err = s.auditLogger.Log(ctx, "ar_invoice", invoice.ID, "invoice.approved", map[string]any{
		"journal_entry_id": journalEntry.ID.String(),
		"approved_by":      approvedBy.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	s.logger.Info("Invoice approved and posted",
		zap.String("invoice_id", id.String()),
		zap.String("journal_entry_id", journalEntry.ID.String()),
	)

	return nil
}

func (s *InvoiceService) postInvoiceToGL(ctx context.Context, invoice *domain.Invoice, customer *domain.Customer) (*gl.JournalEntryResponse, error) {

	lines := make([]gl.JournalLineRequest, 0, len(invoice.Lines)+1)

	arAccountID := customer.ARAccountID
	if arAccountID == nil {
		return nil, fmt.Errorf("customer has no AR account configured")
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *arAccountID,
		Description: fmt.Sprintf("AR - %s - Invoice %s", customer.Name, invoice.InvoiceNumber),
		Debit:       invoice.TotalAmount,
		Credit:      money.Zero(invoice.Currency),
	})

	for _, line := range invoice.Lines {
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   line.RevenueAccountID,
			Description: line.Description,
			Debit:       money.Zero(invoice.Currency),
			Credit:      line.Amount,
		})
	}

	req := gl.CreateJournalEntryRequest{
		EntityID:     invoice.EntityID,
		EntryDate:    invoice.InvoiceDate,
		Description:  fmt.Sprintf("AR Invoice %s to %s", invoice.InvoiceNumber, customer.Name),
		CurrencyCode: invoice.Currency.Code,
		Lines:        lines,
	}

	journalEntry, err := s.glAPI.CreateJournalEntry(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.glAPI.PostJournalEntry(ctx, journalEntry.ID); err != nil {
		return nil, err
	}

	return journalEntry, nil
}

func (s *InvoiceService) RejectInvoice(ctx context.Context, id common.ID) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if err := invoice.Reject(); err != nil {
		return err
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to reject invoice: %w", err)
	}

	s.logger.Info("Invoice rejected",
		zap.String("invoice_id", id.String()),
	)

	return nil
}

func (s *InvoiceService) SendInvoice(ctx context.Context, id common.ID) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if err := invoice.Send(); err != nil {
		return err
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to mark invoice as sent: %w", err)
	}

	s.logger.Info("Invoice sent",
		zap.String("invoice_id", id.String()),
	)

	return nil
}

func (s *InvoiceService) VoidInvoice(ctx context.Context, id common.ID, reason string) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if invoice.JournalEntryID != nil {
		_, err := s.glAPI.ReverseJournalEntry(ctx, *invoice.JournalEntryID, time.Now())
		if err != nil {
			return fmt.Errorf("failed to reverse GL entry: %w", err)
		}
	}

	if err := invoice.Void(); err != nil {
		return err
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to void invoice: %w", err)
	}

	customer, err := s.customerRepo.GetByID(ctx, invoice.CustomerID)
	if err == nil {
		newBalance := customer.CurrentBalance.MustSubtract(invoice.TotalAmount)
		customer.UpdateBalance(newBalance)
		_ = s.customerRepo.Update(ctx, customer)
	}

	err = s.auditLogger.Log(ctx, "ar_invoice", invoice.ID, "invoice.voided", map[string]any{
		"reason": reason,
	})
	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	s.logger.Info("Invoice voided",
		zap.String("invoice_id", id.String()),
		zap.String("reason", reason),
	)

	return nil
}

func (s *InvoiceService) WriteOffInvoice(ctx context.Context, id common.ID, reason string) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	writeOffAmount := invoice.BalanceDue

	if err := invoice.WriteOff(reason); err != nil {
		return err
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to write off invoice: %w", err)
	}

	customer, err := s.customerRepo.GetByID(ctx, invoice.CustomerID)
	if err == nil {
		newBalance := customer.CurrentBalance.MustSubtract(writeOffAmount)
		customer.UpdateBalance(newBalance)
		_ = s.customerRepo.Update(ctx, customer)
	}

	err = s.auditLogger.Log(ctx, "ar_invoice", invoice.ID, "invoice.written_off", map[string]any{
		"reason": reason,
		"amount": writeOffAmount.Amount.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	s.logger.Info("Invoice written off",
		zap.String("invoice_id", id.String()),
		zap.String("reason", reason),
	)

	return nil
}

func (s *InvoiceService) GetOverdueInvoices(ctx context.Context, entityID common.ID) ([]domain.Invoice, error) {
	return s.invoiceRepo.GetOverdueInvoices(ctx, entityID, time.Now())
}

func (s *InvoiceService) GetInvoicesForCustomer(ctx context.Context, customerID common.ID) ([]domain.Invoice, error) {
	return s.invoiceRepo.GetUnpaidInvoicesForCustomer(ctx, customerID)
}

func (s *InvoiceService) CreateCreditMemo(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error) {
	req.InvoiceType = domain.InvoiceTypeCredit
	return s.CreateInvoice(ctx, req)
}
