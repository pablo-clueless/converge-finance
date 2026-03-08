package service

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
	"converge-finance.com/m/internal/modules/ar/internal/repository"
	"go.uber.org/zap"
)

type DunningService struct {
	invoiceRepo  repository.InvoiceRepository
	customerRepo repository.CustomerRepository
	logger       *zap.Logger
}

func NewDunningService(
	invoiceRepo repository.InvoiceRepository,
	customerRepo repository.CustomerRepository,
	logger *zap.Logger,
) *DunningService {
	return &DunningService{
		invoiceRepo:  invoiceRepo,
		customerRepo: customerRepo,
		logger:       logger,
	}
}

func (s *DunningService) GetAgingReport(ctx context.Context, entityID common.ID, asOfDate time.Time) (*domain.AgingReport, error) {
	return s.invoiceRepo.GetAgingReport(ctx, entityID, asOfDate)
}

func (s *DunningService) GetCustomerAgingReport(ctx context.Context, customerID common.ID, asOfDate time.Time) (*domain.CustomerAging, error) {
	return s.invoiceRepo.GetCustomerAgingReport(ctx, customerID, asOfDate)
}

func (s *DunningService) GetInvoicesForDunning(ctx context.Context, entityID common.ID, minDaysOverdue int) ([]domain.Invoice, error) {
	return s.invoiceRepo.GetInvoicesForDunning(ctx, entityID, minDaysOverdue)
}

func (s *DunningService) GetCashForecast(ctx context.Context, entityID common.ID, asOfDate time.Time, weeksAhead int) (*domain.CashForecast, error) {
	currency := money.MustGetCurrency("USD")

	forecast := &domain.CashForecast{
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
	forecast.OverdueAmount = overdueAmount
	forecast.OverdueCount = len(overdueInvoices)

	thisWeekEnd := asOfDate.AddDate(0, 0, 7-int(asOfDate.Weekday()))
	thisWeekInvoices, err := s.invoiceRepo.GetInvoicesDueInRange(ctx, entityID, asOfDate, thisWeekEnd)
	if err != nil {
		return nil, err
	}

	thisWeekAmount := money.Zero(currency)
	for _, inv := range thisWeekInvoices {
		thisWeekAmount = thisWeekAmount.MustAdd(inv.BalanceDue)
	}
	forecast.ExpectedThisWeek = thisWeekAmount

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
	forecast.ExpectedNextWeek = nextWeekAmount

	monthEnd := time.Date(asOfDate.Year(), asOfDate.Month()+1, 0, 23, 59, 59, 0, asOfDate.Location())
	monthInvoices, err := s.invoiceRepo.GetInvoicesDueInRange(ctx, entityID, asOfDate, monthEnd)
	if err != nil {
		return nil, err
	}

	monthAmount := money.Zero(currency)
	for _, inv := range monthInvoices {
		monthAmount = monthAmount.MustAdd(inv.BalanceDue)
	}
	forecast.ExpectedThisMonth = monthAmount

	nextMonthStart := monthEnd.AddDate(0, 0, 1)
	nextMonthEnd := nextMonthStart.AddDate(0, 1, -1)
	nextMonthInvoices, err := s.invoiceRepo.GetInvoicesDueInRange(ctx, entityID, nextMonthStart, nextMonthEnd)
	if err != nil {
		return nil, err
	}

	nextMonthAmount := money.Zero(currency)
	for _, inv := range nextMonthInvoices {
		nextMonthAmount = nextMonthAmount.MustAdd(inv.BalanceDue)
	}
	forecast.ExpectedNextMonth = nextMonthAmount

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

		forecast.Projections = append(forecast.Projections, domain.CashProjection{
			PeriodStart:    weekStart,
			PeriodEnd:      weekEnd,
			Label:          weekStart.Format("Jan 2") + " - " + weekEnd.Format("Jan 2"),
			ExpectedAmount: weekAmount,
			InvoiceCount:   len(weekInvoices),
			Cumulative:     cumulative,
		})
	}

	return forecast, nil
}

type OverdueAlert struct {
	CustomerID    common.ID
	CustomerCode  string
	CustomerName  string
	OverdueAmount money.Money
	OldestInvoice *time.Time
	DaysOverdue   int
	Priority      string
	OnCreditHold  bool
}

func (s *DunningService) GetOverdueAlerts(ctx context.Context, entityID common.ID) ([]OverdueAlert, error) {
	agingReport, err := s.invoiceRepo.GetAgingReport(ctx, entityID, time.Now())
	if err != nil {
		return nil, err
	}

	alerts := make([]OverdueAlert, 0)

	for _, ca := range agingReport.CustomerAgings {
		overdue := ca.Days1To30.MustAdd(ca.Days31To60).MustAdd(ca.Days61To90).MustAdd(ca.Over90Days)
		if overdue.IsZero() {
			continue
		}

		priority := "low"
		if !ca.Over90Days.IsZero() {
			priority = "high"
		} else if !ca.Days61To90.IsZero() {
			priority = "medium"
		}

		daysOverdue := 0
		if ca.OldestDue != nil {
			daysOverdue = int(time.Since(*ca.OldestDue).Hours() / 24)
		}

		customer, _ := s.customerRepo.GetByID(ctx, ca.CustomerID)
		onCreditHold := false
		if customer != nil {
			onCreditHold = customer.OnCreditHold
		}

		alerts = append(alerts, OverdueAlert{
			CustomerID:    ca.CustomerID,
			CustomerCode:  ca.CustomerCode,
			CustomerName:  ca.CustomerName,
			OverdueAmount: overdue,
			OldestInvoice: ca.OldestDue,
			DaysOverdue:   daysOverdue,
			Priority:      priority,
			OnCreditHold:  onCreditHold,
		})
	}

	return alerts, nil
}

type CustomerStatistics struct {
	CustomerID         common.ID
	TotalInvoices      int
	TotalReceived      money.Money
	TotalOutstanding   money.Money
	TotalOverdue       money.Money
	AveragePaymentDays float64
	InvoicesThisYear   int
	ReceiptsThisYear   int
	YTDRevenue         money.Money
	DiscountsTaken     money.Money
	DiscountsMissed    money.Money
}

func (s *DunningService) GetCustomerStatistics(ctx context.Context, customerID common.ID) (*CustomerStatistics, error) {
	customer, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	balance, err := s.customerRepo.GetBalance(ctx, customerID)
	if err != nil {
		return nil, err
	}

	aging, err := s.invoiceRepo.GetCustomerAgingReport(ctx, customerID, time.Now())
	if err != nil {
		return nil, err
	}

	overdueAmount := money.Zero(customer.Currency)
	if aging != nil {
		overdueAmount = aging.Days1To30.MustAdd(aging.Days31To60).MustAdd(aging.Days61To90).MustAdd(aging.Over90Days)
	}

	stats := &CustomerStatistics{
		CustomerID:       customerID,
		TotalOutstanding: balance.CurrentBalance,
		TotalOverdue:     overdueAmount,
	}

	if aging != nil {
		stats.TotalInvoices = aging.InvoiceCount
	}

	return stats, nil
}

func (s *DunningService) EscalateDunning(ctx context.Context, invoiceID common.ID) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return err
	}

	newLevel := domain.DunningLevel(invoice.DunningLevel + 1)
	if newLevel > domain.DunningLevelLegal {
		newLevel = domain.DunningLevelLegal
	}

	if err := s.invoiceRepo.UpdateDunningLevel(ctx, invoiceID, newLevel); err != nil {
		return err
	}

	invoice.DunningLevel = int(newLevel)
	now := time.Now()
	invoice.LastDunningDate = &now

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return err
	}

	s.logger.Info("Dunning escalated",
		zap.String("invoice_id", invoiceID.String()),
		zap.Int("new_level", int(newLevel)),
	)

	return nil
}

func (s *DunningService) GetCustomersForCreditReview(ctx context.Context, entityID common.ID) ([]domain.Customer, error) {

	overLimit, err := s.customerRepo.GetCustomersOverCreditLimit(ctx, entityID)
	if err != nil {
		return nil, err
	}

	onHold, err := s.customerRepo.GetCustomersOnCreditHold(ctx, entityID)
	if err != nil {
		return nil, err
	}

	customerMap := make(map[common.ID]domain.Customer)
	for _, c := range overLimit {
		customerMap[c.ID] = c
	}
	for _, c := range onHold {
		customerMap[c.ID] = c
	}

	result := make([]domain.Customer, 0, len(customerMap))
	for _, c := range customerMap {
		result = append(result, c)
	}

	return result, nil
}
