package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
	"converge-finance.com/m/internal/modules/consol/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/modules/ic"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
)

type ConsolidationService struct {
	db                 *database.PostgresDB
	setRepo            repository.ConsolidationSetRepository
	runRepo            repository.ConsolidationRunRepository
	entityBalanceRepo  repository.EntityBalanceRepository
	consolidatedRepo   repository.ConsolidatedBalanceRepository
	minorityRepo       repository.MinorityInterestRepository
	rateRepo           repository.ExchangeRateRepository
	translationAdjRepo repository.TranslationAdjustmentRepository
	glAPI              gl.API
	icAPI              ic.API
	auditLogger        *audit.Logger
}

func NewConsolidationService(
	db *database.PostgresDB,
	setRepo repository.ConsolidationSetRepository,
	runRepo repository.ConsolidationRunRepository,
	entityBalanceRepo repository.EntityBalanceRepository,
	consolidatedRepo repository.ConsolidatedBalanceRepository,
	minorityRepo repository.MinorityInterestRepository,
	rateRepo repository.ExchangeRateRepository,
	translationAdjRepo repository.TranslationAdjustmentRepository,
	glAPI gl.API,
	icAPI ic.API,
	auditLogger *audit.Logger,
) *ConsolidationService {
	return &ConsolidationService{
		db:                 db,
		setRepo:            setRepo,
		runRepo:            runRepo,
		entityBalanceRepo:  entityBalanceRepo,
		consolidatedRepo:   consolidatedRepo,
		minorityRepo:       minorityRepo,
		rateRepo:           rateRepo,
		translationAdjRepo: translationAdjRepo,
		glAPI:              glAPI,
		icAPI:              icAPI,
		auditLogger:        auditLogger,
	}
}

func (s *ConsolidationService) CreateConsolidationSet(
	ctx context.Context,
	setCode string,
	setName string,
	parentEntityID common.ID,
	reportingCurrency money.Currency,
) (*domain.ConsolidationSet, error) {
	set, err := domain.NewConsolidationSet(setCode, setName, parentEntityID, reportingCurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to create consolidation set: %w", err)
	}

	if err := s.setRepo.Create(ctx, set); err != nil {
		return nil, fmt.Errorf("failed to save consolidation set: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "consol.set", set.ID, "created", map[string]any{
			"set_code":           set.SetCode,
			"set_name":           set.SetName,
			"reporting_currency": set.ReportingCurrency.Code,
		})
	}

	return set, nil
}

func (s *ConsolidationService) AddMemberToSet(
	ctx context.Context,
	setID common.ID,
	entityID common.ID,
	ownershipPercent float64,
	consolidationMethod domain.ConsolidationMethod,
	functionalCurrency money.Currency,
) (*domain.ConsolidationSetMember, error) {
	set, err := s.setRepo.GetByID(ctx, setID)
	if err != nil {
		return nil, fmt.Errorf("consolidation set not found: %w", err)
	}

	member, err := domain.NewConsolidationSetMember(entityID, ownershipPercent, consolidationMethod, functionalCurrency)
	if err != nil {
		return nil, fmt.Errorf("invalid member: %w", err)
	}

	member.ConsolidationSetID = set.ID

	if err := s.setRepo.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "consol.set_member", member.ID, "added", map[string]any{
			"set_id":               setID,
			"entity_id":            entityID,
			"ownership_percent":    ownershipPercent,
			"consolidation_method": consolidationMethod,
		})
	}

	return member, nil
}

func (s *ConsolidationService) InitiateConsolidationRun(
	ctx context.Context,
	setID common.ID,
	fiscalPeriodID common.ID,
	consolidationDate time.Time,
	closingRateDate time.Time,
) (*domain.ConsolidationRun, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	set, err := s.setRepo.GetByID(ctx, setID)
	if err != nil {
		return nil, fmt.Errorf("consolidation set not found: %w", err)
	}

	runNumber, err := s.runRepo.GetNextRunNumber(ctx, setID)
	if err != nil {
		return nil, fmt.Errorf("failed to get run number: %w", err)
	}

	run, err := domain.NewConsolidationRun(
		runNumber,
		setID,
		fiscalPeriodID,
		set.ReportingCurrency,
		consolidationDate,
		closingRateDate,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create consolidation run: %w", err)
	}

	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save consolidation run: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "consol.run", run.ID, "initiated", map[string]any{
			"run_number":         run.RunNumber,
			"set_id":             setID,
			"fiscal_period_id":   fiscalPeriodID,
			"consolidation_date": consolidationDate,
		})
	}

	return run, nil
}

func (s *ConsolidationService) ExecuteConsolidation(ctx context.Context, runID common.ID) error {
	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("consolidation run not found: %w", err)
	}

	if err := run.StartProcessing(); err != nil {
		return fmt.Errorf("cannot start processing: %w", err)
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}

	set, err := s.setRepo.GetByID(ctx, run.ConsolidationSetID)
	if err != nil {
		return fmt.Errorf("consolidation set not found: %w", err)
	}

	members, err := s.setRepo.GetMembers(ctx, run.ConsolidationSetID)
	if err != nil {
		return fmt.Errorf("failed to get set members: %w", err)
	}

	accountBalances := make(map[common.ID]*domain.ConsolidatedBalance)

	for _, member := range members {
		if !member.IsActive {
			continue
		}

		if err := s.processEntityBalances(ctx, run, set, &member, accountBalances); err != nil {
			return fmt.Errorf("failed to process entity %s: %w", member.EntityID, err)
		}

		if member.HasMinorityInterest() {
			if err := s.calculateMinorityInterest(ctx, run, &member); err != nil {
				return fmt.Errorf("failed to calculate minority interest for entity %s: %w", member.EntityID, err)
			}
		}
	}

	if err := s.applyEliminations(ctx, run, accountBalances); err != nil {
		return fmt.Errorf("failed to apply eliminations: %w", err)
	}

	for _, balance := range accountBalances {
		balance.CalculateClosingBalance()
		if err := s.consolidatedRepo.Create(ctx, balance); err != nil {
			return fmt.Errorf("failed to save consolidated balance: %w", err)
		}
	}

	if err := s.updateRunTotals(ctx, run, accountBalances, len(members)); err != nil {
		return fmt.Errorf("failed to update run totals: %w", err)
	}

	userID := auth.GetUserIDFromContext(ctx)
	if err := run.Complete(common.ID(userID)); err != nil {
		return fmt.Errorf("failed to complete run: %w", err)
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "consol.run", run.ID, "completed", map[string]any{
			"entity_count": run.EntityCount,
			"total_assets": run.TotalAssets,
			"net_income":   run.NetIncome,
		})
	}

	return nil
}

func (s *ConsolidationService) processEntityBalances(
	ctx context.Context,
	run *domain.ConsolidationRun,
	set *domain.ConsolidationSet,
	member *domain.ConsolidationSetMember,
	accountBalances map[common.ID]*domain.ConsolidatedBalance,
) error {
	trialBalance, err := s.glAPI.GetTrialBalance(ctx, member.EntityID, run.FiscalPeriodID)
	if err != nil {
		return fmt.Errorf("failed to get trial balance: %w", err)
	}

	translationMethod := member.GetTranslationMethod(set.DefaultTranslationMethod)

	for _, line := range trialBalance.Accounts {
		rateType := s.getRateTypeForAccount(line.AccountType, translationMethod)

		rate, err := s.rateRepo.GetClosingRate(ctx, member.FunctionalCurrency, set.ReportingCurrency, run.ClosingRateDate)
		if err != nil {
			rate = 1.0
		}

		if rateType == domain.RateTypeAverage && run.AverageRateDate != nil {
			avgRate, err := s.rateRepo.GetAverageRate(ctx, member.FunctionalCurrency, set.ReportingCurrency, *run.AverageRateDate)
			if err == nil {
				rate = avgRate
			}
		}

		entityBalance := domain.NewEntityBalance(
			run.ID,
			member.EntityID,
			line.AccountID,
			member.FunctionalCurrency,
			set.ReportingCurrency,
		)
		entityBalance.SetFunctionalAmounts(line.Debit, line.Credit)
		entityBalance.Translate(rate, rateType, set.ReportingCurrency)
		entityBalance.AccountCode = line.AccountCode
		entityBalance.AccountName = line.AccountName
		entityBalance.AccountType = line.AccountType

		if err := s.entityBalanceRepo.Create(ctx, entityBalance); err != nil {
			return fmt.Errorf("failed to save entity balance: %w", err)
		}

		targetAccountID := line.AccountID
		mappings, _ := s.setRepo.GetAccountMappings(ctx, set.ID, member.EntityID)
		for _, mapping := range mappings {
			if mapping.SourceAccountID == line.AccountID {
				targetAccountID = mapping.TargetAccountID
				break
			}
		}

		if _, exists := accountBalances[targetAccountID]; !exists {
			accountBalances[targetAccountID] = domain.NewConsolidatedBalance(
				run.ID,
				targetAccountID,
				set.ReportingCurrency,
			)
			accountBalances[targetAccountID].AccountCode = line.AccountCode
			accountBalances[targetAccountID].AccountName = line.AccountName
			accountBalances[targetAccountID].AccountType = line.AccountType
		}

		accountBalances[targetAccountID].AddEntityBalance(*entityBalance)
	}

	return nil
}

func (s *ConsolidationService) getRateTypeForAccount(accountType string, method domain.TranslationMethod) domain.RateType {
	switch method {
	case domain.TranslationMethodCurrentRate:
		return domain.RateTypeClosing

	case domain.TranslationMethodTemporal:
		switch accountType {
		case "asset", "liability":
			return domain.RateTypeClosing
		case "equity":
			return domain.RateTypeHistorical
		case "revenue", "expense":
			return domain.RateTypeAverage
		default:
			return domain.RateTypeClosing
		}

	case domain.TranslationMethodMonetaryNonMonetary:
		return domain.RateTypeClosing

	default:
		return domain.RateTypeClosing
	}
}

func (s *ConsolidationService) calculateMinorityInterest(
	ctx context.Context,
	run *domain.ConsolidationRun,
	member *domain.ConsolidationSetMember,
) error {
	entityBalances, err := s.entityBalanceRepo.GetByRunAndEntity(ctx, run.ID, member.EntityID)
	if err != nil {
		return fmt.Errorf("failed to get entity balances: %w", err)
	}

	totalEquity := money.Zero(run.ReportingCurrency)
	netIncome := money.Zero(run.ReportingCurrency)

	for _, balance := range entityBalances {
		switch balance.AccountType {
		case "equity":
			totalEquity = totalEquity.MustAdd(balance.TranslatedBalance)
		case "revenue":
			netIncome = netIncome.MustAdd(balance.TranslatedBalance)
		case "expense":
			netIncome = netIncome.MustSubtract(balance.TranslatedBalance)
		}
	}

	mi := domain.NewMinorityInterest(run.ID, member.EntityID, member.OwnershipPercent, run.ReportingCurrency)
	mi.Calculate(totalEquity, netIncome, money.Zero(run.ReportingCurrency))

	if err := s.minorityRepo.Create(ctx, mi); err != nil {
		return fmt.Errorf("failed to save minority interest: %w", err)
	}

	return nil
}

func (s *ConsolidationService) applyEliminations(
	ctx context.Context,
	run *domain.ConsolidationRun,
	accountBalances map[common.ID]*domain.ConsolidatedBalance,
) error {
	set, err := s.setRepo.GetByID(ctx, run.ConsolidationSetID)
	if err != nil {
		return err
	}

	elimRun, err := s.icAPI.GetEliminationRun(ctx, run.FiscalPeriodID)
	if err != nil {
		return nil
	}

	if elimRun == nil || elimRun.Status != "posted" {
		return nil
	}

	_ = set
	_ = accountBalances

	return nil
}

func (s *ConsolidationService) updateRunTotals(
	ctx context.Context,
	run *domain.ConsolidationRun,
	accountBalances map[common.ID]*domain.ConsolidatedBalance,
	entityCount int,
) error {
	totalAssets := money.Zero(run.ReportingCurrency)
	totalLiabilities := money.Zero(run.ReportingCurrency)
	totalEquity := money.Zero(run.ReportingCurrency)
	totalRevenue := money.Zero(run.ReportingCurrency)
	totalExpenses := money.Zero(run.ReportingCurrency)

	for _, balance := range accountBalances {
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
	}

	miList, err := s.minorityRepo.GetByRun(ctx, run.ID)
	if err != nil {
		return err
	}

	totalMI := money.Zero(run.ReportingCurrency)
	for _, mi := range miList {
		totalMI = totalMI.MustAdd(mi.ClosingNCI)
	}

	adjList, err := s.translationAdjRepo.GetByRunAndType(ctx, run.ID, domain.AdjustmentTypeCTA)
	if err != nil {
		return err
	}

	totalCTA := money.Zero(run.ReportingCurrency)
	for _, adj := range adjList {
		if adj.IsDebit() {
			totalCTA = totalCTA.MustAdd(adj.DebitAmount)
		} else {
			totalCTA = totalCTA.MustSubtract(adj.CreditAmount)
		}
	}

	run.UpdateTotals(entityCount, totalAssets, totalLiabilities, totalEquity, totalRevenue, totalExpenses, totalCTA, totalMI)

	return nil
}

func (s *ConsolidationService) PostConsolidation(ctx context.Context, runID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("consolidation run not found: %w", err)
	}

	if run.Status != domain.RunStatusCompleted {
		return fmt.Errorf("can only post completed runs")
	}

	set, err := s.setRepo.GetByID(ctx, run.ConsolidationSetID)
	if err != nil {
		return fmt.Errorf("consolidation set not found: %w", err)
	}

	consolidatedBalances, err := s.consolidatedRepo.GetByRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to get consolidated balances: %w", err)
	}

	var lines []gl.JournalLineRequest
	for _, balance := range consolidatedBalances {
		if balance.ClosingBalance.IsZero() {
			continue
		}

		if balance.ClosingBalance.IsPositive() {
			lines = append(lines, gl.JournalLineRequest{
				AccountID:   balance.AccountID,
				Description: fmt.Sprintf("Consolidation: %s", balance.AccountName),
				Debit:       balance.ClosingBalance,
				Credit:      money.Zero(run.ReportingCurrency),
			})
		} else {
			lines = append(lines, gl.JournalLineRequest{
				AccountID:   balance.AccountID,
				Description: fmt.Sprintf("Consolidation: %s", balance.AccountName),
				Debit:       money.Zero(run.ReportingCurrency),
				Credit:      balance.ClosingBalance.Negate(),
			})
		}
	}

	jeReq := gl.CreateJournalEntryRequest{
		EntityID:     set.ParentEntityID,
		EntryDate:    run.ConsolidationDate,
		Description:  fmt.Sprintf("Consolidation Run %s", run.RunNumber),
		CurrencyCode: run.ReportingCurrency.Code,
		Lines:        lines,
	}

	je, err := s.glAPI.CreateJournalEntry(ctx, jeReq)
	if err != nil {
		return fmt.Errorf("failed to create journal entry: %w", err)
	}

	if err := s.glAPI.PostJournalEntry(ctx, je.ID); err != nil {
		return fmt.Errorf("failed to post journal entry: %w", err)
	}

	if err := run.Post(common.ID(userID), je.ID); err != nil {
		return fmt.Errorf("failed to mark run as posted: %w", err)
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "consol.run", run.ID, "posted", map[string]any{
			"journal_entry_id": je.ID,
		})
	}

	return nil
}

func (s *ConsolidationService) ReverseConsolidation(ctx context.Context, runID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("consolidation run not found: %w", err)
	}

	if run.JournalEntryID != nil {
		_, err := s.glAPI.ReverseJournalEntry(ctx, *run.JournalEntryID, time.Now())
		if err != nil {
			return fmt.Errorf("failed to reverse journal entry: %w", err)
		}
	}

	if err := run.Reverse(common.ID(userID)); err != nil {
		return fmt.Errorf("failed to mark run as reversed: %w", err)
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "consol.run", run.ID, "reversed", nil)
	}

	return nil
}

func (s *ConsolidationService) GetConsolidationSet(ctx context.Context, id common.ID) (*domain.ConsolidationSet, error) {
	set, err := s.setRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("consolidation set not found: %w", err)
	}

	members, err := s.setRepo.GetMembers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get members: %w", err)
	}
	set.Members = members

	return set, nil
}

func (s *ConsolidationService) ListConsolidationSets(
	ctx context.Context,
	filter domain.ConsolidationSetFilter,
) ([]domain.ConsolidationSet, error) {
	return s.setRepo.List(ctx, filter)
}

func (s *ConsolidationService) GetConsolidationRun(ctx context.Context, id common.ID) (*domain.ConsolidationRun, error) {
	return s.runRepo.GetByID(ctx, id)
}

func (s *ConsolidationService) ListConsolidationRuns(
	ctx context.Context,
	filter domain.ConsolidationRunFilter,
) ([]domain.ConsolidationRun, error) {
	return s.runRepo.List(ctx, filter)
}

func (s *ConsolidationService) GetConsolidatedTrialBalance(
	ctx context.Context,
	runID common.ID,
) ([]domain.ConsolidatedBalance, error) {
	return s.consolidatedRepo.GetTrialBalance(ctx, runID)
}
