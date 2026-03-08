package service

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
	"converge-finance.com/m/internal/modules/ap/internal/repository"
	"go.uber.org/zap"
)

type AgingService struct {
	invoiceRepo repository.InvoiceRepository
	vendorRepo  repository.VendorRepository
	paymentRepo repository.PaymentRepository
	logger      *zap.Logger
}

func NewAgingService(
	invoiceRepo repository.InvoiceRepository,
	vendorRepo repository.VendorRepository,
	paymentRepo repository.PaymentRepository,
	logger *zap.Logger,
) *AgingService {
	return &AgingService{
		invoiceRepo: invoiceRepo,
		vendorRepo:  vendorRepo,
		paymentRepo: paymentRepo,
		logger:      logger,
	}
}

func (s *AgingService) GetAgingReport(ctx context.Context, entityID common.ID, asOfDate time.Time) (*domain.AgingReport, error) {
	return s.invoiceRepo.GetAgingReport(ctx, entityID, asOfDate)
}

func (s *AgingService) GetVendorAgingReport(ctx context.Context, vendorID common.ID, asOfDate time.Time) (*domain.VendorAging, error) {
	return s.invoiceRepo.GetVendorAgingReport(ctx, vendorID, asOfDate)
}

func (s *AgingService) GetCashRequirements(ctx context.Context, entityID common.ID, asOfDate time.Time, weeksAhead int) (*domain.CashRequirements, error) {
	currency := money.MustGetCurrency("USD")

	req := &domain.CashRequirements{
		EntityID:    entityID,
		Currency:    currency,
		AsOfDate:    asOfDate,
		GeneratedAt: time.Now(),
		Projections: make([]domain.CashProjection, 0, weeksAhead),
	}

	overdueInvoices, err := s.invoiceRepo.GetOverdueInvoices(ctx, entityID, asOfDate)
	if err != nil {
		return nil, err
	}

	overdueAmount := money.Zero(currency)
	for _, inv := range overdueInvoices {
		overdueAmount = overdueAmount.MustAdd(inv.BalanceDue)
	}
	req.OverdueAmount = overdueAmount
	req.OverdueCount = len(overdueInvoices)

	thisWeekEnd := asOfDate.AddDate(0, 0, 7-int(asOfDate.Weekday()))
	thisWeekInvoices, err := s.invoiceRepo.GetInvoicesDueInRange(ctx, entityID, asOfDate, thisWeekEnd)
	if err != nil {
		return nil, err
	}

	thisWeekAmount := money.Zero(currency)
	for _, inv := range thisWeekInvoices {
		thisWeekAmount = thisWeekAmount.MustAdd(inv.BalanceDue)
	}
	req.DueThisWeek = thisWeekAmount
	req.DueThisWeekCount = len(thisWeekInvoices)

	nextWeekStart := thisWeekEnd.AddDate(0, 0, 1)
	nextWeekEnd := nextWeekStart.AddDate(0, 0, 6)
	nextWeekInvoices, err := s.invoiceRepo.GetInvoicesDueInRange(ctx, entityID, nextWeekStart, nextWeekEnd)
	if err != nil {
		return nil, err
	}

	nextWeekAmount := money.Zero(currency)
	for _, inv := range nextWeekInvoices {
		nextWeekAmount = nextWeekAmount.MustAdd(inv.BalanceDue)
	}
	req.DueNextWeek = nextWeekAmount
	req.DueNextWeekCount = len(nextWeekInvoices)

	monthEnd := time.Date(asOfDate.Year(), asOfDate.Month()+1, 0, 23, 59, 59, 0, asOfDate.Location())
	monthInvoices, err := s.invoiceRepo.GetInvoicesDueInRange(ctx, entityID, asOfDate, monthEnd)
	if err != nil {
		return nil, err
	}

	monthAmount := money.Zero(currency)
	for _, inv := range monthInvoices {
		monthAmount = monthAmount.MustAdd(inv.BalanceDue)
	}
	req.DueThisMonth = monthAmount
	req.DueThisMonthCount = len(monthInvoices)

	scheduledPayments, err := s.paymentRepo.GetScheduledPayments(ctx, entityID, monthEnd)
	if err != nil {
		return nil, err
	}

	scheduledAmount := money.Zero(currency)
	for _, pay := range scheduledPayments {
		scheduledAmount = scheduledAmount.MustAdd(pay.Amount)
	}
	req.ScheduledAmount = scheduledAmount
	req.ScheduledCount = len(scheduledPayments)

	discountInvoices, err := s.invoiceRepo.GetInvoicesWithEarlyPaymentDiscount(ctx, entityID, asOfDate)
	if err != nil {
		return nil, err
	}

	potentialDiscounts := money.Zero(currency)
	for _, inv := range discountInvoices {
		savings := inv.BalanceDue.MustSubtract(inv.EarlyPaymentAmount())
		potentialDiscounts = potentialDiscounts.MustAdd(savings)
	}
	req.PotentialDiscounts = potentialDiscounts

	cumulative := overdueAmount
	for week := 0; week < weeksAhead; week++ {
		weekStart := asOfDate.AddDate(0, 0, week*7)
		weekEnd := weekStart.AddDate(0, 0, 6)

		weekInvoices, err := s.invoiceRepo.GetInvoicesDueInRange(ctx, entityID, weekStart, weekEnd)
		if err != nil {
			continue
		}

		weekAmount := money.Zero(currency)
		for _, inv := range weekInvoices {
			weekAmount = weekAmount.MustAdd(inv.BalanceDue)
		}

		cumulative = cumulative.MustAdd(weekAmount)

		req.Projections = append(req.Projections, domain.CashProjection{
			PeriodStart: weekStart,
			PeriodEnd:   weekEnd,
			Label:       weekStart.Format("Jan 2") + " - " + weekEnd.Format("Jan 2"),
			DueAmount:   weekAmount,
			DueCount:    len(weekInvoices),
			Cumulative:  cumulative,
		})
	}

	return req, nil
}

func (s *AgingService) GetVendorPaymentHistory(ctx context.Context, vendorID common.ID, limit, offset int) ([]domain.Payment, int, error) {
	payments, err := s.paymentRepo.GetPaymentsForVendor(ctx, vendorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	filter := domain.PaymentFilter{
		VendorID: &vendorID,
	}
	count, err := s.paymentRepo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return payments, count, nil
}

type VendorStatistics struct {
	VendorID           common.ID
	TotalInvoices      int
	TotalPaid          money.Money
	TotalOutstanding   money.Money
	TotalOverdue       money.Money
	AveragePaymentDays float64
	InvoicesThisYear   int
	PaymentsThisYear   int
	YTDSpend           money.Money
	DiscountsTaken     money.Money
	DiscountsMissed    money.Money
}

func (s *AgingService) GetVendorStatistics(ctx context.Context, vendorID common.ID) (*VendorStatistics, error) {
	vendor, err := s.vendorRepo.GetByID(ctx, vendorID)
	if err != nil {
		return nil, err
	}

	balance, err := s.vendorRepo.GetBalance(ctx, vendorID)
	if err != nil {
		return nil, err
	}

	aging, err := s.invoiceRepo.GetVendorAgingReport(ctx, vendorID, time.Now())
	if err != nil {
		return nil, err
	}

	yearStart := time.Date(time.Now().Year(), 1, 1, 0, 0, 0, 0, time.Local)
	yearEnd := time.Date(time.Now().Year(), 12, 31, 23, 59, 59, 0, time.Local)

	payments, err := s.paymentRepo.GetPaymentsByDateRange(ctx, vendor.EntityID, yearStart, yearEnd)
	if err != nil {
		return nil, err
	}

	ytdSpend := money.Zero(vendor.Currency)
	paymentsThisYear := 0
	for _, pay := range payments {
		if pay.VendorID == vendorID && pay.Status == domain.PaymentStatusCompleted {
			ytdSpend = ytdSpend.MustAdd(pay.Amount)
			paymentsThisYear++
		}
	}

	overdueAmount := money.Zero(vendor.Currency)
	if aging != nil {
		overdueAmount = aging.Days1To30.MustAdd(aging.Days31To60).MustAdd(aging.Days61To90).MustAdd(aging.Over90Days)
	}

	stats := &VendorStatistics{
		VendorID:         vendorID,
		TotalOutstanding: balance.CurrentBalance,
		TotalOverdue:     overdueAmount,
		PaymentsThisYear: paymentsThisYear,
		YTDSpend:         ytdSpend,
	}

	if aging != nil {
		stats.TotalInvoices = aging.InvoiceCount
	}

	return stats, nil
}

type OverdueAlert struct {
	VendorID      common.ID
	VendorCode    string
	VendorName    string
	OverdueAmount money.Money
	OldestInvoice *time.Time
	DaysOverdue   int
	Priority      string
}

func (s *AgingService) GetOverdueAlerts(ctx context.Context, entityID common.ID) ([]OverdueAlert, error) {
	agingReport, err := s.invoiceRepo.GetAgingReport(ctx, entityID, time.Now())
	if err != nil {
		return nil, err
	}

	alerts := make([]OverdueAlert, 0)

	for _, va := range agingReport.VendorAgings {
		overdue := va.Days1To30.MustAdd(va.Days31To60).MustAdd(va.Days61To90).MustAdd(va.Over90Days)
		if overdue.IsZero() {
			continue
		}

		priority := "low"
		if !va.Over90Days.IsZero() {
			priority = "high"
		} else if !va.Days61To90.IsZero() {
			priority = "medium"
		}

		daysOverdue := 0
		if va.OldestDue != nil {
			daysOverdue = int(time.Since(*va.OldestDue).Hours() / 24)
		}

		alerts = append(alerts, OverdueAlert{
			VendorID:      va.VendorID,
			VendorCode:    va.VendorCode,
			VendorName:    va.VendorName,
			OverdueAmount: overdue,
			OldestInvoice: va.OldestDue,
			DaysOverdue:   daysOverdue,
			Priority:      priority,
		})
	}

	return alerts, nil
}
