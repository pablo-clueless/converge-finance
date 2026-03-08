package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
	"converge-finance.com/m/internal/modules/ap/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"go.uber.org/zap"
)

type InvoiceService struct {
	invoiceRepo repository.InvoiceRepository
	vendorRepo  repository.VendorRepository
	glAPI       gl.API
	auditLogger *audit.Logger
	logger      *zap.Logger
}

func NewInvoiceService(
	invoiceRepo repository.InvoiceRepository,
	vendorRepo repository.VendorRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *InvoiceService {
	return &InvoiceService{
		invoiceRepo: invoiceRepo,
		vendorRepo:  vendorRepo,
		glAPI:       glAPI,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

type CreateInvoiceRequest struct {
	EntityID      common.ID
	VendorID      common.ID
	InvoiceNumber string
	PONumber      string
	InvoiceDate   time.Time
	DueDate       *time.Time
	Currency      string
	Description   string
	Notes         string
	Lines         []CreateInvoiceLineRequest
	CreatedBy     common.ID
}

type CreateInvoiceLineRequest struct {
	AccountID    common.ID
	Description  string
	Quantity     float64
	UnitPrice    float64
	TaxCode      string
	TaxAmount    float64
	ItemCode     string
	ProjectID    *common.ID
	CostCenterID *common.ID
}

func (s *InvoiceService) CreateInvoice(ctx context.Context, req CreateInvoiceRequest) (*domain.Invoice, error) {

	vendor, err := s.vendorRepo.GetByID(ctx, req.VendorID)
	if err != nil {
		return nil, fmt.Errorf("vendor not found: %w", err)
	}

	if vendor.Status != domain.VendorStatusActive {
		return nil, fmt.Errorf("vendor is not active")
	}

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}

	duplicate, existingInv, err := s.invoiceRepo.CheckDuplicateInvoice(ctx, req.VendorID, req.InvoiceNumber, 0)
	if err == nil && duplicate && existingInv != nil {
		return nil, fmt.Errorf("duplicate invoice: invoice %s already exists for this vendor", req.InvoiceNumber)
	}

	dueDate := req.DueDate
	if dueDate == nil {
		calculatedDue := vendor.GetDueDate(req.InvoiceDate)
		dueDate = &calculatedDue
	}

	internalNumber, err := s.invoiceRepo.GetNextInternalNumber(ctx, req.EntityID, "AP")
	if err != nil {
		return nil, fmt.Errorf("failed to generate internal number: %w", err)
	}

	invoice, err := domain.NewInvoice(
		req.EntityID,
		req.VendorID,
		req.InvoiceNumber,
		req.InvoiceDate,
		*dueDate,
		currency,
		req.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	invoice.InternalNumber = internalNumber
	invoice.PONumber = req.PONumber
	invoice.Description = req.Description
	invoice.Notes = req.Notes

	for _, lineReq := range req.Lines {
		unitPrice := money.New(lineReq.UnitPrice, currency.Code)
		if err := invoice.AddLine(lineReq.AccountID, lineReq.Description, lineReq.Quantity, unitPrice); err != nil {
			return nil, fmt.Errorf("failed to add line: %w", err)
		}

		lineIdx := len(invoice.Lines) - 1
		if lineReq.TaxCode != "" {
			invoice.Lines[lineIdx].TaxCode = lineReq.TaxCode
			invoice.Lines[lineIdx].TaxAmount = money.New(lineReq.TaxAmount, currency.Code)
		}
		if lineReq.ItemCode != "" {
			invoice.Lines[lineIdx].ItemCode = lineReq.ItemCode
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

	if !vendor.IsWithinCreditLimit(invoice.TotalAmount) {
		s.logger.Warn("Invoice exceeds vendor credit limit",
			zap.String("vendor_id", vendor.ID.String()),
			zap.String("invoice_id", invoice.ID.String()),
		)

	}

	err = s.invoiceRepo.Create(ctx, invoice)
	if err != nil {
		return nil, fmt.Errorf("failed to save invoice: %w", err)
	}

	err = s.auditLogger.Log(ctx, "ap_invoice", invoice.ID, "invoice.created", map[string]any{
		"invoice_number":  invoice.InvoiceNumber,
		"internal_number": invoice.InternalNumber,
		"vendor_id":       vendor.ID.String(),
		"total_amount":    invoice.TotalAmount.Amount.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	s.logger.Info("AP Invoice created",
		zap.String("invoice_id", invoice.ID.String()),
		zap.String("invoice_number", invoice.InvoiceNumber),
		zap.String("vendor_id", vendor.ID.String()),
	)

	return invoice, nil
}

func (s *InvoiceService) GetInvoice(ctx context.Context, id common.ID) (*domain.Invoice, error) {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	vendor, err := s.vendorRepo.GetByID(ctx, invoice.VendorID)
	if err == nil {
		invoice.Vendor = vendor
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

func (s *InvoiceService) ApproveInvoice(ctx context.Context, id common.ID, approvedBy common.ID, notes string) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	vendor, err := s.vendorRepo.GetByID(ctx, invoice.VendorID)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	if err := s.glAPI.ValidatePeriodOpen(ctx, invoice.EntityID, invoice.InvoiceDate); err != nil {
		return fmt.Errorf("cannot post to closed period: %w", err)
	}

	if err := invoice.Approve(approvedBy, notes); err != nil {
		return err
	}

	journalEntry, err := s.postInvoiceToGL(ctx, invoice, vendor)
	if err != nil {
		return fmt.Errorf("failed to post to GL: %w", err)
	}

	invoice.JournalEntryID = &journalEntry.ID
	now := time.Now()
	invoice.PostingDate = &now

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	newBalance := vendor.CurrentBalance.MustAdd(invoice.TotalAmount)
	vendor.UpdateBalance(newBalance)
	if err := s.vendorRepo.Update(ctx, vendor); err != nil {
		s.logger.Error("Failed to update vendor balance", zap.Error(err))
	}

	err = s.auditLogger.Log(ctx, "ap_invoice", invoice.ID, "invoice.approved", map[string]any{
		"journal_entry_id": journalEntry.ID.String(),
		"notes":            notes,
	})
	if err != nil {
		return fmt.Errorf("failed to log posted run action: %w", err)
	}

	s.logger.Info("Invoice approved and posted",
		zap.String("invoice_id", id.String()),
		zap.String("journal_entry_id", journalEntry.ID.String()),
	)

	return nil
}

func (s *InvoiceService) postInvoiceToGL(ctx context.Context, invoice *domain.Invoice, vendor *domain.Vendor) (*gl.JournalEntryResponse, error) {

	lines := make([]gl.JournalLineRequest, 0, len(invoice.Lines)+1)

	for _, line := range invoice.Lines {
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   line.AccountID,
			Description: line.Description,
			Debit:       line.Amount,
			Credit:      money.Zero(invoice.Currency),
		})
	}

	apAccountID := vendor.APAccountID
	if apAccountID == nil {
		return nil, fmt.Errorf("vendor has no AP account configured")
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *apAccountID,
		Description: fmt.Sprintf("AP - %s - Invoice %s", vendor.Name, invoice.InvoiceNumber),
		Debit:       money.Zero(invoice.Currency),
		Credit:      invoice.TotalAmount,
	})

	req := gl.CreateJournalEntryRequest{
		EntityID:     invoice.EntityID,
		EntryDate:    invoice.InvoiceDate,
		Description:  fmt.Sprintf("AP Invoice %s from %s", invoice.InvoiceNumber, vendor.Name),
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

func (s *InvoiceService) RejectInvoice(ctx context.Context, id common.ID, notes string) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if err := invoice.Reject(notes); err != nil {
		return err
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to reject invoice: %w", err)
	}

	s.logger.Info("Invoice rejected",
		zap.String("invoice_id", id.String()),
		zap.String("notes", notes),
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

	vendor, err := s.vendorRepo.GetByID(ctx, invoice.VendorID)
	if err == nil {
		newBalance := vendor.CurrentBalance.MustSubtract(invoice.TotalAmount)
		vendor.UpdateBalance(newBalance)
		err = s.vendorRepo.Update(ctx, vendor)
		if err != nil {
			s.logger.Fatal("unable to update vendor")
		}
	}

	s.logger.Info("Invoice voided",
		zap.String("invoice_id", id.String()),
		zap.String("reason", reason),
	)

	return nil
}

func (s *InvoiceService) AddInvoiceLine(ctx context.Context, invoiceID common.ID, req CreateInvoiceLineRequest) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if invoice.Status != domain.InvoiceStatusDraft {
		return fmt.Errorf("can only add lines to draft invoices")
	}

	unitPrice := money.New(req.UnitPrice, invoice.Currency.Code)
	if err := invoice.AddLine(req.AccountID, req.Description, req.Quantity, unitPrice); err != nil {
		return err
	}

	lineIdx := len(invoice.Lines) - 1
	if req.TaxCode != "" {
		invoice.Lines[lineIdx].TaxCode = req.TaxCode
		invoice.Lines[lineIdx].TaxAmount = money.New(req.TaxAmount, invoice.Currency.Code)
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	return nil
}

func (s *InvoiceService) RemoveInvoiceLine(ctx context.Context, invoiceID common.ID, lineNumber int) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if err := invoice.RemoveLine(lineNumber); err != nil {
		return err
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	return nil
}

func (s *InvoiceService) GetOverdueInvoices(ctx context.Context, entityID common.ID) ([]domain.Invoice, error) {
	return s.invoiceRepo.GetOverdueInvoices(ctx, entityID, time.Now())
}

func (s *InvoiceService) GetInvoicesForPayment(ctx context.Context, vendorID common.ID) ([]domain.Invoice, error) {
	return s.invoiceRepo.GetUnpaidInvoicesForVendor(ctx, vendorID)
}

func (s *InvoiceService) GetInvoicesWithDiscountAvailable(ctx context.Context, entityID common.ID) ([]domain.Invoice, error) {
	return s.invoiceRepo.GetInvoicesWithEarlyPaymentDiscount(ctx, entityID, time.Now())
}
