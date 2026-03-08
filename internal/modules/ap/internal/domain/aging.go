package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type VendorBalance struct {
	VendorID        common.ID
	TotalInvoiced   money.Money
	TotalPaid       money.Money
	CurrentBalance  money.Money
	OverdueBalance  money.Money
	AvailableCredit money.Money
	LastInvoiceDate *time.Time
	LastPaymentDate *time.Time
	UpdatedAt       time.Time
}

type AgingBucket struct {
	Label      string
	MinDays    int
	MaxDays    int
	Amount     money.Money
	Count      int
	Percentage float64
}

type VendorAging struct {
	VendorID     common.ID
	VendorCode   string
	VendorName   string
	Currency     money.Currency
	Current      money.Money
	Days1To30    money.Money
	Days31To60   money.Money
	Days61To90   money.Money
	Over90Days   money.Money
	TotalBalance money.Money
	InvoiceCount int
	OldestDue    *time.Time
}

type AgingReport struct {
	EntityID common.ID
	AsOfDate time.Time
	Currency money.Currency

	TotalCurrent    money.Money
	TotalDays1To30  money.Money
	TotalDays31To60 money.Money
	TotalDays61To90 money.Money
	TotalOver90Days money.Money
	GrandTotal      money.Money

	VendorAgings []VendorAging

	TotalVendors       int
	VendorsWithBalance int
	VendorsOverdue     int

	GeneratedAt time.Time
}

type PaymentsSummary struct {
	EntityID  common.ID
	StartDate time.Time
	EndDate   time.Time
	Currency  money.Currency

	TotalPayments   int
	DraftCount      int
	PendingCount    int
	ApprovedCount   int
	ScheduledCount  int
	ProcessingCount int
	CompletedCount  int
	FailedCount     int
	VoidCount       int

	TotalAmount     money.Money
	CompletedAmount money.Money
	PendingAmount   money.Money
	ScheduledAmount money.Money

	TotalDiscountsTaken money.Money

	CheckAmount money.Money
	ACHAmount   money.Money
	WireAmount  money.Money
	CardAmount  money.Money

	GeneratedAt time.Time
}

type CashRequirements struct {
	EntityID common.ID
	Currency money.Currency
	AsOfDate time.Time

	OverdueAmount money.Money
	OverdueCount  int

	DueThisWeek      money.Money
	DueThisWeekCount int

	DueNextWeek      money.Money
	DueNextWeekCount int

	DueThisMonth      money.Money
	DueThisMonthCount int

	ScheduledAmount money.Money
	ScheduledCount  int

	PotentialDiscounts money.Money

	Projections []CashProjection

	GeneratedAt time.Time
}

type CashProjection struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	Label       string
	DueAmount   money.Money
	DueCount    int
	Cumulative  money.Money
}

func NewAgingReport(entityID common.ID, asOfDate time.Time, currency money.Currency) *AgingReport {
	return &AgingReport{
		EntityID:        entityID,
		AsOfDate:        asOfDate,
		Currency:        currency,
		TotalCurrent:    money.Zero(currency),
		TotalDays1To30:  money.Zero(currency),
		TotalDays31To60: money.Zero(currency),
		TotalDays61To90: money.Zero(currency),
		TotalOver90Days: money.Zero(currency),
		GrandTotal:      money.Zero(currency),
		VendorAgings:    make([]VendorAging, 0),
		GeneratedAt:     time.Now(),
	}
}

func (r *AgingReport) AddVendorAging(va VendorAging) {
	r.VendorAgings = append(r.VendorAgings, va)

	r.TotalCurrent = r.TotalCurrent.MustAdd(va.Current)
	r.TotalDays1To30 = r.TotalDays1To30.MustAdd(va.Days1To30)
	r.TotalDays31To60 = r.TotalDays31To60.MustAdd(va.Days31To60)
	r.TotalDays61To90 = r.TotalDays61To90.MustAdd(va.Days61To90)
	r.TotalOver90Days = r.TotalOver90Days.MustAdd(va.Over90Days)
	r.GrandTotal = r.GrandTotal.MustAdd(va.TotalBalance)

	r.TotalVendors++
	if !va.TotalBalance.IsZero() {
		r.VendorsWithBalance++
	}
	if !va.Days1To30.IsZero() || !va.Days31To60.IsZero() ||
		!va.Days61To90.IsZero() || !va.Over90Days.IsZero() {
		r.VendorsOverdue++
	}
}

func (r *AgingReport) GetAgingBuckets() []AgingBucket {
	total := r.GrandTotal.Amount.InexactFloat64()
	if total == 0 {
		total = 1
	}

	return []AgingBucket{
		{
			Label:      "Current",
			MinDays:    0,
			MaxDays:    0,
			Amount:     r.TotalCurrent,
			Percentage: r.TotalCurrent.Amount.InexactFloat64() / total * 100,
		},
		{
			Label:      "1-30 Days",
			MinDays:    1,
			MaxDays:    30,
			Amount:     r.TotalDays1To30,
			Percentage: r.TotalDays1To30.Amount.InexactFloat64() / total * 100,
		},
		{
			Label:      "31-60 Days",
			MinDays:    31,
			MaxDays:    60,
			Amount:     r.TotalDays31To60,
			Percentage: r.TotalDays31To60.Amount.InexactFloat64() / total * 100,
		},
		{
			Label:      "61-90 Days",
			MinDays:    61,
			MaxDays:    90,
			Amount:     r.TotalDays61To90,
			Percentage: r.TotalDays61To90.Amount.InexactFloat64() / total * 100,
		},
		{
			Label:      "Over 90 Days",
			MinDays:    91,
			MaxDays:    -1,
			Amount:     r.TotalOver90Days,
			Percentage: r.TotalOver90Days.Amount.InexactFloat64() / total * 100,
		},
	}
}
