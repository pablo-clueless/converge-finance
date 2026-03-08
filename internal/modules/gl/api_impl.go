package gl

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"converge-finance.com/m/internal/modules/gl/internal/service"
	"github.com/shopspring/decimal"
)

// glAPI implements the public GL API interface
type glAPI struct {
	postingEngine *service.PostingEngine
	journalRepo   repository.JournalRepository
	periodRepo    repository.PeriodRepository
	accountRepo   repository.AccountRepository
	balanceRepo   repository.AccountBalanceRepository
}

// NewGLAPI creates a new GL API implementation
func NewGLAPI(
	postingEngine *service.PostingEngine,
	journalRepo repository.JournalRepository,
	periodRepo repository.PeriodRepository,
	accountRepo repository.AccountRepository,
	balanceRepo repository.AccountBalanceRepository,
) API {
	return &glAPI{
		postingEngine: postingEngine,
		journalRepo:   journalRepo,
		periodRepo:    periodRepo,
		accountRepo:   accountRepo,
		balanceRepo:   balanceRepo,
	}
}

// CreateJournalEntry creates a new journal entry
func (a *glAPI) CreateJournalEntry(ctx context.Context, req CreateJournalEntryRequest) (*JournalEntryResponse, error) {
	// Get fiscal period for the date
	period, err := a.periodRepo.GetPeriodForDate(ctx, req.EntityID, req.EntryDate.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("no fiscal period found for date: %w", err)
	}

	if !period.CanPost() {
		return nil, fmt.Errorf("fiscal period is not open for posting")
	}

	// Get currency
	currency, err := money.GetCurrency(req.CurrencyCode)
	if err != nil {
		return nil, fmt.Errorf("invalid currency code: %w", err)
	}

	// Generate entry number
	entryNumber, err := a.journalRepo.GetNextEntryNumber(ctx, req.EntityID, "JE")
	if err != nil {
		return nil, fmt.Errorf("failed to generate entry number: %w", err)
	}

	// Create journal entry
	entry, err := domain.NewJournalEntry(
		req.EntityID,
		entryNumber,
		period.ID,
		req.EntryDate,
		req.Description,
		currency,
		common.ID(""), // No user ID in API context
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create journal entry: %w", err)
	}

	// Add lines
	for _, line := range req.Lines {
		if err := entry.AddLine(line.AccountID, line.Description, line.Debit, line.Credit); err != nil {
			return nil, fmt.Errorf("failed to add line: %w", err)
		}
	}

	// Validate
	if err := entry.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Save
	if err := a.journalRepo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to save journal entry: %w", err)
	}

	return toJournalEntryResponse(entry), nil
}

// PostJournalEntry posts a journal entry
func (a *glAPI) PostJournalEntry(ctx context.Context, entryID common.ID) error {
	return a.postingEngine.PostEntry(ctx, entryID)
}

// ReverseJournalEntry reverses a posted journal entry
func (a *glAPI) ReverseJournalEntry(ctx context.Context, entryID common.ID, reversalDate time.Time) (*JournalEntryResponse, error) {
	reversal, err := a.postingEngine.ReverseEntry(ctx, entryID, reversalDate.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	return toJournalEntryResponse(reversal), nil
}

// GetJournalEntry retrieves a journal entry by ID
func (a *glAPI) GetJournalEntry(ctx context.Context, entryID common.ID) (*JournalEntryResponse, error) {
	entry, err := a.journalRepo.GetByID(ctx, entryID)
	if err != nil {
		return nil, err
	}
	return toJournalEntryResponse(entry), nil
}

// GetAccountByID retrieves an account by ID
func (a *glAPI) GetAccountByID(ctx context.Context, accountID common.ID) (*AccountResponse, error) {
	account, err := a.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return toAccountResponse(account), nil
}

// GetAccountByCode retrieves an account by code
func (a *glAPI) GetAccountByCode(ctx context.Context, entityID common.ID, code string) (*AccountResponse, error) {
	account, err := a.accountRepo.GetByCode(ctx, entityID, code)
	if err != nil {
		return nil, err
	}
	return toAccountResponse(account), nil
}

// ListAccounts lists accounts with filtering
func (a *glAPI) ListAccounts(ctx context.Context, entityID common.ID, filter AccountFilterRequest) ([]AccountResponse, error) {
	domainFilter := domain.AccountFilter{
		EntityID:  entityID,
		IsActive:  filter.IsActive,
		IsPosting: filter.IsPosting,
		Limit:     filter.Limit,
		Offset:    filter.Offset,
	}

	if filter.Type != nil {
		t := domain.AccountType(*filter.Type)
		domainFilter.Type = &t
	}

	accounts, err := a.accountRepo.List(ctx, domainFilter)
	if err != nil {
		return nil, err
	}

	responses := make([]AccountResponse, len(accounts))
	for i, acc := range accounts {
		responses[i] = *toAccountResponse(&acc)
	}
	return responses, nil
}

// ValidatePeriodOpen validates that a period is open for posting
func (a *glAPI) ValidatePeriodOpen(ctx context.Context, entityID common.ID, date time.Time) error {
	period, err := a.periodRepo.GetPeriodForDate(ctx, entityID, date.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("no fiscal period found for date: %w", err)
	}

	if !period.CanPost() {
		return fmt.Errorf("fiscal period %s is not open for posting", period.PeriodName)
	}

	return nil
}

// GetFiscalPeriodForDate gets the fiscal period for a given date
func (a *glAPI) GetFiscalPeriodForDate(ctx context.Context, entityID common.ID, date time.Time) (*FiscalPeriodResponse, error) {
	period, err := a.periodRepo.GetPeriodForDate(ctx, entityID, date.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	return toFiscalPeriodResponse(period), nil
}

// GetOpenPeriods gets all open periods for an entity
func (a *glAPI) GetOpenPeriods(ctx context.Context, entityID common.ID) ([]FiscalPeriodResponse, error) {
	periods, err := a.periodRepo.GetOpenPeriods(ctx, entityID)
	if err != nil {
		return nil, err
	}

	responses := make([]FiscalPeriodResponse, len(periods))
	for i, p := range periods {
		responses[i] = *toFiscalPeriodResponse(&p)
	}
	return responses, nil
}

// GetAccountBalance gets the balance for an account in a period
func (a *glAPI) GetAccountBalance(ctx context.Context, accountID common.ID, periodID common.ID) (*AccountBalanceResponse, error) {
	account, err := a.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}

	debit, credit, err := a.journalRepo.GetAccountActivity(ctx, accountID, periodID)
	if err != nil {
		return nil, err
	}

	debitMoney := money.NewFromDecimal(decimal.NewFromFloat(debit), account.Currency)
	creditMoney := money.NewFromDecimal(decimal.NewFromFloat(credit), account.Currency)

	return &AccountBalanceResponse{
		AccountID:     account.ID,
		AccountCode:   account.Code,
		AccountName:   account.Name,
		PeriodID:      periodID,
		OpeningDebit:  money.Zero(account.Currency),
		OpeningCredit: money.Zero(account.Currency),
		PeriodDebit:   debitMoney,
		PeriodCredit:  creditMoney,
		ClosingDebit:  debitMoney,
		ClosingCredit: creditMoney,
	}, nil
}

// GetTrialBalance gets the trial balance for an entity in a period
func (a *glAPI) GetTrialBalance(ctx context.Context, entityID common.ID, periodID common.ID) (*TrialBalanceResponse, error) {
	// Get all posting accounts
	accounts, err := a.accountRepo.List(ctx, domain.AccountFilter{
		EntityID:  entityID,
		IsPosting: boolPtr(true),
		IsActive:  boolPtr(true),
	})
	if err != nil {
		return nil, err
	}

	lines := make([]TrialBalanceLineResponse, 0, len(accounts))
	totalDebit := money.Zero(money.MustGetCurrency("USD"))
	totalCredit := money.Zero(money.MustGetCurrency("USD"))

	for _, account := range accounts {
		debit, credit, err := a.journalRepo.GetAccountActivity(ctx, account.ID, periodID)
		if err != nil {
			continue
		}

		if debit == 0 && credit == 0 {
			continue
		}

		debitMoney := money.New(debit, account.Currency.Code)
		creditMoney := money.New(credit, account.Currency.Code)

		lines = append(lines, TrialBalanceLineResponse{
			AccountID:   account.ID,
			AccountCode: account.Code,
			AccountName: account.Name,
			AccountType: string(account.Type),
			Debit:       debitMoney,
			Credit:      creditMoney,
		})

		totalDebit = totalDebit.MustAdd(debitMoney)
		totalCredit = totalCredit.MustAdd(creditMoney)
	}

	return &TrialBalanceResponse{
		EntityID:    entityID,
		PeriodID:    periodID,
		AsOfDate:    time.Now(),
		Accounts:    lines,
		TotalDebit:  totalDebit,
		TotalCredit: totalCredit,
		IsBalanced:  totalDebit.Amount.Equal(totalCredit.Amount),
	}, nil
}

// Helper functions to convert domain objects to API responses
func toJournalEntryResponse(entry *domain.JournalEntry) *JournalEntryResponse {
	lines := make([]JournalLineResponse, len(entry.Lines))
	for i, line := range entry.Lines {
		lines[i] = JournalLineResponse{
			ID:           line.ID,
			LineNumber:   line.LineNumber,
			AccountID:    line.AccountID,
			Description:  line.Description,
			DebitAmount:  line.DebitAmount,
			CreditAmount: line.CreditAmount,
		}
	}

	return &JournalEntryResponse{
		ID:              entry.ID,
		EntityID:        entry.EntityID,
		EntryNumber:     entry.EntryNumber,
		FiscalPeriodID:  entry.FiscalPeriodID,
		EntryDate:       entry.EntryDate,
		PostingDate:     entry.PostingDate,
		Description:     entry.Description,
		Source:          string(entry.Source),
		SourceReference: entry.SourceReference,
		Status:          string(entry.Status),
		CurrencyCode:    entry.Currency.Code,
		TotalDebit:      entry.TotalDebits(),
		TotalCredit:     entry.TotalCredits(),
		Lines:           lines,
		CreatedAt:       entry.CreatedAt,
		PostedAt:        entry.PostedAt,
	}
}

func toAccountResponse(account *domain.Account) *AccountResponse {
	resp := &AccountResponse{
		ID:            account.ID,
		EntityID:      account.EntityID,
		Code:          account.Code,
		Name:          account.Name,
		Type:          string(account.Type),
		Subtype:       string(account.Subtype),
		CurrencyCode:  account.Currency.Code,
		IsControl:     account.IsControl,
		IsPosting:     account.IsPosting,
		IsActive:      account.IsActive,
		NormalBalance: string(account.NormalBalance),
	}

	if account.ParentID != nil {
		resp.ParentID = account.ParentID
	}

	return resp
}

func toFiscalPeriodResponse(period *domain.FiscalPeriod) *FiscalPeriodResponse {
	return &FiscalPeriodResponse{
		ID:           period.ID,
		EntityID:     period.EntityID,
		FiscalYearID: period.FiscalYearID,
		PeriodNumber: period.PeriodNumber,
		PeriodName:   period.PeriodName,
		StartDate:    period.StartDate,
		EndDate:      period.EndDate,
		Status:       string(period.Status),
		IsAdjustment: period.IsAdjustment,
	}
}

// Helper function
func boolPtr(b bool) *bool {
	return &b
}
