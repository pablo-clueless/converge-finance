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

type PaymentService struct {
	paymentRepo repository.PaymentRepository
	invoiceRepo repository.InvoiceRepository
	vendorRepo  repository.VendorRepository
	glAPI       gl.API
	auditLogger *audit.Logger
	logger      *zap.Logger
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	invoiceRepo repository.InvoiceRepository,
	vendorRepo repository.VendorRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *PaymentService {
	return &PaymentService{
		paymentRepo: paymentRepo,
		invoiceRepo: invoiceRepo,
		vendorRepo:  vendorRepo,
		glAPI:       glAPI,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

type CreatePaymentRequest struct {
	EntityID      common.ID
	VendorID      common.ID
	PaymentDate   time.Time
	PaymentMethod domain.PaymentMethod
	Currency      string
	BankAccountID *common.ID
	Memo          string
	Notes         string
	Allocations   []PaymentAllocationRequest
	CreatedBy     common.ID
}

type PaymentAllocationRequest struct {
	InvoiceID     common.ID
	Amount        float64
	DiscountTaken float64
}

func (s *PaymentService) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*domain.Payment, error) {

	vendor, err := s.vendorRepo.GetByID(ctx, req.VendorID)
	if err != nil {
		return nil, fmt.Errorf("vendor not found: %w", err)
	}

	if !vendor.CanReceivePayments() {
		return nil, fmt.Errorf("vendor is not eligible to receive payments")
	}

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}

	paymentNumber, err := s.paymentRepo.GetNextPaymentNumber(ctx, req.EntityID, "PAY")
	if err != nil {
		return nil, fmt.Errorf("failed to generate payment number: %w", err)
	}

	payment, err := domain.NewPayment(
		req.EntityID,
		req.VendorID,
		paymentNumber,
		req.PaymentDate,
		req.PaymentMethod,
		currency,
		req.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	payment.BankAccountID = req.BankAccountID
	payment.Memo = req.Memo
	payment.Notes = req.Notes

	for _, alloc := range req.Allocations {

		invoice, err := s.invoiceRepo.GetByID(ctx, alloc.InvoiceID)
		if err != nil {
			return nil, fmt.Errorf("invoice %s not found", alloc.InvoiceID)
		}

		if invoice.Status != domain.InvoiceStatusApproved && invoice.Status != domain.InvoiceStatusPartialPaid {
			return nil, fmt.Errorf("invoice %s is not ready for payment", invoice.InvoiceNumber)
		}

		if invoice.VendorID != req.VendorID {
			return nil, fmt.Errorf("invoice %s does not belong to this vendor", invoice.InvoiceNumber)
		}

		amount := money.New(alloc.Amount, currency.Code)
		discount := money.New(alloc.DiscountTaken, currency.Code)

		if amount.GreaterThan(invoice.BalanceDue) {
			return nil, fmt.Errorf("payment amount exceeds invoice %s balance due", invoice.InvoiceNumber)
		}

		if err := payment.AllocateToInvoice(invoice.ID, amount, discount); err != nil {
			return nil, fmt.Errorf("failed to allocate to invoice: %w", err)
		}
	}

	if err := payment.Validate(); err != nil {
		return nil, err
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to save payment: %w", err)
	}

	s.auditLogger.Log(ctx, "ap_payment", payment.ID, "payment.created", map[string]any{
		"payment_number": payment.PaymentNumber,
		"vendor_id":      vendor.ID.String(),
		"amount":         payment.Amount.Amount.String(),
		"allocations":    len(payment.Allocations),
	})

	s.logger.Info("AP Payment created",
		zap.String("payment_id", payment.ID.String()),
		zap.String("payment_number", payment.PaymentNumber),
		zap.String("vendor_id", vendor.ID.String()),
	)

	return payment, nil
}

func (s *PaymentService) GetPayment(ctx context.Context, id common.ID) (*domain.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	vendor, err := s.vendorRepo.GetByID(ctx, payment.VendorID)
	if err == nil {
		payment.Vendor = vendor
	}

	for i := range payment.Allocations {
		invoice, err := s.invoiceRepo.GetByID(ctx, payment.Allocations[i].InvoiceID)
		if err == nil {
			payment.Allocations[i].Invoice = invoice
		}
	}

	return payment, nil
}

func (s *PaymentService) ListPayments(ctx context.Context, filter domain.PaymentFilter) ([]domain.Payment, int, error) {
	payments, err := s.paymentRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.paymentRepo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return payments, count, nil
}

func (s *PaymentService) SubmitPayment(ctx context.Context, id common.ID) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if err := payment.Submit(); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to submit payment: %w", err)
	}

	s.logger.Info("Payment submitted for approval",
		zap.String("payment_id", id.String()),
	)

	return nil
}

func (s *PaymentService) ApprovePayment(ctx context.Context, id common.ID, approvedBy common.ID) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if err := payment.Approve(approvedBy); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to approve payment: %w", err)
	}

	s.auditLogger.Log(ctx, "ap_payment", payment.ID, "payment.approved", map[string]any{
		"approved_by": approvedBy.String(),
	})

	s.logger.Info("Payment approved",
		zap.String("payment_id", id.String()),
	)

	return nil
}

func (s *PaymentService) RejectPayment(ctx context.Context, id common.ID) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if err := payment.Reject(); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to reject payment: %w", err)
	}

	s.logger.Info("Payment rejected",
		zap.String("payment_id", id.String()),
	)

	return nil
}

func (s *PaymentService) SchedulePayment(ctx context.Context, id common.ID, scheduledDate time.Time) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if err := payment.Schedule(scheduledDate); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to schedule payment: %w", err)
	}

	s.logger.Info("Payment scheduled",
		zap.String("payment_id", id.String()),
		zap.Time("scheduled_date", scheduledDate),
	)

	return nil
}

func (s *PaymentService) ProcessPayment(ctx context.Context, id common.ID) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	vendor, err := s.vendorRepo.GetByID(ctx, payment.VendorID)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	if err := payment.Process(); err != nil {
		return err
	}

	if err := s.glAPI.ValidatePeriodOpen(ctx, payment.EntityID, payment.PaymentDate); err != nil {
		return fmt.Errorf("cannot post to closed period: %w", err)
	}

	journalEntry, err := s.postPaymentToGL(ctx, payment, vendor)
	if err != nil {
		return fmt.Errorf("failed to post to GL: %w", err)
	}

	payment.JournalEntryID = &journalEntry.ID

	for _, alloc := range payment.Allocations {
		invoice, err := s.invoiceRepo.GetByID(ctx, alloc.InvoiceID)
		if err != nil {
			continue
		}

		if err := invoice.ApplyPayment(alloc.Amount); err != nil {
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

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	s.auditLogger.Log(ctx, "ap_payment", payment.ID, "payment.processed", map[string]any{
		"journal_entry_id": journalEntry.ID.String(),
	})

	s.logger.Info("Payment processed",
		zap.String("payment_id", id.String()),
		zap.String("journal_entry_id", journalEntry.ID.String()),
	)

	return nil
}

func (s *PaymentService) postPaymentToGL(ctx context.Context, payment *domain.Payment, vendor *domain.Vendor) (*gl.JournalEntryResponse, error) {

	lines := make([]gl.JournalLineRequest, 0, 2)

	apAccountID := vendor.APAccountID
	if apAccountID == nil {
		return nil, fmt.Errorf("vendor has no AP account configured")
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *apAccountID,
		Description: fmt.Sprintf("AP Payment - %s - %s", vendor.Name, payment.PaymentNumber),
		Debit:       payment.Amount,
		Credit:      money.Zero(payment.Currency),
	})

	if payment.BankAccountID == nil {
		return nil, fmt.Errorf("payment has no bank account configured")
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *payment.BankAccountID,
		Description: fmt.Sprintf("Payment to %s - %s", vendor.Name, payment.PaymentNumber),
		Debit:       money.Zero(payment.Currency),
		Credit:      payment.Amount,
	})

	if !payment.DiscountTaken.IsZero() {

	}

	req := gl.CreateJournalEntryRequest{
		EntityID:     payment.EntityID,
		EntryDate:    payment.PaymentDate,
		Description:  fmt.Sprintf("AP Payment %s to %s", payment.PaymentNumber, vendor.Name),
		CurrencyCode: payment.Currency.Code,
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

func (s *PaymentService) CompletePayment(ctx context.Context, id common.ID, bankReference string) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if err := payment.Complete(bankReference); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to complete payment: %w", err)
	}

	vendor, err := s.vendorRepo.GetByID(ctx, payment.VendorID)
	if err == nil {
		newBalance := vendor.CurrentBalance.MustSubtract(payment.Amount)
		vendor.UpdateBalance(newBalance)
		s.vendorRepo.Update(ctx, vendor)
	}

	s.auditLogger.Log(ctx, "ap_payment", payment.ID, "payment.completed", map[string]any{
		"bank_reference": bankReference,
	})

	s.logger.Info("Payment completed",
		zap.String("payment_id", id.String()),
		zap.String("bank_reference", bankReference),
	)

	return nil
}

func (s *PaymentService) FailPayment(ctx context.Context, id common.ID, reason string) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if err := payment.Fail(reason); err != nil {
		return err
	}

	if payment.JournalEntryID != nil {
		_, err := s.glAPI.ReverseJournalEntry(ctx, *payment.JournalEntryID, time.Now())
		if err != nil {
			s.logger.Error("Failed to reverse GL entry for failed payment", zap.Error(err))
		}
	}

	for _, alloc := range payment.Allocations {
		invoice, err := s.invoiceRepo.GetByID(ctx, alloc.InvoiceID)
		if err != nil {
			continue
		}

		if err := invoice.UnapplyPayment(alloc.Amount); err != nil {
			s.logger.Error("Failed to unapply payment from invoice",
				zap.String("invoice_id", invoice.ID.String()),
				zap.Error(err),
			)
			continue
		}

		s.invoiceRepo.Update(ctx, invoice)
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	s.auditLogger.Log(ctx, "ap_payment", payment.ID, "payment.failed", map[string]any{
		"reason": reason,
	})

	s.logger.Info("Payment failed",
		zap.String("payment_id", id.String()),
		zap.String("reason", reason),
	)

	return nil
}

func (s *PaymentService) VoidPayment(ctx context.Context, id common.ID) error {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if payment.JournalEntryID != nil {
		_, err := s.glAPI.ReverseJournalEntry(ctx, *payment.JournalEntryID, time.Now())
		if err != nil {
			s.logger.Error("Failed to reverse GL entry for voided payment", zap.Error(err))
		}
	}

	for _, alloc := range payment.Allocations {
		invoice, err := s.invoiceRepo.GetByID(ctx, alloc.InvoiceID)
		if err != nil {
			continue
		}

		if err := invoice.UnapplyPayment(alloc.Amount); err != nil {
			s.logger.Error("Failed to unapply payment from invoice",
				zap.String("invoice_id", invoice.ID.String()),
				zap.Error(err),
			)
			continue
		}

		s.invoiceRepo.Update(ctx, invoice)
	}

	if err := payment.Void(); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to void payment: %w", err)
	}

	s.logger.Info("Payment voided",
		zap.String("payment_id", id.String()),
	)

	return nil
}

func (s *PaymentService) GetScheduledPayments(ctx context.Context, entityID common.ID, beforeDate time.Time) ([]domain.Payment, error) {
	return s.paymentRepo.GetScheduledPayments(ctx, entityID, beforeDate)
}

func (s *PaymentService) GetPaymentsSummary(ctx context.Context, entityID common.ID, startDate, endDate time.Time) (*domain.PaymentsSummary, error) {
	return s.paymentRepo.GetPaymentsSummary(ctx, entityID, startDate, endDate)
}

func (s *PaymentService) AddAllocation(ctx context.Context, paymentID common.ID, invoiceID common.ID, amount float64, discount float64) error {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if invoice.VendorID != payment.VendorID {
		return fmt.Errorf("invoice does not belong to payment's vendor")
	}

	amountMoney := money.New(amount, payment.Currency.Code)
	discountMoney := money.New(discount, payment.Currency.Code)

	if err := payment.AllocateToInvoice(invoiceID, amountMoney, discountMoney); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	return nil
}

func (s *PaymentService) RemoveAllocation(ctx context.Context, paymentID common.ID, invoiceID common.ID) error {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("payment not found: %w", err)
	}

	if err := payment.RemoveAllocation(invoiceID); err != nil {
		return err
	}

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}

	return nil
}
