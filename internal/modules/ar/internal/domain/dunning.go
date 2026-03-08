package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type DunningLevel int

const (
	DunningLevelNone     DunningLevel = 0
	DunningLevelReminder DunningLevel = 1
	DunningLevelFirst    DunningLevel = 2
	DunningLevelSecond   DunningLevel = 3
	DunningLevelFinal    DunningLevel = 4
	DunningLevelLegal    DunningLevel = 5
)

func (d DunningLevel) String() string {
	switch d {
	case DunningLevelNone:
		return "none"
	case DunningLevelReminder:
		return "reminder"
	case DunningLevelFirst:
		return "first_notice"
	case DunningLevelSecond:
		return "second_notice"
	case DunningLevelFinal:
		return "final_notice"
	case DunningLevelLegal:
		return "legal_collections"
	default:
		return "unknown"
	}
}

type DunningAction string

const (
	DunningActionEmail        DunningAction = "email"
	DunningActionLetter       DunningAction = "letter"
	DunningActionPhone        DunningAction = "phone"
	DunningActionCreditHold   DunningAction = "credit_hold"
	DunningActionSendToAgency DunningAction = "collection_agency"
	DunningActionWriteOff     DunningAction = "write_off"
)

type DunningProfile struct {
	ID          common.ID
	EntityID    common.ID
	Name        string
	Description string
	IsDefault   bool

	Levels []DunningLevelConfig

	GracePeriodDays int

	AutoCreditHoldDays    int
	AutoCollectionsDays   int
	AutoWriteOffDays      int
	AutoWriteOffThreshold float64

	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DunningLevelConfig struct {
	Level            DunningLevel
	DaysAfterDue     int
	Actions          []DunningAction
	EmailTemplateID  *common.ID
	LetterTemplateID *common.ID
	ChargeLateFee    bool
	LateFeeAmount    float64
	LateFeePercent   float64
}

type DunningRun struct {
	ID        common.ID
	EntityID  common.ID
	ProfileID common.ID
	RunDate   time.Time
	Status    string

	CustomersProcessed int
	InvoicesProcessed  int
	EmailsSent         int
	LettersPrinted     int
	CreditHoldsApplied int
	LateFeesCharged    money.Money

	ErrorCount int
	Errors     []DunningError

	StartedAt   time.Time
	CompletedAt *time.Time
	CreatedBy   common.ID
}

type DunningError struct {
	CustomerID   common.ID
	InvoiceID    *common.ID
	Action       DunningAction
	ErrorMessage string
	OccurredAt   time.Time
}

type DunningHistory struct {
	ID         common.ID
	EntityID   common.ID
	CustomerID common.ID
	InvoiceID  common.ID
	RunID      *common.ID

	Level      DunningLevel
	Action     DunningAction
	ActionDate time.Time

	EmailSent    bool
	EmailAddress string
	EmailSentAt  *time.Time
	LetterSent   bool
	LetterSentAt *time.Time

	PhoneCalled   bool
	PhoneNumber   string
	PhoneCalledAt *time.Time
	PhoneNotes    string

	LateFeeCharged money.Money

	ResponseReceived bool
	ResponseDate     *time.Time
	ResponseNotes    string
	PromisedAmount   money.Money
	PromisedDate     *time.Time

	Notes     string
	CreatedBy common.ID
	CreatedAt time.Time
}

type CustomerAging struct {
	CustomerID   common.ID
	CustomerCode string
	CustomerName string
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

	CustomerAgings []CustomerAging

	TotalCustomers       int
	CustomersWithBalance int
	CustomersOverdue     int

	GeneratedAt time.Time
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
		CustomerAgings:  make([]CustomerAging, 0),
		GeneratedAt:     time.Now(),
	}
}

func (r *AgingReport) AddCustomerAging(ca CustomerAging) {
	r.CustomerAgings = append(r.CustomerAgings, ca)

	r.TotalCurrent = r.TotalCurrent.MustAdd(ca.Current)
	r.TotalDays1To30 = r.TotalDays1To30.MustAdd(ca.Days1To30)
	r.TotalDays31To60 = r.TotalDays31To60.MustAdd(ca.Days31To60)
	r.TotalDays61To90 = r.TotalDays61To90.MustAdd(ca.Days61To90)
	r.TotalOver90Days = r.TotalOver90Days.MustAdd(ca.Over90Days)
	r.GrandTotal = r.GrandTotal.MustAdd(ca.TotalBalance)

	r.TotalCustomers++
	if !ca.TotalBalance.IsZero() {
		r.CustomersWithBalance++
	}
	if !ca.Days1To30.IsZero() || !ca.Days31To60.IsZero() ||
		!ca.Days61To90.IsZero() || !ca.Over90Days.IsZero() {
		r.CustomersOverdue++
	}
}

type CollectionCase struct {
	ID         common.ID
	EntityID   common.ID
	CustomerID common.ID
	Customer   *Customer

	CaseNumber    string
	AgencyName    string
	AgencyContact string
	AgencyPhone   string
	AgencyEmail   string

	Currency        money.Currency
	OriginalAmount  money.Money
	CurrentAmount   money.Money
	RecoveredAmount money.Money
	AgencyFees      money.Money

	Status       string
	OpenedDate   time.Time
	ClosedDate   *time.Time
	ClosedReason string

	InvoiceIDs []common.ID

	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy common.ID
}

type CreditMemoRequest struct {
	CustomerID        common.ID
	OriginalInvoiceID *common.ID
	Reason            string
	Amount            money.Money
	Lines             []CreditMemoLine
}

type CreditMemoLine struct {
	RevenueAccountID common.ID
	Description      string
	Quantity         float64
	UnitPrice        money.Money
	TaxCode          string
	TaxAmount        money.Money
}

type ReceiptsSummary struct {
	EntityID  common.ID
	StartDate time.Time
	EndDate   time.Time
	Currency  money.Currency

	TotalReceipts  int
	ConfirmedCount int
	AppliedCount   int
	ReversedCount  int
	PendingCount   int

	TotalAmount     money.Money
	ConfirmedAmount money.Money
	AppliedAmount   money.Money
	UnappliedAmount money.Money
	ReversedAmount  money.Money

	CashAmount   money.Money
	CheckAmount  money.Money
	ACHAmount    money.Money
	WireAmount   money.Money
	CardAmount   money.Money
	OnlineAmount money.Money

	TotalDiscounts money.Money

	GeneratedAt time.Time
}

type CashForecast struct {
	EntityID common.ID
	Currency money.Currency
	AsOfDate time.Time

	OverdueAmount money.Money
	OverdueCount  int

	ExpectedThisWeek  money.Money
	ExpectedNextWeek  money.Money
	ExpectedThisMonth money.Money
	ExpectedNextMonth money.Money

	Projections []CashProjection

	GeneratedAt time.Time
}

type CashProjection struct {
	PeriodStart    time.Time
	PeriodEnd      time.Time
	Label          string
	ExpectedAmount money.Money
	InvoiceCount   int
	Cumulative     money.Money
}
