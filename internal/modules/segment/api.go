package segment

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

type API interface {
	CreateSegment(ctx context.Context, req CreateSegmentRequest) (*SegmentResponse, error)

	GetSegment(ctx context.Context, id common.ID) (*SegmentResponse, error)

	GetSegmentTree(ctx context.Context, entityID common.ID, segmentType string) (*SegmentTreeResponse, error)

	ListSegments(ctx context.Context, req ListSegmentsRequest) (*ListSegmentsResponse, error)

	UpdateSegment(ctx context.Context, req UpdateSegmentRequest) (*SegmentResponse, error)

	DeleteSegment(ctx context.Context, id common.ID) error

	AssignToSegment(ctx context.Context, req AssignToSegmentRequest) (*AssignmentResponse, error)

	GetEffectiveAssignments(ctx context.Context, entityID common.ID, assignmentType string, assignmentID common.ID, date time.Time) ([]AssignmentResponse, error)

	CalculateSegmentBalances(ctx context.Context, req CalculateBalancesRequest) error

	GetBalanceSummary(ctx context.Context, entityID, periodID common.ID) ([]BalanceSummaryResponse, error)

	GenerateSegmentReport(ctx context.Context, req GenerateReportRequest) (*ReportResponse, error)

	GetReport(ctx context.Context, id common.ID) (*ReportResponse, error)

	ListReports(ctx context.Context, req ListReportsRequest) (*ListReportsResponse, error)

	FinalizeReport(ctx context.Context, id common.ID) (*ReportResponse, error)

	ApproveReport(ctx context.Context, id common.ID) (*ReportResponse, error)

	PublishReport(ctx context.Context, id common.ID) (*ReportResponse, error)
}

type CreateSegmentRequest struct {
	EntityID     common.ID
	SegmentCode  string
	SegmentName  string
	SegmentType  string
	ParentID     *common.ID
	Description  string
	ManagerID    *common.ID
	IsReportable bool
}

type UpdateSegmentRequest struct {
	ID           common.ID
	SegmentName  string
	Description  string
	ParentID     *common.ID
	ManagerID    *common.ID
	IsReportable bool
}

type ListSegmentsRequest struct {
	EntityID     common.ID
	SegmentType  *string
	IsActive     *bool
	IsReportable *bool
	Page         int
	PageSize     int
}

type SegmentResponse struct {
	ID           common.ID
	EntityID     common.ID
	SegmentCode  string
	SegmentName  string
	SegmentType  string
	ParentID     *common.ID
	Description  string
	ManagerID    *common.ID
	IsReportable bool
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SegmentTreeResponse struct {
	Segments    []SegmentWithChildren
	SegmentType string
	TotalCount  int
}

type SegmentWithChildren struct {
	ID           common.ID
	SegmentCode  string
	SegmentName  string
	ParentID     *common.ID
	IsReportable bool
	IsActive     bool
	Children     []SegmentWithChildren
}

type ListSegmentsResponse struct {
	Segments []SegmentResponse
	Total    int
	Page     int
}

type AssignToSegmentRequest struct {
	EntityID          common.ID
	SegmentID         common.ID
	AssignmentType    string
	AssignmentID      common.ID
	AllocationPercent decimal.Decimal
	EffectiveFrom     time.Time
	EffectiveTo       *time.Time
}

type AssignmentResponse struct {
	ID                common.ID
	EntityID          common.ID
	SegmentID         common.ID
	AssignmentType    string
	AssignmentID      common.ID
	AllocationPercent decimal.Decimal
	EffectiveFrom     time.Time
	EffectiveTo       *time.Time
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type BalanceCalculationProvider interface {
	GetAccountBalances(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]AccountBalanceData, error)
	GetSegmentAllocations(ctx context.Context, entityID, accountID common.ID, effectiveDate time.Time) ([]SegmentAllocationData, error)
}

type AccountBalanceData struct {
	AccountID    common.ID
	AccountCode  string
	AccountName  string
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
	NetAmount    decimal.Decimal
	CurrencyCode string
}

type SegmentAllocationData struct {
	SegmentID         common.ID
	AllocationPercent decimal.Decimal
}

type CalculateBalancesRequest struct {
	EntityID      common.ID
	PeriodID      common.ID
	Provider      BalanceCalculationProvider
	EffectiveDate time.Time
	CurrencyCode  string
}

type BalanceSummaryResponse struct {
	SegmentID    common.ID
	SegmentCode  string
	SegmentName  string
	TotalDebit   decimal.Decimal
	TotalCredit  decimal.Decimal
	NetAmount    decimal.Decimal
	AccountCount int
}

type GenerateReportRequest struct {
	EntityID            common.ID
	ReportName          string
	FiscalPeriodID      common.ID
	FiscalYearID        common.ID
	AsOfDate            time.Time
	SegmentType         string
	HierarchyID         *common.ID
	IncludeIntersegment bool
	CurrencyCode        string
	GeneratedBy         common.ID
}

type ListReportsRequest struct {
	EntityID       common.ID
	FiscalPeriodID *common.ID
	FiscalYearID   *common.ID
	SegmentType    *string
	Status         *string
	Page           int
	PageSize       int
}

type ReportResponse struct {
	ID                  common.ID
	EntityID            common.ID
	ReportNumber        string
	ReportName          string
	FiscalPeriodID      common.ID
	FiscalYearID        common.ID
	AsOfDate            time.Time
	SegmentType         string
	HierarchyID         *common.ID
	IncludeIntersegment bool
	CurrencyCode        string
	Status              string
	GeneratedBy         common.ID
	GeneratedAt         time.Time
	Data                []ReportDataResponse
}

type ReportDataResponse struct {
	ID                 common.ID
	ReportID           common.ID
	SegmentID          common.ID
	RowType            string
	LineItem           string
	Amount             decimal.Decimal
	IntersegmentAmount decimal.Decimal
	ExternalAmount     decimal.Decimal
	PercentageOfTotal  *decimal.Decimal
	RowOrder           int
}

type ListReportsResponse struct {
	Reports []ReportResponse
	Total   int
	Page    int
}
