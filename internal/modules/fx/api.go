package fx

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type API interface {
	Convert(ctx context.Context, req ConvertRequest) (*ConvertResponse, error)

	FindConversionPath(ctx context.Context, entityID common.ID, from, to money.Currency, date time.Time, rateType money.RateType) (*ConversionPathResponse, error)

	GetConfig(ctx context.Context, entityID common.ID) (*TriangulationConfigResponse, error)

	GetCurrencyPairConfig(ctx context.Context, entityID common.ID, from, to string) (*CurrencyPairConfigResponse, error)

	GetRevaluationRun(ctx context.Context, id common.ID) (*RevaluationRunResponse, error)

	ListRevaluationRuns(ctx context.Context, req ListRevaluationRunsRequest) (*ListRevaluationRunsResponse, error)

	ApproveRevaluation(ctx context.Context, runID, approverID common.ID) (*RevaluationRunResponse, error)
}

type ConvertRequest struct {
	EntityID      common.ID
	Amount        money.Money
	ToCurrency    money.Currency
	Date          time.Time
	RateType      money.RateType
	ReferenceType string
	ReferenceID   common.ID
	CreatedBy     common.ID
}

type ConvertResponse struct {
	FromCurrency   money.Currency
	ToCurrency     money.Currency
	OriginalAmount money.Money
	ResultAmount   money.Money
	EffectiveRate  decimal.Decimal
	Legs           []ConversionLeg
	LegsCount      int
	Method         string
	ConversionDate time.Time
	RateType       money.RateType
}

type ConversionLeg struct {
	FromCurrency string
	ToCurrency   string
	Rate         decimal.Decimal
	RateType     string
	RateDate     time.Time
}

type ConversionPathResponse struct {
	Currencies    []string
	Rates         []decimal.Decimal
	EffectiveRate decimal.Decimal
	LegsCount     int
}

type TriangulationConfigResponse struct {
	ID                 common.ID
	EntityID           common.ID
	BaseCurrency       string
	FallbackCurrencies []string
	MaxLegs            int
	AllowInverseRates  bool
	RateTolerance      decimal.Decimal
	IsActive           bool
}

type CurrencyPairConfigResponse struct {
	ID              common.ID
	EntityID        common.ID
	FromCurrency    string
	ToCurrency      string
	PreferredMethod string
	ViaCurrency     *string
	SpreadMarkup    decimal.Decimal
	Priority        int
	IsActive        bool
}

type RevaluationRunResponse struct {
	ID                  common.ID
	EntityID            common.ID
	RunNumber           string
	FiscalPeriodID      common.ID
	RevaluationDate     time.Time
	RateDate            time.Time
	FunctionalCurrency  string
	Status              string
	TotalUnrealizedGain decimal.Decimal
	TotalUnrealizedLoss decimal.Decimal
	NetRevaluation      decimal.Decimal
	AccountsProcessed   int
	JournalEntryID      *common.ID
	CreatedBy           common.ID
	ApprovedBy          *common.ID
	PostedBy            *common.ID
	CreatedAt           time.Time
	ApprovedAt          *time.Time
	PostedAt            *time.Time
	Details             []RevaluationDetailResponse
}

type RevaluationDetailResponse struct {
	ID                       common.ID
	AccountID                common.ID
	AccountCode              string
	AccountName              string
	OriginalCurrency         string
	OriginalBalance          decimal.Decimal
	OriginalRate             decimal.Decimal
	OriginalFunctionalAmount decimal.Decimal
	NewRate                  decimal.Decimal
	NewFunctionalAmount      decimal.Decimal
	RevaluationAmount        decimal.Decimal
	IsGain                   bool
	GainLossAccountID        common.ID
}

type ListRevaluationRunsRequest struct {
	EntityID       common.ID
	FiscalPeriodID common.ID
	Status         string
	DateFrom       *time.Time
	DateTo         *time.Time
	Page           int
	PageSize       int
}

type ListRevaluationRunsResponse struct {
	Runs  []RevaluationRunResponse
	Total int
	Page  int
}
