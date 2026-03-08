package domain

import (
	"encoding/json"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type TriangulationLeg struct {
	FromCurrency string          `json:"from"`
	ToCurrency   string          `json:"to"`
	Rate         decimal.Decimal `json:"rate"`
	RateType     string          `json:"rate_type"`
	RateDate     time.Time       `json:"rate_date"`
}

type TriangulationResult struct {
	FromCurrency   money.Currency
	ToCurrency     money.Currency
	OriginalAmount money.Money
	ResultAmount   money.Money
	EffectiveRate  decimal.Decimal
	Legs           []TriangulationLeg
	Method         TriangulationMethod
	ConversionDate time.Time
	RateType       money.RateType
}

func (r *TriangulationResult) LegsCount() int {
	return len(r.Legs)
}

func (r *TriangulationResult) IsDirect() bool {
	return len(r.Legs) == 1
}

func (r *TriangulationResult) LegsJSON() ([]byte, error) {
	return json.Marshal(r.Legs)
}

type TriangulationLog struct {
	ID             common.ID
	EntityID       common.ID
	FromCurrency   string
	ToCurrency     string
	OriginalAmount decimal.Decimal
	ResultAmount   decimal.Decimal
	EffectiveRate  decimal.Decimal
	Legs           []TriangulationLeg
	LegsCount      int
	MethodUsed     TriangulationMethod
	ConversionDate time.Time
	RateType       string
	ReferenceType  string
	ReferenceID    common.ID
	CreatedBy      common.ID
	CreatedAt      time.Time
}

func NewTriangulationLog(
	entityID common.ID,
	result *TriangulationResult,
	referenceType string,
	referenceID common.ID,
	createdBy common.ID,
) *TriangulationLog {
	return &TriangulationLog{
		ID:             common.NewID(),
		EntityID:       entityID,
		FromCurrency:   result.FromCurrency.Code,
		ToCurrency:     result.ToCurrency.Code,
		OriginalAmount: result.OriginalAmount.Amount,
		ResultAmount:   result.ResultAmount.Amount,
		EffectiveRate:  result.EffectiveRate,
		Legs:           result.Legs,
		LegsCount:      len(result.Legs),
		MethodUsed:     result.Method,
		ConversionDate: result.ConversionDate,
		RateType:       string(result.RateType),
		ReferenceType:  referenceType,
		ReferenceID:    referenceID,
		CreatedBy:      createdBy,
		CreatedAt:      time.Now(),
	}
}

type ConversionPath struct {
	Currencies    []string
	Rates         []decimal.Decimal
	EffectiveRate decimal.Decimal
	LegsCount     int
}

func (p *ConversionPath) IsValid() bool {
	return p.LegsCount > 0 && len(p.Currencies) == p.LegsCount+1 && len(p.Rates) == p.LegsCount
}
