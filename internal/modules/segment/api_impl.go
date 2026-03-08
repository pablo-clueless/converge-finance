package segment

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/modules/segment/internal/repository"
	"converge-finance.com/m/internal/modules/segment/internal/service"
)

type segmentAPI struct {
	segmentService *service.SegmentService
	reportService  *service.ReportService
}

func NewSegmentAPI(segmentService *service.SegmentService, reportService *service.ReportService) API {
	return &segmentAPI{
		segmentService: segmentService,
		reportService:  reportService,
	}
}

func (a *segmentAPI) CreateSegment(ctx context.Context, req CreateSegmentRequest) (*SegmentResponse, error) {
	segment, err := a.segmentService.CreateSegment(ctx, service.CreateSegmentRequest{
		EntityID:     req.EntityID,
		SegmentCode:  req.SegmentCode,
		SegmentName:  req.SegmentName,
		SegmentType:  domain.SegmentType(req.SegmentType),
		ParentID:     req.ParentID,
		Description:  req.Description,
		ManagerID:    req.ManagerID,
		IsReportable: req.IsReportable,
	})
	if err != nil {
		return nil, err
	}

	return a.mapSegmentToResponse(segment), nil
}

func (a *segmentAPI) GetSegment(ctx context.Context, id common.ID) (*SegmentResponse, error) {
	segment, err := a.segmentService.GetSegment(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapSegmentToResponse(segment), nil
}

func (a *segmentAPI) GetSegmentTree(ctx context.Context, entityID common.ID, segmentType string) (*SegmentTreeResponse, error) {
	tree, err := a.segmentService.GetSegmentTree(ctx, entityID, domain.SegmentType(segmentType))
	if err != nil {
		return nil, err
	}

	return &SegmentTreeResponse{
		Segments:    a.mapSegmentsWithChildren(tree.Segments),
		SegmentType: string(tree.SegmentType),
		TotalCount:  tree.TotalCount,
	}, nil
}

func (a *segmentAPI) mapSegmentsWithChildren(segments []domain.Segment) []SegmentWithChildren {
	result := make([]SegmentWithChildren, len(segments))
	for i, seg := range segments {
		result[i] = SegmentWithChildren{
			ID:           seg.ID,
			SegmentCode:  seg.SegmentCode,
			SegmentName:  seg.SegmentName,
			ParentID:     seg.ParentID,
			IsReportable: seg.IsReportable,
			IsActive:     seg.IsActive,
			Children:     a.mapSegmentsWithChildren(seg.Children),
		}
	}
	return result
}

func (a *segmentAPI) ListSegments(ctx context.Context, req ListSegmentsRequest) (*ListSegmentsResponse, error) {
	filter := repository.SegmentFilter{
		EntityID: req.EntityID,
		IsActive: req.IsActive,
		Limit:    req.PageSize,
		Offset:   (req.Page - 1) * req.PageSize,
	}

	if req.SegmentType != nil {
		t := domain.SegmentType(*req.SegmentType)
		filter.SegmentType = &t
	}

	if req.IsReportable != nil {
		filter.IsReportable = req.IsReportable
	}

	segments, total, err := a.segmentService.ListSegments(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]SegmentResponse, len(segments))
	for i, seg := range segments {
		responses[i] = *a.mapSegmentToResponse(&seg)
	}

	return &ListSegmentsResponse{
		Segments: responses,
		Total:    total,
		Page:     req.Page,
	}, nil
}

func (a *segmentAPI) UpdateSegment(ctx context.Context, req UpdateSegmentRequest) (*SegmentResponse, error) {
	segment, err := a.segmentService.UpdateSegment(ctx, service.UpdateSegmentRequest{
		ID:           req.ID,
		SegmentName:  req.SegmentName,
		Description:  req.Description,
		ParentID:     req.ParentID,
		ManagerID:    req.ManagerID,
		IsReportable: req.IsReportable,
	})
	if err != nil {
		return nil, err
	}

	return a.mapSegmentToResponse(segment), nil
}

func (a *segmentAPI) DeleteSegment(ctx context.Context, id common.ID) error {
	return a.segmentService.DeleteSegment(ctx, id)
}

func (a *segmentAPI) AssignToSegment(ctx context.Context, req AssignToSegmentRequest) (*AssignmentResponse, error) {
	assignment, err := a.segmentService.AssignToSegment(ctx, service.AssignToSegmentRequest{
		EntityID:          req.EntityID,
		SegmentID:         req.SegmentID,
		AssignmentType:    req.AssignmentType,
		AssignmentID:      req.AssignmentID,
		AllocationPercent: req.AllocationPercent,
		EffectiveFrom:     req.EffectiveFrom,
		EffectiveTo:       req.EffectiveTo,
	})
	if err != nil {
		return nil, err
	}

	return a.mapAssignmentToResponse(assignment), nil
}

func (a *segmentAPI) GetEffectiveAssignments(ctx context.Context, entityID common.ID, assignmentType string, assignmentID common.ID, date time.Time) ([]AssignmentResponse, error) {
	assignments, err := a.segmentService.GetEffectiveAssignments(ctx, entityID, assignmentType, assignmentID, date)
	if err != nil {
		return nil, err
	}

	responses := make([]AssignmentResponse, len(assignments))
	for i, assign := range assignments {
		responses[i] = *a.mapAssignmentToResponse(&assign)
	}

	return responses, nil
}

func (a *segmentAPI) CalculateSegmentBalances(ctx context.Context, req CalculateBalancesRequest) error {
	provider := &balanceCalculationProviderAdapter{provider: req.Provider}
	return a.segmentService.CalculateSegmentBalances(
		ctx,
		req.EntityID,
		req.PeriodID,
		provider,
		req.EffectiveDate,
		req.CurrencyCode,
	)
}

type balanceCalculationProviderAdapter struct {
	provider BalanceCalculationProvider
}

func (p *balanceCalculationProviderAdapter) GetAccountBalances(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]repository.AccountBalanceData, error) {
	data, err := p.provider.GetAccountBalances(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return nil, err
	}

	result := make([]repository.AccountBalanceData, len(data))
	for i, d := range data {
		result[i] = repository.AccountBalanceData{
			AccountID:    d.AccountID,
			AccountCode:  d.AccountCode,
			AccountName:  d.AccountName,
			DebitAmount:  d.DebitAmount,
			CreditAmount: d.CreditAmount,
			NetAmount:    d.NetAmount,
			CurrencyCode: d.CurrencyCode,
		}
	}
	return result, nil
}

func (p *balanceCalculationProviderAdapter) GetSegmentAllocations(ctx context.Context, entityID, accountID common.ID, effectiveDate time.Time) ([]repository.SegmentAllocationData, error) {
	data, err := p.provider.GetSegmentAllocations(ctx, entityID, accountID, effectiveDate)
	if err != nil {
		return nil, err
	}

	result := make([]repository.SegmentAllocationData, len(data))
	for i, d := range data {
		result[i] = repository.SegmentAllocationData{
			SegmentID:         d.SegmentID,
			AllocationPercent: d.AllocationPercent,
		}
	}
	return result, nil
}

func (a *segmentAPI) GetBalanceSummary(ctx context.Context, entityID, periodID common.ID) ([]BalanceSummaryResponse, error) {
	summaries, err := a.segmentService.GetBalanceSummary(ctx, entityID, periodID)
	if err != nil {
		return nil, err
	}

	responses := make([]BalanceSummaryResponse, len(summaries))
	for i, sum := range summaries {
		responses[i] = BalanceSummaryResponse{
			SegmentID:    sum.SegmentID,
			SegmentCode:  sum.SegmentCode,
			SegmentName:  sum.SegmentName,
			TotalDebit:   sum.TotalDebit,
			TotalCredit:  sum.TotalCredit,
			NetAmount:    sum.NetAmount,
			AccountCount: sum.AccountCount,
		}
	}

	return responses, nil
}

func (a *segmentAPI) GenerateSegmentReport(ctx context.Context, req GenerateReportRequest) (*ReportResponse, error) {
	report, err := a.reportService.GenerateSegmentReport(ctx, service.GenerateSegmentReportRequest{
		EntityID:            req.EntityID,
		ReportName:          req.ReportName,
		FiscalPeriodID:      req.FiscalPeriodID,
		FiscalYearID:        req.FiscalYearID,
		AsOfDate:            req.AsOfDate,
		SegmentType:         domain.SegmentType(req.SegmentType),
		HierarchyID:         req.HierarchyID,
		IncludeIntersegment: req.IncludeIntersegment,
		CurrencyCode:        req.CurrencyCode,
		GeneratedBy:         req.GeneratedBy,
	})
	if err != nil {
		return nil, err
	}

	return a.mapReportToResponse(report), nil
}

func (a *segmentAPI) GetReport(ctx context.Context, id common.ID) (*ReportResponse, error) {
	report, err := a.reportService.GetReport(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapReportToResponse(report), nil
}

func (a *segmentAPI) ListReports(ctx context.Context, req ListReportsRequest) (*ListReportsResponse, error) {
	filter := repository.ReportFilter{
		EntityID: req.EntityID,
		Limit:    req.PageSize,
		Offset:   (req.Page - 1) * req.PageSize,
	}

	if req.FiscalPeriodID != nil {
		filter.FiscalPeriodID = req.FiscalPeriodID
	}

	if req.FiscalYearID != nil {
		filter.FiscalYearID = req.FiscalYearID
	}

	if req.SegmentType != nil {
		t := domain.SegmentType(*req.SegmentType)
		filter.SegmentType = &t
	}

	if req.Status != nil {
		s := domain.ReportStatus(*req.Status)
		filter.Status = &s
	}

	reports, total, err := a.reportService.ListReports(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]ReportResponse, len(reports))
	for i, rpt := range reports {
		responses[i] = *a.mapReportToResponse(&rpt)
	}

	return &ListReportsResponse{
		Reports: responses,
		Total:   total,
		Page:    req.Page,
	}, nil
}

func (a *segmentAPI) FinalizeReport(ctx context.Context, id common.ID) (*ReportResponse, error) {
	report, err := a.reportService.FinalizeReport(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapReportToResponse(report), nil
}

func (a *segmentAPI) ApproveReport(ctx context.Context, id common.ID) (*ReportResponse, error) {
	report, err := a.reportService.ApproveReport(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapReportToResponse(report), nil
}

func (a *segmentAPI) PublishReport(ctx context.Context, id common.ID) (*ReportResponse, error) {
	report, err := a.reportService.PublishReport(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapReportToResponse(report), nil
}

func (a *segmentAPI) mapSegmentToResponse(seg *domain.Segment) *SegmentResponse {
	return &SegmentResponse{
		ID:           seg.ID,
		EntityID:     seg.EntityID,
		SegmentCode:  seg.SegmentCode,
		SegmentName:  seg.SegmentName,
		SegmentType:  string(seg.SegmentType),
		ParentID:     seg.ParentID,
		Description:  seg.Description,
		ManagerID:    seg.ManagerID,
		IsReportable: seg.IsReportable,
		IsActive:     seg.IsActive,
		CreatedAt:    seg.CreatedAt,
		UpdatedAt:    seg.UpdatedAt,
	}
}

func (a *segmentAPI) mapAssignmentToResponse(assign *domain.Assignment) *AssignmentResponse {
	return &AssignmentResponse{
		ID:                assign.ID,
		EntityID:          assign.EntityID,
		SegmentID:         assign.SegmentID,
		AssignmentType:    assign.AssignmentType,
		AssignmentID:      assign.AssignmentID,
		AllocationPercent: assign.AllocationPercent,
		EffectiveFrom:     assign.EffectiveFrom,
		EffectiveTo:       assign.EffectiveTo,
		IsActive:          assign.IsActive,
		CreatedAt:         assign.CreatedAt,
		UpdatedAt:         assign.UpdatedAt,
	}
}

func (a *segmentAPI) mapReportToResponse(rpt *domain.SegmentReport) *ReportResponse {
	resp := &ReportResponse{
		ID:                  rpt.ID,
		EntityID:            rpt.EntityID,
		ReportNumber:        rpt.ReportNumber,
		ReportName:          rpt.ReportName,
		FiscalPeriodID:      rpt.FiscalPeriodID,
		FiscalYearID:        rpt.FiscalYearID,
		AsOfDate:            rpt.AsOfDate,
		SegmentType:         string(rpt.SegmentType),
		HierarchyID:         rpt.HierarchyID,
		IncludeIntersegment: rpt.IncludeIntersegment,
		CurrencyCode:        rpt.CurrencyCode,
		Status:              string(rpt.Status),
		GeneratedBy:         rpt.GeneratedBy,
		GeneratedAt:         rpt.GeneratedAt,
		Data:                make([]ReportDataResponse, len(rpt.Data)),
	}

	for i, data := range rpt.Data {
		resp.Data[i] = ReportDataResponse{
			ID:                 data.ID,
			ReportID:           data.ReportID,
			SegmentID:          data.SegmentID,
			RowType:            string(data.RowType),
			LineItem:           data.LineItem,
			Amount:             data.Amount,
			IntersegmentAmount: data.IntersegmentAmount,
			ExternalAmount:     data.ExternalAmount,
			PercentageOfTotal:  data.PercentageOfTotal,
			RowOrder:           data.RowOrder,
		}
	}

	return resp
}
