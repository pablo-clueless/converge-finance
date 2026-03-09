package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type ReportHandler struct {
	*Handler
	accountRepo repository.AccountRepository
	journalRepo repository.JournalRepository
	periodRepo  repository.PeriodRepository
}

func NewReportHandler(
	logger *zap.Logger,
	accountRepo repository.AccountRepository,
	journalRepo repository.JournalRepository,
	periodRepo repository.PeriodRepository,
) *ReportHandler {
	return &ReportHandler{
		Handler:     NewHandler(logger),
		accountRepo: accountRepo,
		journalRepo: journalRepo,
		periodRepo:  periodRepo,
	}
}

func (h *ReportHandler) RegisterRoutes(r chi.Router) {
	r.Route("/reports", func(r chi.Router) {
		r.Get("/trial-balance", h.TrialBalance)
		r.Get("/account-activity", h.AccountActivity)
		r.Get("/balance-sheet", h.BalanceSheet)
		r.Get("/income-statement", h.IncomeStatement)
	})
}

type TrialBalanceLine struct {
	AccountID   string `json:"account_id"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	AccountType string `json:"account_type"`
	Debit       string `json:"debit"`
	Credit      string `json:"credit"`
	Currency    string `json:"currency"`
}

type TrialBalanceResponse struct {
	EntityID    string             `json:"entity_id"`
	PeriodID    string             `json:"period_id,omitempty"`
	AsOfDate    string             `json:"as_of_date"`
	Currency    string             `json:"currency"`
	Lines       []TrialBalanceLine `json:"lines"`
	TotalDebit  string             `json:"total_debit"`
	TotalCredit string             `json:"total_credit"`
	IsBalanced  bool               `json:"is_balanced"`
	GeneratedAt time.Time          `json:"generated_at"`
}

func (h *ReportHandler) TrialBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	var periodID common.ID
	periodIDStr := r.URL.Query().Get("period_id")
	if periodIDStr != "" {
		id, err := common.Parse(periodIDStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid period_id")
			return
		}
		periodID = id
	}

	filter := domain.AccountFilter{
		EntityID:  entityID,
		IsPosting: boolPtr(true),
		IsActive:  boolPtr(true),
	}

	accounts, err := h.accountRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list accounts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to generate trial balance")
		return
	}

	lines := make([]TrialBalanceLine, 0, len(accounts))
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero

	for _, account := range accounts {

		debit, credit, err := h.journalRepo.GetAccountActivity(ctx, account.ID, periodID)
		if err != nil {
			h.logger.Error("Failed to get account activity",
				zap.String("account_id", account.ID.String()),
				zap.Error(err))
			continue
		}

		if debit == 0 && credit == 0 {
			continue
		}

		debitDec := decimal.NewFromFloat(debit)
		creditDec := decimal.NewFromFloat(credit)

		lines = append(lines, TrialBalanceLine{
			AccountID:   account.ID.String(),
			AccountCode: account.Code,
			AccountName: account.Name,
			AccountType: string(account.Type),
			Debit:       debitDec.StringFixed(2),
			Credit:      creditDec.StringFixed(2),
			Currency:    account.Currency.Code,
		})

		totalDebit = totalDebit.Add(debitDec)
		totalCredit = totalCredit.Add(creditDec)
	}

	asOfDate := time.Now().Format("2006-01-02")
	if dateStr := r.URL.Query().Get("as_of_date"); dateStr != "" {
		if _, err := time.Parse("2006-01-02", dateStr); err == nil {
			asOfDate = dateStr
		}
	}

	resp := TrialBalanceResponse{
		EntityID:    entityID.String(),
		AsOfDate:    asOfDate,
		Currency:    "USD",
		Lines:       lines,
		TotalDebit:  totalDebit.StringFixed(2),
		TotalCredit: totalCredit.StringFixed(2),
		IsBalanced:  totalDebit.Equal(totalCredit),
		GeneratedAt: time.Now(),
	}

	if !periodID.IsZero() {
		resp.PeriodID = periodID.String()
	}

	respondJSON(w, http.StatusOK, resp)
}

type AccountActivityLine struct {
	Date          string `json:"date"`
	JournalID     string `json:"journal_id"`
	JournalNumber string `json:"journal_number"`
	Description   string `json:"description"`
	Debit         string `json:"debit"`
	Credit        string `json:"credit"`
	Balance       string `json:"balance"`
}

type AccountActivityResponse struct {
	AccountID      string                `json:"account_id"`
	AccountCode    string                `json:"account_code"`
	AccountName    string                `json:"account_name"`
	AccountType    string                `json:"account_type"`
	NormalBalance  string                `json:"normal_balance"`
	Currency       string                `json:"currency"`
	DateFrom       string                `json:"date_from"`
	DateTo         string                `json:"date_to"`
	OpeningBalance string                `json:"opening_balance"`
	ClosingBalance string                `json:"closing_balance"`
	TotalDebit     string                `json:"total_debit"`
	TotalCredit    string                `json:"total_credit"`
	Lines          []AccountActivityLine `json:"lines"`
	GeneratedAt    time.Time             `json:"generated_at"`
}

func (h *ReportHandler) AccountActivity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	accountIDStr := r.URL.Query().Get("account_id")
	if accountIDStr == "" {
		respondError(w, http.StatusBadRequest, "account_id query parameter is required")
		return
	}

	accountID, err := common.Parse(accountIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account_id")
		return
	}

	account, err := h.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		if common.IsNotFoundError(err) {
			respondError(w, http.StatusNotFound, "Account not found")
			return
		}
		h.logger.Error("Failed to get account", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get account activity")
		return
	}

	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")

	lineFilter := repository.JournalLineFilter{
		DateFrom:   &dateFrom,
		DateTo:     &dateTo,
		PostedOnly: true,
	}

	journalLines, err := h.journalRepo.GetLinesByAccount(ctx, accountID, lineFilter)
	if err != nil {
		h.logger.Error("Failed to get journal lines", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to get account activity")
		return
	}

	lines := make([]AccountActivityLine, 0, len(journalLines))
	totalDebit := decimal.Zero
	totalCredit := decimal.Zero
	balance := decimal.Zero

	for _, jl := range journalLines {
		debitAmt := jl.DebitAmount.Amount
		creditAmt := jl.CreditAmount.Amount

		if jl.IsDebit() {
			if account.NormalBalance == domain.BalanceTypeDebit {
				balance = balance.Add(debitAmt)
			} else {
				balance = balance.Sub(debitAmt)
			}
		} else {
			if account.NormalBalance == domain.BalanceTypeCredit {
				balance = balance.Add(creditAmt)
			} else {
				balance = balance.Sub(creditAmt)
			}
		}

		totalDebit = totalDebit.Add(debitAmt)
		totalCredit = totalCredit.Add(creditAmt)

		lines = append(lines, AccountActivityLine{
			Date:        jl.CreatedAt.Format("2006-01-02"),
			JournalID:   jl.JournalEntryID.String(),
			Description: jl.Description,
			Debit:       debitAmt.StringFixed(2),
			Credit:      creditAmt.StringFixed(2),
			Balance:     balance.StringFixed(2),
		})
	}

	resp := AccountActivityResponse{
		AccountID:      account.ID.String(),
		AccountCode:    account.Code,
		AccountName:    account.Name,
		AccountType:    string(account.Type),
		NormalBalance:  string(account.NormalBalance),
		Currency:       account.Currency.Code,
		DateFrom:       dateFrom,
		DateTo:         dateTo,
		OpeningBalance: "0.00",
		ClosingBalance: balance.StringFixed(2),
		TotalDebit:     totalDebit.StringFixed(2),
		TotalCredit:    totalCredit.StringFixed(2),
		Lines:          lines,
		GeneratedAt:    time.Now(),
	}

	respondJSON(w, http.StatusOK, resp)
}

type BalanceSheetSection struct {
	Title    string             `json:"title"`
	Accounts []BalanceSheetLine `json:"accounts"`
	Total    string             `json:"total"`
}

type BalanceSheetLine struct {
	AccountID   string `json:"account_id"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Balance     string `json:"balance"`
	Indent      int    `json:"indent"`
}

type BalanceSheetResponse struct {
	EntityID        string              `json:"entity_id"`
	AsOfDate        string              `json:"as_of_date"`
	Currency        string              `json:"currency"`
	Assets          BalanceSheetSection `json:"assets"`
	Liabilities     BalanceSheetSection `json:"liabilities"`
	Equity          BalanceSheetSection `json:"equity"`
	TotalAssets     string              `json:"total_assets"`
	TotalLiabEquity string              `json:"total_liabilities_and_equity"`
	IsBalanced      bool                `json:"is_balanced"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

func (h *ReportHandler) BalanceSheet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	asOfDate := r.URL.Query().Get("as_of_date")
	if asOfDate == "" {
		asOfDate = time.Now().Format("2006-01-02")
	}

	accounts, err := h.accountRepo.List(ctx, domain.AccountFilter{
		EntityID: entityID,
		IsActive: boolPtr(true),
	})
	if err != nil {
		h.logger.Error("Failed to list accounts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to generate balance sheet")
		return
	}

	assets := BalanceSheetSection{Title: "Assets", Accounts: []BalanceSheetLine{}}
	liabilities := BalanceSheetSection{Title: "Liabilities", Accounts: []BalanceSheetLine{}}
	equity := BalanceSheetSection{Title: "Equity", Accounts: []BalanceSheetLine{}}

	totalAssets := decimal.Zero
	totalLiabilities := decimal.Zero
	totalEquity := decimal.Zero

	var emptyPeriodID common.ID

	for _, account := range accounts {
		if !account.IsPosting {
			continue
		}

		debit, credit, err := h.journalRepo.GetAccountActivity(ctx, account.ID, emptyPeriodID)
		if err != nil {
			continue
		}

		debitDec := decimal.NewFromFloat(debit)
		creditDec := decimal.NewFromFloat(credit)

		var balance decimal.Decimal
		if account.NormalBalance == domain.BalanceTypeDebit {
			balance = debitDec.Sub(creditDec)
		} else {
			balance = creditDec.Sub(debitDec)
		}

		line := BalanceSheetLine{
			AccountID:   account.ID.String(),
			AccountCode: account.Code,
			AccountName: account.Name,
			Balance:     balance.StringFixed(2),
			Indent:      0,
		}

		switch account.Type {
		case domain.AccountTypeAsset:
			assets.Accounts = append(assets.Accounts, line)
			totalAssets = totalAssets.Add(balance)
		case domain.AccountTypeLiability:
			liabilities.Accounts = append(liabilities.Accounts, line)
			totalLiabilities = totalLiabilities.Add(balance)
		case domain.AccountTypeEquity:
			equity.Accounts = append(equity.Accounts, line)
			totalEquity = totalEquity.Add(balance)
		}
	}

	assets.Total = totalAssets.StringFixed(2)
	liabilities.Total = totalLiabilities.StringFixed(2)
	equity.Total = totalEquity.StringFixed(2)

	totalLiabEquity := totalLiabilities.Add(totalEquity)

	resp := BalanceSheetResponse{
		EntityID:        entityID.String(),
		AsOfDate:        asOfDate,
		Currency:        "USD",
		Assets:          assets,
		Liabilities:     liabilities,
		Equity:          equity,
		TotalAssets:     totalAssets.StringFixed(2),
		TotalLiabEquity: totalLiabEquity.StringFixed(2),
		IsBalanced:      totalAssets.Equal(totalLiabEquity),
		GeneratedAt:     time.Now(),
	}

	respondJSON(w, http.StatusOK, resp)
}

type IncomeStatementSection struct {
	Title    string                `json:"title"`
	Accounts []IncomeStatementLine `json:"accounts"`
	Total    string                `json:"total"`
}

type IncomeStatementLine struct {
	AccountID   string `json:"account_id"`
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	Amount      string `json:"amount"`
}

type IncomeStatementResponse struct {
	EntityID      string                 `json:"entity_id"`
	PeriodFrom    string                 `json:"period_from"`
	PeriodTo      string                 `json:"period_to"`
	Currency      string                 `json:"currency"`
	Revenue       IncomeStatementSection `json:"revenue"`
	Expenses      IncomeStatementSection `json:"expenses"`
	TotalRevenue  string                 `json:"total_revenue"`
	TotalExpenses string                 `json:"total_expenses"`
	NetIncome     string                 `json:"net_income"`
	GeneratedAt   time.Time              `json:"generated_at"`
}

func (h *ReportHandler) IncomeStatement(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := getEntityID(r)

	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	periodFrom := r.URL.Query().Get("period_from")
	periodTo := r.URL.Query().Get("period_to")
	if periodFrom == "" || periodTo == "" {

		now := time.Now()
		periodFrom = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		periodTo = now.Format("2006-01-02")
	}

	accounts, err := h.accountRepo.List(ctx, domain.AccountFilter{
		EntityID: entityID,
		IsActive: boolPtr(true),
	})
	if err != nil {
		h.logger.Error("Failed to list accounts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, "Failed to generate income statement")
		return
	}

	revenue := IncomeStatementSection{Title: "Revenue", Accounts: []IncomeStatementLine{}}
	expenses := IncomeStatementSection{Title: "Expenses", Accounts: []IncomeStatementLine{}}

	totalRevenue := decimal.Zero
	totalExpenses := decimal.Zero

	var emptyPeriodID common.ID

	for _, account := range accounts {
		if !account.IsPosting {
			continue
		}

		if account.Type != domain.AccountTypeRevenue && account.Type != domain.AccountTypeExpense {
			continue
		}

		debit, credit, err := h.journalRepo.GetAccountActivity(ctx, account.ID, emptyPeriodID)
		if err != nil {
			continue
		}

		debitDec := decimal.NewFromFloat(debit)
		creditDec := decimal.NewFromFloat(credit)

		var amount decimal.Decimal
		if account.Type == domain.AccountTypeRevenue {
			amount = creditDec.Sub(debitDec)
		} else {
			amount = debitDec.Sub(creditDec)
		}

		line := IncomeStatementLine{
			AccountID:   account.ID.String(),
			AccountCode: account.Code,
			AccountName: account.Name,
			Amount:      amount.StringFixed(2),
		}

		if account.Type == domain.AccountTypeRevenue {
			revenue.Accounts = append(revenue.Accounts, line)
			totalRevenue = totalRevenue.Add(amount)
		} else {
			expenses.Accounts = append(expenses.Accounts, line)
			totalExpenses = totalExpenses.Add(amount)
		}
	}

	revenue.Total = totalRevenue.StringFixed(2)
	expenses.Total = totalExpenses.StringFixed(2)

	netIncome := totalRevenue.Sub(totalExpenses)

	resp := IncomeStatementResponse{
		EntityID:      entityID.String(),
		PeriodFrom:    periodFrom,
		PeriodTo:      periodTo,
		Currency:      "USD",
		Revenue:       revenue,
		Expenses:      expenses,
		TotalRevenue:  totalRevenue.StringFixed(2),
		TotalExpenses: totalExpenses.StringFixed(2),
		NetIncome:     netIncome.StringFixed(2),
		GeneratedAt:   time.Now(),
	}

	respondJSON(w, http.StatusOK, resp)
}

func boolPtr(b bool) *bool {
	return &b
}
