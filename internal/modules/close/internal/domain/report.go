package domain

import (
	"encoding/json"
	"errors"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type ReportType string

const (
	ReportTypeTrialBalance     ReportType = "trial_balance"
	ReportTypeIncomeStatement  ReportType = "income_statement"
	ReportTypeBalanceSheet     ReportType = "balance_sheet"
	ReportTypeCashFlow         ReportType = "cash_flow"
	ReportTypeChangesInEquity  ReportType = "changes_in_equity"
	ReportTypeGeneralLedger    ReportType = "general_ledger"
	ReportTypeSubsidiaryLedger ReportType = "subsidiary_ledger"
	ReportTypeAging            ReportType = "aging"
	ReportTypeCustom           ReportType = "custom"
)

func (t ReportType) IsValid() bool {
	switch t {
	case ReportTypeTrialBalance, ReportTypeIncomeStatement, ReportTypeBalanceSheet,
		ReportTypeCashFlow, ReportTypeChangesInEquity, ReportTypeGeneralLedger,
		ReportTypeSubsidiaryLedger, ReportTypeAging, ReportTypeCustom:
		return true
	}
	return false
}

func (t ReportType) String() string {
	return string(t)
}

type ReportFormat string

const (
	ReportFormatSummary      ReportFormat = "summary"
	ReportFormatDetailed     ReportFormat = "detailed"
	ReportFormatComparative  ReportFormat = "comparative"
	ReportFormatConsolidated ReportFormat = "consolidated"
)

func (f ReportFormat) IsValid() bool {
	switch f {
	case ReportFormatSummary, ReportFormatDetailed, ReportFormatComparative, ReportFormatConsolidated:
		return true
	}
	return false
}

func (f ReportFormat) String() string {
	return string(f)
}

type ReportStatus string

const (
	ReportStatusPending    ReportStatus = "pending"
	ReportStatusGenerating ReportStatus = "generating"
	ReportStatusCompleted  ReportStatus = "completed"
	ReportStatusFailed     ReportStatus = "failed"
)

func (s ReportStatus) IsValid() bool {
	switch s {
	case ReportStatusPending, ReportStatusGenerating, ReportStatusCompleted, ReportStatusFailed:
		return true
	}
	return false
}

func (s ReportStatus) String() string {
	return string(s)
}

type ReportTemplate struct {
	ID               common.ID
	EntityID         *common.ID
	TemplateCode     string
	TemplateName     string
	ReportType       ReportType
	ReportFormat     ReportFormat
	IsSystemTemplate bool
	Configuration    json.RawMessage
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewReportTemplate(
	entityID *common.ID,
	templateCode, templateName string,
	reportType ReportType,
	reportFormat ReportFormat,
) *ReportTemplate {
	now := time.Now()
	return &ReportTemplate{
		ID:               common.NewID(),
		EntityID:         entityID,
		TemplateCode:     templateCode,
		TemplateName:     templateName,
		ReportType:       reportType,
		ReportFormat:     reportFormat,
		IsSystemTemplate: entityID == nil,
		Configuration:    json.RawMessage("{}"),
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func (t *ReportTemplate) SetConfiguration(config map[string]interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	t.Configuration = data
	t.UpdatedAt = time.Now()
	return nil
}

func (t *ReportTemplate) GetConfiguration() (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal(t.Configuration, &config); err != nil {
		return nil, err
	}
	return config, nil
}

type ReportRun struct {
	ID                 common.ID
	EntityID           common.ID
	ReportNumber       string
	TemplateID         *common.ID
	ReportType         ReportType
	ReportFormat       ReportFormat
	ReportName         string
	FiscalPeriodID     *common.ID
	FiscalYearID       *common.ID
	AsOfDate           time.Time
	ComparisonPeriodID *common.ID
	ComparisonYearID   *common.ID
	Parameters         json.RawMessage
	Status             ReportStatus
	GeneratedBy        common.ID
	GeneratedAt        *time.Time
	CompletedAt        *time.Time
	ErrorMessage       string
	CreatedAt          time.Time

	DataRows []ReportDataRow
}

func NewReportRun(
	entityID common.ID,
	reportNumber string,
	templateID *common.ID,
	reportType ReportType,
	reportFormat ReportFormat,
	reportName string,
	asOfDate time.Time,
	generatedBy common.ID,
) *ReportRun {
	now := time.Now()
	return &ReportRun{
		ID:           common.NewID(),
		EntityID:     entityID,
		ReportNumber: reportNumber,
		TemplateID:   templateID,
		ReportType:   reportType,
		ReportFormat: reportFormat,
		ReportName:   reportName,
		AsOfDate:     asOfDate,
		Parameters:   json.RawMessage("{}"),
		Status:       ReportStatusPending,
		GeneratedBy:  generatedBy,
		CreatedAt:    now,
	}
}

func (r *ReportRun) SetPeriod(fiscalPeriodID, fiscalYearID common.ID) {
	r.FiscalPeriodID = &fiscalPeriodID
	r.FiscalYearID = &fiscalYearID
}

func (r *ReportRun) SetComparisonPeriod(periodID, yearID common.ID) {
	r.ComparisonPeriodID = &periodID
	r.ComparisonYearID = &yearID
}

func (r *ReportRun) SetParameters(params map[string]interface{}) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	r.Parameters = data
	return nil
}

func (r *ReportRun) StartGenerating() error {
	if r.Status != ReportStatusPending {
		return errors.New("report must be pending to start generating")
	}

	now := time.Now()
	r.Status = ReportStatusGenerating
	r.GeneratedAt = &now
	return nil
}

func (r *ReportRun) Complete() error {
	if r.Status != ReportStatusGenerating {
		return errors.New("report must be generating to complete")
	}

	now := time.Now()
	r.Status = ReportStatusCompleted
	r.CompletedAt = &now
	return nil
}

func (r *ReportRun) Fail(errorMessage string) error {
	if r.Status != ReportStatusGenerating {
		return errors.New("report must be generating to fail")
	}

	r.Status = ReportStatusFailed
	r.ErrorMessage = errorMessage
	return nil
}

func (r *ReportRun) AddDataRow(row ReportDataRow) {
	r.DataRows = append(r.DataRows, row)
}

type RowType string

const (
	RowTypeHeader   RowType = "header"
	RowTypeDetail   RowType = "detail"
	RowTypeSubtotal RowType = "subtotal"
	RowTypeTotal    RowType = "total"
	RowTypeBlank    RowType = "blank"
)

type ReportDataRow struct {
	ID             common.ID
	ReportRunID    common.ID
	RowNumber      int
	RowType        RowType
	IndentLevel    int
	AccountID      *common.ID
	AccountCode    string
	AccountName    string
	Description    string
	CostCenterID   *common.ID
	CostCenterCode string
	Amount1        *float64
	Amount2        *float64
	Amount3        *float64
	Amount4        *float64
	Amount5        *float64
	CurrencyCode   string
	IsBold         bool
	IsUnderlined   bool
	CreatedAt      time.Time
}

func NewReportDataRow(
	reportRunID common.ID,
	rowNumber int,
	rowType RowType,
) *ReportDataRow {
	return &ReportDataRow{
		ID:           common.NewID(),
		ReportRunID:  reportRunID,
		RowNumber:    rowNumber,
		RowType:      rowType,
		IndentLevel:  0,
		IsBold:       false,
		IsUnderlined: false,
		CreatedAt:    time.Now(),
	}
}

type ReportTemplateFilter struct {
	EntityID         *common.ID
	ReportType       *ReportType
	IsSystemTemplate *bool
	IsActive         *bool
	Limit            int
	Offset           int
}

type ReportRunFilter struct {
	EntityID       *common.ID
	TemplateID     *common.ID
	ReportType     *ReportType
	FiscalPeriodID *common.ID
	FiscalYearID   *common.ID
	Status         *ReportStatus
	Limit          int
	Offset         int
}

type ScheduledReport struct {
	ID           common.ID
	EntityID     common.ID
	TemplateID   common.ID
	ScheduleName string
	Frequency    string
	DayOfWeek    *int
	DayOfMonth   *int
	Parameters   json.RawMessage
	Recipients   json.RawMessage
	OutputFormat string
	IsActive     bool
	LastRunAt    *time.Time
	NextRunAt    *time.Time
	CreatedBy    common.ID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type YearEndChecklist struct {
	ID            common.ID
	EntityID      common.ID
	FiscalYearID  common.ID
	ChecklistName string
	Status        string
	StartedAt     time.Time
	CompletedAt   *time.Time
	CompletedBy   *common.ID
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Items []YearEndChecklistItem
}

type YearEndChecklistItem struct {
	ID             common.ID
	ChecklistID    common.ID
	SequenceNumber int
	ItemCode       string
	ItemName       string
	Description    string
	IsRequired     bool
	IsCompleted    bool
	CompletedAt    *time.Time
	CompletedBy    *common.ID
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
