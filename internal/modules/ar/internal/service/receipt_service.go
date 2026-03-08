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

type ReceiptService struct {
	receiptRepo  repository.ReceiptRepository
	invoiceRepo  repository.InvoiceRepository
	customerRepo repository.CustomerRepository
	glAPI        gl.API
	auditLogger  *audit.Logger
	logger       *zap.Logger
}

func NewReceiptService(
	receiptRepo repository.ReceiptRepository,
	invoiceRepo repository.InvoiceRepository,
	customerRepo repository.CustomerRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *ReceiptService {
	return &ReceiptService{
		receiptRepo:  receiptRepo,
		invoiceRepo:  invoiceRepo,
		customerRepo: customerRepo,
		glAPI:        glAPI,
		auditLogger:  auditLogger,
		logger:       logger,
	}
}

type CreateReceiptRequest struct {
	EntityID        common.ID
	CustomerID      common.ID
	ReceiptDate     time.Time
	ReceiptMethod   domain.ReceiptMethod
	Currency        string
	Amount          float64
	CheckNumber     string
	ReferenceNumber string
	BankAccountID   *common.ID
	Memo            string
	Notes           string
	Applications    []ReceiptApplicationRequest
	CreatedBy       common.ID
}

type ReceiptApplicationRequest struct {
	InvoiceID     common.ID
	Amount        float64
	DiscountTaken float64
}

func (s *ReceiptService) CreateReceipt(ctx context.Context, req CreateReceiptRequest) (*domain.Receipt, error) {

	customer, err := s.customerRepo.GetByID(ctx, req.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}

	amount := money.New(req.Amount, currency.Code)

	receiptNumber, err := s.receiptRepo.GetNextReceiptNumber(ctx, req.EntityID, "RCP")
	if err != nil {
		return nil, fmt.Errorf("failed to generate receipt number: %w", err)
	}

	receipt, err := domain.NewReceipt(
		req.EntityID,
		req.CustomerID,
		receiptNumber,
		req.ReceiptDate,
		req.ReceiptMethod,
		currency,
		amount,
		req.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create receipt: %w", err)
	}

	receipt.CheckNumber = req.CheckNumber
	receipt.ReferenceNumber = req.ReferenceNumber
	receipt.BankAccountID = req.BankAccountID
	receipt.Memo = req.Memo
	receipt.Notes = req.Notes

	for _, app := range req.Applications {

		invoice, err := s.invoiceRepo.GetByID(ctx, app.InvoiceID)
		if err != nil {
			return nil, fmt.Errorf("invoice %s not found", app.InvoiceID)
		}

		if invoice.CustomerID != req.CustomerID {
			return nil, fmt.Errorf("invoice %s does not belong to this customer", invoice.InvoiceNumber)
		}

		appAmount := money.New(app.Amount, currency.Code)
		appDiscount := money.New(app.DiscountTaken, currency.Code)

		if appAmount.GreaterThan(invoice.BalanceDue) {
			return nil, fmt.Errorf("application amount exceeds invoice %s balance due", invoice.InvoiceNumber)
		}

		if err := receipt.ApplyToInvoice(invoice.ID, appAmount, appDiscount); err != nil {
			return nil, fmt.Errorf("failed to apply to invoice: %w", err)
		}
	}

	if err := receipt.Validate(); err != nil {
		return nil, err
	}

	if err := s.receiptRepo.Create(ctx, receipt); err != nil {
		return nil, fmt.Errorf("failed to save receipt: %w", err)
	}

	s.auditLogger.Log(ctx, "ar_receipt", receipt.ID, "receipt.created", map[string]any{
		"receipt_number": receipt.ReceiptNumber,
		"customer_id":    customer.ID.String(),
		"amount":         receipt.Amount.Amount.String(),
		"applications":   len(receipt.Applications),
	})

	s.logger.Info("Receipt created",
		zap.String("receipt_id", receipt.ID.String()),
		zap.String("receipt_number", receipt.ReceiptNumber),
		zap.String("customer_id", customer.ID.String()),
	)

	return receipt, nil
}

func (s *ReceiptService) GetReceipt(ctx context.Context, id common.ID) (*domain.Receipt, error) {
	receipt, err := s.receiptRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	customer, err := s.customerRepo.GetByID(ctx, receipt.CustomerID)
	if err == nil {
		receipt.Customer = customer
	}

	for i := range receipt.Applications {
		invoice, err := s.invoiceRepo.GetByID(ctx, receipt.Applications[i].InvoiceID)
		if err == nil {
			receipt.Applications[i].Invoice = invoice
		}
	}

	return receipt, nil
}

func (s *ReceiptService) ListReceipts(ctx context.Context, filter domain.ReceiptFilter) ([]domain.Receipt, int, error) {
	receipts, err := s.receiptRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.receiptRepo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return receipts, count, nil
}

func (s *ReceiptService) ConfirmReceipt(ctx context.Context, id common.ID) error {
	receipt, err := s.receiptRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("receipt not found: %w", err)
	}

	customer, err := s.customerRepo.GetByID(ctx, receipt.CustomerID)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	if err := receipt.Confirm(); err != nil {
		return err
	}

	if err := s.glAPI.ValidatePeriodOpen(ctx, receipt.EntityID, receipt.ReceiptDate); err != nil {
		return fmt.Errorf("cannot post to closed period: %w", err)
	}

	journalEntry, err := s.postReceiptToGL(ctx, receipt, customer)
	if err != nil {
		return fmt.Errorf("failed to post to GL: %w", err)
	}

	receipt.JournalEntryID = &journalEntry.ID

	for _, app := range receipt.Applications {
		invoice, err := s.invoiceRepo.GetByID(ctx, app.InvoiceID)
		if err != nil {
			continue
		}

		if err := invoice.ApplyPayment(app.Amount); err != nil {
			s.logger.Error("Failed to apply payment to invoice",
				zap.String("invoice_id", invoice.ID.String()),
				zap.Error(err),
			)
			continue
		}

		if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
			s.logger.Error("Failed to update invoice after payment",
				zap.String("invoice_id", invoice.ID.String()),
				zap.Error(err),
			)
		}
	}

	if receipt.IsFullyApplied() {
		receipt.MarkApplied()
	}

	if err := s.receiptRepo.Update(ctx, receipt); err != nil {
		return fmt.Errorf("failed to update receipt: %w", err)
	}

	newBalance := customer.CurrentBalance.MustSubtract(receipt.AppliedAmount)
	customer.UpdateBalance(newBalance)

	if customer.OnCreditHold && !customer.ShouldTriggerCreditHold() {

	}

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		s.logger.Error("Failed to update customer balance", zap.Error(err))
	}

	s.auditLogger.Log(ctx, "ar_receipt", receipt.ID, "receipt.confirmed", map[string]any{
		"journal_entry_id": journalEntry.ID.String(),
	})

	s.logger.Info("Receipt confirmed and posted",
		zap.String("receipt_id", id.String()),
		zap.String("journal_entry_id", journalEntry.ID.String()),
	)

	return nil
}

func (s *ReceiptService) postReceiptToGL(ctx context.Context, receipt *domain.Receipt, customer *domain.Customer) (*gl.JournalEntryResponse, error) {

	lines := make([]gl.JournalLineRequest, 0, 2)

	if receipt.BankAccountID == nil {
		return nil, fmt.Errorf("receipt has no bank account configured")
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *receipt.BankAccountID,
		Description: fmt.Sprintf("Receipt from %s - %s", customer.Name, receipt.ReceiptNumber),
		Debit:       receipt.Amount,
		Credit:      money.Zero(receipt.Currency),
	})

	arAccountID := customer.ARAccountID
	if arAccountID == nil {
		return nil, fmt.Errorf("customer has no AR account configured")
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *arAccountID,
		Description: fmt.Sprintf("AR Receipt - %s - %s", customer.Name, receipt.ReceiptNumber),
		Debit:       money.Zero(receipt.Currency),
		Credit:      receipt.Amount,
	})

	req := gl.CreateJournalEntryRequest{
		EntityID:     receipt.EntityID,
		EntryDate:    receipt.ReceiptDate,
		Description:  fmt.Sprintf("AR Receipt %s from %s", receipt.ReceiptNumber, customer.Name),
		CurrencyCode: receipt.Currency.Code,
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

func (s *ReceiptService) ReverseReceipt(ctx context.Context, id common.ID, reason string, reversedBy common.ID) error {
	receipt, err := s.receiptRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("receipt not found: %w", err)
	}

	if receipt.JournalEntryID != nil {
		reversalEntry, err := s.glAPI.ReverseJournalEntry(ctx, *receipt.JournalEntryID, time.Now())
		if err != nil {
			return fmt.Errorf("failed to reverse GL entry: %w", err)
		}
		receipt.ReversalEntryID = &reversalEntry.ID
	}

	for _, app := range receipt.Applications {
		invoice, err := s.invoiceRepo.GetByID(ctx, app.InvoiceID)
		if err != nil {
			continue
		}

		if err := invoice.UnapplyPayment(app.Amount); err != nil {
			s.logger.Error("Failed to unapply payment from invoice",
				zap.String("invoice_id", invoice.ID.String()),
				zap.Error(err),
			)
			continue
		}

		s.invoiceRepo.Update(ctx, invoice)
	}

	if err := receipt.Reverse(reason, reversedBy); err != nil {
		return err
	}

	if err := s.receiptRepo.Update(ctx, receipt); err != nil {
		return fmt.Errorf("failed to reverse receipt: %w", err)
	}

	customer, err := s.customerRepo.GetByID(ctx, receipt.CustomerID)
	if err == nil {
		newBalance := customer.CurrentBalance.MustAdd(receipt.AppliedAmount)
		customer.UpdateBalance(newBalance)
		s.customerRepo.Update(ctx, customer)
	}

	s.auditLogger.Log(ctx, "ar_receipt", receipt.ID, "receipt.reversed", map[string]any{
		"reason":      reason,
		"reversed_by": reversedBy.String(),
	})

	s.logger.Info("Receipt reversed",
		zap.String("receipt_id", id.String()),
		zap.String("reason", reason),
	)

	return nil
}

func (s *ReceiptService) VoidReceipt(ctx context.Context, id common.ID) error {
	receipt, err := s.receiptRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("receipt not found: %w", err)
	}

	if err := receipt.Void(); err != nil {
		return err
	}

	if err := s.receiptRepo.Update(ctx, receipt); err != nil {
		return fmt.Errorf("failed to void receipt: %w", err)
	}

	s.logger.Info("Receipt voided",
		zap.String("receipt_id", id.String()),
	)

	return nil
}

func (s *ReceiptService) AddApplication(ctx context.Context, receiptID common.ID, invoiceID common.ID, amount float64, discount float64) error {
	receipt, err := s.receiptRepo.GetByID(ctx, receiptID)
	if err != nil {
		return fmt.Errorf("receipt not found: %w", err)
	}

	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if invoice.CustomerID != receipt.CustomerID {
		return fmt.Errorf("invoice does not belong to receipt's customer")
	}

	amountMoney := money.New(amount, receipt.Currency.Code)
	discountMoney := money.New(discount, receipt.Currency.Code)

	if err := receipt.ApplyToInvoice(invoiceID, amountMoney, discountMoney); err != nil {
		return err
	}

	if err := s.receiptRepo.Update(ctx, receipt); err != nil {
		return fmt.Errorf("failed to update receipt: %w", err)
	}

	return nil
}

func (s *ReceiptService) RemoveApplication(ctx context.Context, receiptID common.ID, invoiceID common.ID) error {
	receipt, err := s.receiptRepo.GetByID(ctx, receiptID)
	if err != nil {
		return fmt.Errorf("receipt not found: %w", err)
	}

	if err := receipt.RemoveApplication(invoiceID); err != nil {
		return err
	}

	if err := s.receiptRepo.Update(ctx, receipt); err != nil {
		return fmt.Errorf("failed to update receipt: %w", err)
	}

	return nil
}

func (s *ReceiptService) GetUnappliedReceipts(ctx context.Context, entityID common.ID) ([]domain.Receipt, error) {
	return s.receiptRepo.GetUnappliedReceipts(ctx, entityID)
}

func (s *ReceiptService) GetUndepositedReceipts(ctx context.Context, entityID common.ID) ([]domain.Receipt, error) {
	return s.receiptRepo.GetUndepositedReceipts(ctx, entityID)
}

func (s *ReceiptService) GetReceiptsSummary(ctx context.Context, entityID common.ID, startDate, endDate time.Time) (*domain.ReceiptsSummary, error) {
	return s.receiptRepo.GetReceiptsSummary(ctx, entityID, startDate, endDate)
}
