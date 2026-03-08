package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type JournalLine struct {
	ID             common.ID
	JournalEntryID common.ID
	LineNumber     int
	AccountID      common.ID
	Account        *Account
	Description    string
	DebitAmount    money.Money
	CreditAmount   money.Money
	BaseDebit      money.Money
	BaseCredit     money.Money
	CreatedAt      time.Time
}

func (jl JournalLine) IsDebit() bool {
	return !jl.DebitAmount.IsZero()
}

func (jl JournalLine) IsCredit() bool {
	return !jl.CreditAmount.IsZero()
}

func (jl JournalLine) Amount() money.Money {
	if jl.IsDebit() {
		return jl.DebitAmount
	}
	return jl.CreditAmount
}

func (jl JournalLine) BaseAmount() money.Money {
	if jl.IsDebit() {
		return jl.BaseDebit
	}
	return jl.BaseCredit
}

func (jl JournalLine) SignedAmount() money.Money {
	if jl.IsDebit() {
		return jl.DebitAmount
	}
	return jl.CreditAmount.Negate()
}

func (jl JournalLine) BalanceImpact(normalBalance BalanceType) money.Money {
	if normalBalance == BalanceTypeDebit {

		if jl.IsDebit() {
			return jl.DebitAmount
		}
		return jl.CreditAmount.Negate()
	}

	if jl.IsCredit() {
		return jl.CreditAmount
	}
	return jl.DebitAmount.Negate()
}

func (jl JournalLine) Validate() error {
	ve := common.NewValidationError()

	if jl.AccountID.IsZero() {
		ve.Add("account_id", "required", "Account ID is required")
	}

	if jl.DebitAmount.IsZero() && jl.CreditAmount.IsZero() {
		ve.Add("amount", "required", "Either debit or credit amount is required")
	}

	if !jl.DebitAmount.IsZero() && !jl.CreditAmount.IsZero() {
		ve.Add("amount", "invalid", "Cannot have both debit and credit amounts")
	}

	if !jl.DebitAmount.IsZero() && jl.DebitAmount.IsNegative() {
		ve.Add("debit_amount", "invalid", "Debit amount cannot be negative")
	}

	if !jl.CreditAmount.IsZero() && jl.CreditAmount.IsNegative() {
		ve.Add("credit_amount", "invalid", "Credit amount cannot be negative")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (jl JournalLine) Clone() JournalLine {
	return JournalLine{
		ID:             common.NewID(),
		JournalEntryID: jl.JournalEntryID,
		LineNumber:     jl.LineNumber,
		AccountID:      jl.AccountID,
		Description:    jl.Description,
		DebitAmount:    jl.DebitAmount,
		CreditAmount:   jl.CreditAmount,
		BaseDebit:      jl.BaseDebit,
		BaseCredit:     jl.BaseCredit,
		CreatedAt:      time.Now(),
	}
}
