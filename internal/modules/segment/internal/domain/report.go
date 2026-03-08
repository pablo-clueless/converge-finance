package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

type ReportStatus string

const (
	ReportStatusDraft     ReportStatus = "draft"
	ReportStatusFinalized ReportStatus = "finalized"
	ReportStatusApproved  ReportStatus = "approved"
	ReportStatusPublished ReportStatus = "published"
)

func (s ReportStatus) IsValid() bool {
	switch s {
	case ReportStatusDraft, ReportStatusFinalized, ReportStatusApproved, ReportStatusPublished:
		return true
	default:
		return false
	}
}

type RowType string

const (
	RowTypeRevenue          RowType = "revenue"
	RowTypeExpense          RowType = "expense"
	RowTypeAsset            RowType = "asset"
	RowTypeLiability        RowType = "liability"
	RowTypeEquity           RowType = "equity"
	RowTypeIntersegment     RowType = "intersegment"
	RowTypeElimination      RowType = "elimination"
	RowTypeSubtotal         RowType = "subtotal"
	RowTypeTotal            RowType = "total"
)

type SegmentReport struct {
	ID                  common.ID
	EntityID            common.ID
	ReportNumber        string
	ReportName          string
	FiscalPeriodID      common.ID
	FiscalYearID        common.ID
	AsOfDate            time.Time
	SegmentType         SegmentType
	HierarchyID         *common.ID
	IncludeIntersegment bool
	CurrencyCode        string
	Status              ReportStatus
	GeneratedBy         common.ID
	GeneratedAt         time.Time

	Data []ReportData
}

func NewSegmentReport(
	entityID common.ID,
	reportNumber, reportName string,
	fiscalPeriodID, fiscalYearID common.ID,
	asOfDate time.Time,
	segmentType SegmentType,
	currencyCode string,
	generatedBy common.ID,
) *SegmentReport {
	return &SegmentReport{
		ID:                  common.NewID(),
		EntityID:            entityID,
		ReportNumber:        reportNumber,
		ReportName:          reportName,
		FiscalPeriodID:      fiscalPeriodID,
		FiscalYearID:        fiscalYearID,
		AsOfDate:            asOfDate,
		SegmentType:         segmentType,
		IncludeIntersegment: true,
		CurrencyCode:        currencyCode,
		Status:              ReportStatusDraft,
		GeneratedBy:         generatedBy,
		GeneratedAt:         time.Now(),
		Data:                []ReportData{},
	}
}

func (r *SegmentReport) SetHierarchy(hierarchyID common.ID) {
	r.HierarchyID = &hierarchyID
}

func (r *SegmentReport) SetIncludeIntersegment(include bool) {
	r.IncludeIntersegment = include
}

func (r *SegmentReport) AddData(data ReportData) {
	r.Data = append(r.Data, data)
}

func (r *SegmentReport) Finalize() error {
	if r.Status != ReportStatusDraft {
		return ErrInvalidReportStatus
	}
	r.Status = ReportStatusFinalized
	return nil
}

func (r *SegmentReport) Approve() error {
	if r.Status != ReportStatusFinalized {
		return ErrInvalidReportStatus
	}
	r.Status = ReportStatusApproved
	return nil
}

func (r *SegmentReport) Publish() error {
	if r.Status != ReportStatusApproved {
		return ErrInvalidReportStatus
	}
	r.Status = ReportStatusPublished
	return nil
}

func (r *SegmentReport) CanEdit() bool {
	return r.Status == ReportStatusDraft
}

type ReportData struct {
	ID                common.ID
	ReportID          common.ID
	SegmentID         common.ID
	RowType           RowType
	LineItem          string
	Amount            decimal.Decimal
	IntersegmentAmount decimal.Decimal
	ExternalAmount    decimal.Decimal
	PercentageOfTotal *decimal.Decimal
	RowOrder          int
	CreatedAt         time.Time
}

func NewReportData(
	reportID, segmentID common.ID,
	rowType RowType,
	lineItem string,
	amount, intersegmentAmount, externalAmount decimal.Decimal,
	rowOrder int,
) *ReportData {
	return &ReportData{
		ID:                common.NewID(),
		ReportID:          reportID,
		SegmentID:         segmentID,
		RowType:           rowType,
		LineItem:          lineItem,
		Amount:            amount,
		IntersegmentAmount: intersegmentAmount,
		ExternalAmount:    externalAmount,
		RowOrder:          rowOrder,
		CreatedAt:         time.Now(),
	}
}

func (d *ReportData) SetPercentageOfTotal(total decimal.Decimal) {
	if total.IsZero() {
		return
	}
	pct := d.Amount.Div(total).Mul(decimal.NewFromInt(100))
	d.PercentageOfTotal = &pct
}

func (d *ReportData) CalculateExternalAmount() {
	d.ExternalAmount = d.Amount.Sub(d.IntersegmentAmount)
}

type ReportSummary struct {
	ReportID       common.ID
	ReportNumber   string
	ReportName     string
	SegmentType    SegmentType
	Status         ReportStatus
	SegmentCount   int
	TotalRevenue   decimal.Decimal
	TotalExpenses  decimal.Decimal
	TotalAssets    decimal.Decimal
	GeneratedAt    time.Time
}
