package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/modules/segment/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type ReportService struct {
	reportRepo     repository.ReportRepository
	reportDataRepo repository.ReportDataRepository
	segmentRepo    repository.SegmentRepository
	balanceRepo    repository.BalanceRepository
	auditLogger    *audit.Logger
	logger         *zap.Logger
}

func NewReportService(
	reportRepo repository.ReportRepository,
	reportDataRepo repository.ReportDataRepository,
	segmentRepo repository.SegmentRepository,
	balanceRepo repository.BalanceRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *ReportService {
	return &ReportService{
		reportRepo:     reportRepo,
		reportDataRepo: reportDataRepo,
		segmentRepo:    segmentRepo,
		balanceRepo:    balanceRepo,
		auditLogger:    auditLogger,
		logger:         logger,
	}
}

type GenerateSegmentReportRequest struct {
	EntityID            common.ID
	ReportName          string
	FiscalPeriodID      common.ID
	FiscalYearID        common.ID
	AsOfDate            time.Time
	SegmentType         domain.SegmentType
	HierarchyID         *common.ID
	IncludeIntersegment bool
	CurrencyCode        string
	GeneratedBy         common.ID
}

func (s *ReportService) GenerateSegmentReport(ctx context.Context, req GenerateSegmentReportRequest) (*domain.SegmentReport, error) {
	if !req.SegmentType.IsValid() {
		return nil, domain.ErrInvalidSegmentType
	}

	segments, _, err := s.segmentRepo.List(ctx, repository.SegmentFilter{
		EntityID:    req.EntityID,
		SegmentType: &req.SegmentType,
		IsActive:    boolPtr(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list segments: %w", err)
	}

	if len(segments) == 0 {
		return nil, domain.ErrNoSegmentsForReport
	}

	reportNumber, err := s.reportRepo.GenerateReportNumber(ctx, req.EntityID, "SEG")
	if err != nil {
		return nil, fmt.Errorf("failed to generate report number: %w", err)
	}

	report := domain.NewSegmentReport(
		req.EntityID,
		reportNumber,
		req.ReportName,
		req.FiscalPeriodID,
		req.FiscalYearID,
		req.AsOfDate,
		req.SegmentType,
		req.CurrencyCode,
		req.GeneratedBy,
	)

	if req.HierarchyID != nil {
		report.SetHierarchy(*req.HierarchyID)
	}
	report.SetIncludeIntersegment(req.IncludeIntersegment)

	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to create report: %w", err)
	}

	var reportData []domain.ReportData
	rowOrder := 0

	for _, segment := range segments {
		if !segment.IsReportable {
			continue
		}

		balances, err := s.balanceRepo.ListBySegment(ctx, segment.ID, req.FiscalPeriodID)
		if err != nil {
			s.logger.Warn("failed to get segment balances",
				zap.String("segment_id", string(segment.ID)),
				zap.Error(err))
			continue
		}

		segmentTotals := s.calculateSegmentTotals(balances)

		for _, total := range segmentTotals {
			rowOrder++
			data := domain.NewReportData(
				report.ID,
				segment.ID,
				total.RowType,
				total.LineItem,
				total.Amount,
				total.IntersegmentAmount,
				total.ExternalAmount,
				rowOrder,
			)
			reportData = append(reportData, *data)
		}
	}

	if len(reportData) > 0 {
		totalAmount := decimal.Zero
		for i := range reportData {
			totalAmount = totalAmount.Add(reportData[i].Amount)
		}

		for i := range reportData {
			reportData[i].SetPercentageOfTotal(totalAmount)
		}

		if err := s.reportDataRepo.CreateBatch(ctx, reportData); err != nil {
			return nil, fmt.Errorf("failed to create report data: %w", err)
		}
	}

	report.Data = reportData

	err = s.auditLogger.Log(ctx, "segment_report", report.ID, "report.generated", map[string]any{
		"entity_id":     req.EntityID,
		"report_number": reportNumber,
		"segment_type":  req.SegmentType,
		"segment_count": len(segments),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log audit action: %w", err)
	}

	return report, nil
}

type segmentTotal struct {
	RowType            domain.RowType
	LineItem           string
	Amount             decimal.Decimal
	IntersegmentAmount decimal.Decimal
	ExternalAmount     decimal.Decimal
}

func (s *ReportService) calculateSegmentTotals(balances []domain.SegmentBalance) []segmentTotal {
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for _, bal := range balances {
		totalDebit = totalDebit.Add(bal.DebitAmount)
		totalCredit = totalCredit.Add(bal.CreditAmount)
	}

	netAmount := totalDebit.Sub(totalCredit)

	return []segmentTotal{
		{
			RowType:            domain.RowTypeRevenue,
			LineItem:           "Total Revenue",
			Amount:             totalCredit,
			IntersegmentAmount: decimal.Zero,
			ExternalAmount:     totalCredit,
		},
		{
			RowType:            domain.RowTypeExpense,
			LineItem:           "Total Expenses",
			Amount:             totalDebit,
			IntersegmentAmount: decimal.Zero,
			ExternalAmount:     totalDebit,
		},
		{
			RowType:            domain.RowTypeTotal,
			LineItem:           "Net Amount",
			Amount:             netAmount,
			IntersegmentAmount: decimal.Zero,
			ExternalAmount:     netAmount,
		},
	}
}

func (s *ReportService) GetReport(ctx context.Context, id common.ID) (*domain.SegmentReport, error) {
	report, err := s.reportRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	data, err := s.reportDataRepo.GetByReportID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load report data: %w", err)
	}
	report.Data = data

	return report, nil
}

func (s *ReportService) GetReportByNumber(ctx context.Context, entityID common.ID, reportNumber string) (*domain.SegmentReport, error) {
	report, err := s.reportRepo.GetByReportNumber(ctx, entityID, reportNumber)
	if err != nil {
		return nil, err
	}

	data, err := s.reportDataRepo.GetByReportID(ctx, report.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load report data: %w", err)
	}
	report.Data = data

	return report, nil
}

func (s *ReportService) ListReports(ctx context.Context, filter repository.ReportFilter) ([]domain.SegmentReport, int, error) {
	return s.reportRepo.List(ctx, filter)
}

func (s *ReportService) FinalizeReport(ctx context.Context, id common.ID) (*domain.SegmentReport, error) {
	report, err := s.GetReport(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := report.Finalize(); err != nil {
		return nil, err
	}

	if err := s.reportRepo.Update(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to update report: %w", err)
	}

	err = s.auditLogger.Log(ctx, "segment_report", id, "report.finalized", map[string]any{
		"entity_id":     report.EntityID,
		"report_number": report.ReportNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return report, nil
}

func (s *ReportService) ApproveReport(ctx context.Context, id common.ID) (*domain.SegmentReport, error) {
	report, err := s.GetReport(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := report.Approve(); err != nil {
		return nil, err
	}

	if err := s.reportRepo.Update(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to update report: %w", err)
	}

	err = s.auditLogger.Log(ctx, "segment_report", id, "report.approved", map[string]any{
		"entity_id":     report.EntityID,
		"report_number": report.ReportNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return report, nil
}

func (s *ReportService) PublishReport(ctx context.Context, id common.ID) (*domain.SegmentReport, error) {
	report, err := s.GetReport(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := report.Publish(); err != nil {
		return nil, err
	}

	if err := s.reportRepo.Update(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to update report: %w", err)
	}

	err = s.auditLogger.Log(ctx, "segment_report", id, "report.published", map[string]any{
		"entity_id":     report.EntityID,
		"report_number": report.ReportNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return report, nil
}

func (s *ReportService) DeleteReport(ctx context.Context, id common.ID) error {
	report, err := s.reportRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !report.CanEdit() {
		return domain.ErrInvalidReportStatus
	}

	if err := s.reportDataRepo.DeleteByReportID(ctx, id); err != nil {
		return fmt.Errorf("failed to delete report data: %w", err)
	}

	if err := s.reportRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete report: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "segment_report", id, "report.deleted", map[string]any{
		"entity_id":     report.EntityID,
		"report_number": report.ReportNumber,
	}); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

func (s *ReportService) RegenerateReportData(ctx context.Context, id common.ID) (*domain.SegmentReport, error) {
	report, err := s.reportRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !report.CanEdit() {
		return nil, domain.ErrInvalidReportStatus
	}

	if err := s.reportDataRepo.DeleteByReportID(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to clear report data: %w", err)
	}

	segments, _, err := s.segmentRepo.List(ctx, repository.SegmentFilter{
		EntityID:    report.EntityID,
		SegmentType: &report.SegmentType,
		IsActive:    boolPtr(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list segments: %w", err)
	}

	var reportData []domain.ReportData
	rowOrder := 0

	for _, segment := range segments {
		if !segment.IsReportable {
			continue
		}

		balances, err := s.balanceRepo.ListBySegment(ctx, segment.ID, report.FiscalPeriodID)
		if err != nil {
			s.logger.Warn("failed to get segment balances",
				zap.String("segment_id", string(segment.ID)),
				zap.Error(err))
			continue
		}

		segmentTotals := s.calculateSegmentTotals(balances)

		for _, total := range segmentTotals {
			rowOrder++
			data := domain.NewReportData(
				report.ID,
				segment.ID,
				total.RowType,
				total.LineItem,
				total.Amount,
				total.IntersegmentAmount,
				total.ExternalAmount,
				rowOrder,
			)
			reportData = append(reportData, *data)
		}
	}

	if len(reportData) > 0 {
		totalAmount := decimal.Zero
		for i := range reportData {
			totalAmount = totalAmount.Add(reportData[i].Amount)
		}

		for i := range reportData {
			reportData[i].SetPercentageOfTotal(totalAmount)
		}

		if err := s.reportDataRepo.CreateBatch(ctx, reportData); err != nil {
			return nil, fmt.Errorf("failed to create report data: %w", err)
		}
	}

	report.Data = reportData

	if err := s.auditLogger.Log(ctx, "segment_report", id, "report.regenerated", map[string]any{
		"entity_id":     report.EntityID,
		"report_number": report.ReportNumber,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return report, nil
}

type ReportSummaryData struct {
	SegmentID      common.ID
	SegmentCode    string
	SegmentName    string
	TotalRevenue   decimal.Decimal
	TotalExpenses  decimal.Decimal
	NetAmount      decimal.Decimal
	PercentOfTotal decimal.Decimal
}

func (s *ReportService) GetReportSummary(ctx context.Context, id common.ID) ([]ReportSummaryData, error) {
	report, err := s.GetReport(ctx, id)
	if err != nil {
		return nil, err
	}

	segmentMap := make(map[common.ID]*ReportSummaryData)

	for _, data := range report.Data {
		if _, exists := segmentMap[data.SegmentID]; !exists {
			segment, err := s.segmentRepo.GetByID(ctx, data.SegmentID)
			if err != nil {
				continue
			}
			segmentMap[data.SegmentID] = &ReportSummaryData{
				SegmentID:   data.SegmentID,
				SegmentCode: segment.SegmentCode,
				SegmentName: segment.SegmentName,
			}
		}

		summary := segmentMap[data.SegmentID]

		switch data.RowType {
		case domain.RowTypeRevenue:
			summary.TotalRevenue = summary.TotalRevenue.Add(data.Amount)
		case domain.RowTypeExpense:
			summary.TotalExpenses = summary.TotalExpenses.Add(data.Amount)
		case domain.RowTypeTotal:
			summary.NetAmount = data.Amount
		}

		if data.PercentageOfTotal != nil {
			summary.PercentOfTotal = *data.PercentageOfTotal
		}
	}

	var summaries []ReportSummaryData
	for _, summary := range segmentMap {
		summaries = append(summaries, *summary)
	}

	return summaries, nil
}

func boolPtr(b bool) *bool {
	return &b
}
