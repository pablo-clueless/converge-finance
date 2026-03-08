package consol

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
	"converge-finance.com/m/internal/modules/consol/internal/service"
)

type apiImpl struct {
	consolidationService *service.ConsolidationService
	translationService   *service.TranslationService
}

func NewConsolAPI(
	consolidationService *service.ConsolidationService,
	translationService *service.TranslationService,
) API {
	return &apiImpl{
		consolidationService: consolidationService,
		translationService:   translationService,
	}
}

func (a *apiImpl) GetConsolidationSet(ctx context.Context, id common.ID) (*ConsolidationSetResponse, error) {
	set, err := a.consolidationService.GetConsolidationSet(ctx, id)
	if err != nil {
		return nil, err
	}
	return toConsolidationSetResponse(set), nil
}

func (a *apiImpl) ListConsolidationSets(ctx context.Context, parentEntityID common.ID) ([]ConsolidationSetResponse, error) {
	filter := domain.ConsolidationSetFilter{
		ParentEntityID: &parentEntityID,
	}

	sets, err := a.consolidationService.ListConsolidationSets(ctx, filter)
	if err != nil {
		return nil, err
	}

	response := make([]ConsolidationSetResponse, len(sets))
	for i, set := range sets {
		response[i] = *toConsolidationSetResponse(&set)
	}
	return response, nil
}

func (a *apiImpl) InitiateConsolidationRun(ctx context.Context, req InitiateRunRequest) (*ConsolidationRunResponse, error) {
	run, err := a.consolidationService.InitiateConsolidationRun(
		ctx,
		req.ConsolidationSetID,
		req.FiscalPeriodID,
		req.ConsolidationDate,
		req.ClosingRateDate,
	)
	if err != nil {
		return nil, err
	}

	if req.AverageRateDate != nil {
		run.SetAverageRateDate(*req.AverageRateDate)
	}

	return toConsolidationRunResponse(run), nil
}

func (a *apiImpl) ExecuteConsolidation(ctx context.Context, runID common.ID) error {
	return a.consolidationService.ExecuteConsolidation(ctx, runID)
}

func (a *apiImpl) PostConsolidation(ctx context.Context, runID common.ID) error {
	return a.consolidationService.PostConsolidation(ctx, runID)
}

func (a *apiImpl) GetConsolidationRun(ctx context.Context, runID common.ID) (*ConsolidationRunResponse, error) {
	run, err := a.consolidationService.GetConsolidationRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return toConsolidationRunResponse(run), nil
}

func (a *apiImpl) GetConsolidatedTrialBalance(ctx context.Context, runID common.ID) (*ConsolidatedTrialBalanceResponse, error) {
	run, err := a.consolidationService.GetConsolidationRun(ctx, runID)
	if err != nil {
		return nil, err
	}

	balances, err := a.consolidationService.GetConsolidatedTrialBalance(ctx, runID)
	if err != nil {
		return nil, err
	}

	accounts := make([]ConsolidatedAccountBalance, len(balances))
	totalAssets := money.Zero(run.ReportingCurrency)
	totalLiabilities := money.Zero(run.ReportingCurrency)
	totalEquity := money.Zero(run.ReportingCurrency)
	totalRevenue := money.Zero(run.ReportingCurrency)
	totalExpenses := money.Zero(run.ReportingCurrency)
	totalDebit := money.Zero(run.ReportingCurrency)
	totalCredit := money.Zero(run.ReportingCurrency)

	for i, balance := range balances {
		accounts[i] = ConsolidatedAccountBalance{
			AccountID:             balance.AccountID,
			AccountCode:           balance.AccountCode,
			AccountName:           balance.AccountName,
			AccountType:           balance.AccountType,
			OpeningBalance:        balance.OpeningBalance,
			PeriodDebit:           balance.PeriodDebit,
			PeriodCredit:          balance.PeriodCredit,
			EliminationDebit:      balance.EliminationDebit,
			EliminationCredit:     balance.EliminationCredit,
			TranslationAdjustment: balance.TranslationAdjustment,
			MinorityInterest:      balance.MinorityInterest,
			ClosingBalance:        balance.ClosingBalance,
		}

		switch balance.AccountType {
		case "asset":
			totalAssets = totalAssets.MustAdd(balance.ClosingBalance)
		case "liability":
			totalLiabilities = totalLiabilities.MustAdd(balance.ClosingBalance)
		case "equity":
			totalEquity = totalEquity.MustAdd(balance.ClosingBalance)
		case "revenue":
			totalRevenue = totalRevenue.MustAdd(balance.ClosingBalance)
		case "expense":
			totalExpenses = totalExpenses.MustAdd(balance.ClosingBalance)
		}

		totalDebit = totalDebit.MustAdd(balance.PeriodDebit)
		totalCredit = totalCredit.MustAdd(balance.PeriodCredit)
	}

	return &ConsolidatedTrialBalanceResponse{
		RunID:            runID,
		ReportingCurrency: run.ReportingCurrency.Code,
		Accounts:         accounts,
		TotalAssets:      totalAssets,
		TotalLiabilities: totalLiabilities,
		TotalEquity:      totalEquity,
		TotalRevenue:     totalRevenue,
		TotalExpenses:    totalExpenses,
		NetIncome:        totalRevenue.MustSubtract(totalExpenses),
		IsBalanced:       totalDebit.Equals(totalCredit),
	}, nil
}

func (a *apiImpl) GetExchangeRate(ctx context.Context, fromCurrency, toCurrency money.Currency, date time.Time) (*ExchangeRateResponse, error) {
	rate, err := a.translationService.GetExchangeRate(ctx, fromCurrency, toCurrency, date)
	if err != nil {
		return nil, err
	}

	return &ExchangeRateResponse{
		ID:             rate.ID,
		FromCurrency:   rate.FromCurrency.Code,
		ToCurrency:     rate.ToCurrency.Code,
		RateDate:       rate.RateDate,
		ClosingRate:    rate.ClosingRate,
		AverageRate:    rate.AverageRate,
		HistoricalRate: rate.HistoricalRate,
	}, nil
}

func (a *apiImpl) TranslateAmount(ctx context.Context, amount money.Money, toCurrency money.Currency, date time.Time) (money.Money, float64, error) {
	return a.translationService.TranslateAmount(ctx, amount, toCurrency, date, domain.RateTypeClosing)
}

func toConsolidationSetResponse(set *domain.ConsolidationSet) *ConsolidationSetResponse {
	resp := &ConsolidationSetResponse{
		ID:                       set.ID,
		SetCode:                  set.SetCode,
		SetName:                  set.SetName,
		Description:              set.Description,
		ParentEntityID:           set.ParentEntityID,
		ReportingCurrency:        set.ReportingCurrency.Code,
		DefaultTranslationMethod: string(set.DefaultTranslationMethod),
		IsActive:                 set.IsActive,
	}

	if len(set.Members) > 0 {
		resp.Members = make([]ConsolidationSetMemberResponse, len(set.Members))
		for i, member := range set.Members {
			resp.Members[i] = ConsolidationSetMemberResponse{
				ID:                  member.ID,
				EntityID:            member.EntityID,
				EntityCode:          member.EntityCode,
				EntityName:          member.EntityName,
				OwnershipPercent:    member.OwnershipPercent,
				MinorityPercent:     member.MinorityPercent(),
				ConsolidationMethod: string(member.ConsolidationMethod),
				FunctionalCurrency:  member.FunctionalCurrency.Code,
				IsActive:            member.IsActive,
			}
		}
	}

	return resp
}

func toConsolidationRunResponse(run *domain.ConsolidationRun) *ConsolidationRunResponse {
	return &ConsolidationRunResponse{
		ID:                    run.ID,
		RunNumber:             run.RunNumber,
		ConsolidationSetID:    run.ConsolidationSetID,
		FiscalPeriodID:        run.FiscalPeriodID,
		ReportingCurrency:     run.ReportingCurrency.Code,
		ConsolidationDate:     run.ConsolidationDate,
		ClosingRateDate:       run.ClosingRateDate,
		AverageRateDate:       run.AverageRateDate,
		EntityCount:           run.EntityCount,
		TotalAssets:           run.TotalAssets,
		TotalLiabilities:      run.TotalLiabilities,
		TotalEquity:           run.TotalEquity,
		TotalRevenue:          run.TotalRevenue,
		TotalExpenses:         run.TotalExpenses,
		NetIncome:             run.NetIncome,
		TotalCTA:              run.TotalCTA,
		TotalMinorityInterest: run.TotalMinorityInterest,
		Status:                string(run.Status),
		JournalEntryID:        run.JournalEntryID,
		CreatedAt:             run.CreatedAt,
		CompletedAt:           run.CompletedAt,
		PostedAt:              run.PostedAt,
	}
}
