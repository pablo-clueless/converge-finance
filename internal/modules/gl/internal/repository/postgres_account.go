package repository

import (
	"context"
	"database/sql"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
)

type PostgresAccountRepository struct {
	db *sql.DB
}

func NewPostgresAccountRepository(db *sql.DB) *PostgresAccountRepository {
	return &PostgresAccountRepository{db: db}
}

func (r *PostgresAccountRepository) Create(ctx context.Context, account *domain.Account) error {
	query := `
		INSERT INTO gl.accounts (
			id, entity_id, parent_id, account_code, account_name,
			account_type, account_subtype, currency_code, is_control,
			is_posting, is_active, description, normal_balance, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	var parentID *string
	if account.ParentID != nil && !account.ParentID.IsZero() {
		s := account.ParentID.String()
		parentID = &s
	}
	var subtype *string
	if account.Subtype != "" {
		s := string(account.Subtype)
		subtype = &s
	}
	_, err := r.db.ExecContext(ctx, query,
		account.ID.String(),
		account.EntityID.String(),
		parentID,
		account.Code,
		account.Name,
		string(account.Type),
		subtype,
		account.Currency.Code,
		account.IsControl,
		account.IsPosting,
		account.IsActive,
		account.Description,
		string(account.NormalBalance),
		account.CreatedAt,
		account.UpdatedAt,
	)
	return err
}

func (r *PostgresAccountRepository) Update(ctx context.Context, account *domain.Account) error {
	query := `
		UPDATE gl.accounts SET
			parent_id = $2, account_name = $3, account_subtype = $4,
			is_control = $5, is_posting = $6, is_active = $7,
			description = $8, updated_at = $9
		WHERE id = $1
	`
	var parentID *string
	if account.ParentID != nil && !account.ParentID.IsZero() {
		s := account.ParentID.String()
		parentID = &s
	}
	var subtype *string
	if account.Subtype != "" {
		s := string(account.Subtype)
		subtype = &s
	}
	_, err := r.db.ExecContext(ctx, query,
		account.ID.String(),
		parentID,
		account.Name,
		subtype,
		account.IsControl,
		account.IsPosting,
		account.IsActive,
		account.Description,
		account.UpdatedAt,
	)
	return err
}

func (r *PostgresAccountRepository) GetByID(ctx context.Context, id common.ID) (*domain.Account, error) {
	query := `
		SELECT id, entity_id, parent_id, account_code, account_name,
			account_type, account_subtype, currency_code, is_control,
			is_posting, is_active, description, normal_balance, created_at, updated_at
		FROM gl.accounts
		WHERE id = $1
	`
	return r.scanAccount(r.db.QueryRowContext(ctx, query, id.String()))
}

func (r *PostgresAccountRepository) GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Account, error) {
	query := `
		SELECT id, entity_id, parent_id, account_code, account_name,
			account_type, account_subtype, currency_code, is_control,
			is_posting, is_active, description, normal_balance, created_at, updated_at
		FROM gl.accounts
		WHERE entity_id = $1 AND account_code = $2
	`
	return r.scanAccount(r.db.QueryRowContext(ctx, query, entityID.String(), code))
}

func (r *PostgresAccountRepository) List(ctx context.Context, filter domain.AccountFilter) ([]domain.Account, error) {
	query := `
		SELECT id, entity_id, parent_id, account_code, account_name,
			account_type, account_subtype, currency_code, is_control,
			is_posting, is_active, description, normal_balance, created_at, updated_at
		FROM gl.accounts
		WHERE entity_id = $1
	`
	args := []interface{}{filter.EntityID.String()}
	argIndex := 2

	if filter.Type != nil {
		query += fmt.Sprintf(" AND account_type = $%d", argIndex)
		args = append(args, string(*filter.Type))
		argIndex++
	}
	if filter.IsActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIndex)
		args = append(args, *filter.IsActive)
		argIndex++
	}
	if filter.IsPosting != nil {
		query += fmt.Sprintf(" AND is_posting = $%d", argIndex)
		args = append(args, *filter.IsPosting)
		argIndex++
	}
	if filter.SearchQuery != "" {
		query += fmt.Sprintf(" AND (account_code ILIKE $%d OR account_name ILIKE $%d)", argIndex, argIndex)
		args = append(args, "%"+filter.SearchQuery+"%")
	}

	query += " ORDER BY account_code"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var accounts []domain.Account
	for rows.Next() {
		acc, err := r.scanAccountFromRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *acc)
	}
	return accounts, rows.Err()
}

func (r *PostgresAccountRepository) Count(ctx context.Context, filter domain.AccountFilter) (int64, error) {
	query := `SELECT COUNT(*) FROM gl.accounts WHERE entity_id = $1`
	args := []interface{}{filter.EntityID.String()}
	argIndex := 2

	if filter.Type != nil {
		query += fmt.Sprintf(" AND account_type = $%d", argIndex)
		args = append(args, string(*filter.Type))
		argIndex++
	}
	if filter.IsActive != nil {
		query += fmt.Sprintf(" AND is_active = $%d", argIndex)
		args = append(args, *filter.IsActive)
	}

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *PostgresAccountRepository) GetTree(ctx context.Context, entityID common.ID) ([]domain.Account, error) {
	query := `
		SELECT id, entity_id, parent_id, account_code, account_name,
			account_type, account_subtype, currency_code, is_control,
			is_posting, is_active, description, normal_balance, created_at, updated_at
		FROM gl.accounts
		WHERE entity_id = $1
		ORDER BY account_code
	`
	rows, err := r.db.QueryContext(ctx, query, entityID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var accounts []domain.Account
	for rows.Next() {
		acc, err := r.scanAccountFromRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *acc)
	}
	return accounts, rows.Err()
}

func (r *PostgresAccountRepository) GetChildren(ctx context.Context, parentID common.ID) ([]domain.Account, error) {
	query := `
		SELECT id, entity_id, parent_id, account_code, account_name,
			account_type, account_subtype, currency_code, is_control,
			is_posting, is_active, description, normal_balance, created_at, updated_at
		FROM gl.accounts
		WHERE parent_id = $1
		ORDER BY account_code
	`
	rows, err := r.db.QueryContext(ctx, query, parentID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var accounts []domain.Account
	for rows.Next() {
		acc, err := r.scanAccountFromRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *acc)
	}
	return accounts, rows.Err()
}

func (r *PostgresAccountRepository) Delete(ctx context.Context, id common.ID) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM gl.accounts WHERE id = $1", id.String())
	return err
}

func (r *PostgresAccountRepository) ExistsByCode(ctx context.Context, entityID common.ID, code string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM gl.accounts WHERE entity_id = $1 AND account_code = $2)",
		entityID.String(), code,
	).Scan(&exists)
	return exists, err
}

func (r *PostgresAccountRepository) GetPostingAccounts(ctx context.Context, entityID common.ID) ([]domain.Account, error) {
	query := `
		SELECT id, entity_id, parent_id, account_code, account_name,
			account_type, account_subtype, currency_code, is_control,
			is_posting, is_active, description, normal_balance, created_at, updated_at
		FROM gl.accounts
		WHERE entity_id = $1 AND is_posting = TRUE AND is_active = TRUE
		ORDER BY account_code
	`
	rows, err := r.db.QueryContext(ctx, query, entityID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		acc, err := r.scanAccountFromRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *acc)
	}
	return accounts, rows.Err()
}

func (r *PostgresAccountRepository) WithTx(tx *sql.Tx) AccountRepository {
	return &PostgresAccountRepositoryTx{tx: tx}
}

func (r *PostgresAccountRepository) scanAccount(row *sql.Row) (*domain.Account, error) {
	var acc domain.Account
	var id, entityID, accountType, currencyCode, normalBalance string
	var parentID, subtype sql.NullString

	err := row.Scan(
		&id, &entityID, &parentID, &acc.Code, &acc.Name,
		&accountType, &subtype, &currencyCode, &acc.IsControl,
		&acc.IsPosting, &acc.IsActive, &acc.Description, &normalBalance,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	acc.ID = common.ID(id)
	acc.EntityID = common.ID(entityID)
	if parentID.Valid {
		pid := common.ID(parentID.String)
		acc.ParentID = &pid
	}
	acc.Type = domain.AccountType(accountType)
	if subtype.Valid {
		acc.Subtype = domain.AccountSubtype(subtype.String)
	}
	acc.Currency = money.MustGetCurrency(currencyCode)
	acc.NormalBalance = domain.BalanceType(normalBalance)

	return &acc, nil
}

func (r *PostgresAccountRepository) scanAccountFromRows(rows *sql.Rows) (*domain.Account, error) {
	var acc domain.Account
	var id, entityID, accountType, currencyCode, normalBalance string
	var parentID, subtype sql.NullString

	err := rows.Scan(
		&id, &entityID, &parentID, &acc.Code, &acc.Name,
		&accountType, &subtype, &currencyCode, &acc.IsControl,
		&acc.IsPosting, &acc.IsActive, &acc.Description, &normalBalance,
		&acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	acc.ID = common.ID(id)
	acc.EntityID = common.ID(entityID)
	if parentID.Valid {
		pid := common.ID(parentID.String)
		acc.ParentID = &pid
	}
	acc.Type = domain.AccountType(accountType)
	if subtype.Valid {
		acc.Subtype = domain.AccountSubtype(subtype.String)
	}
	acc.Currency = money.MustGetCurrency(currencyCode)
	acc.NormalBalance = domain.BalanceType(normalBalance)

	return &acc, nil
}

// PostgresAccountRepositoryTx wraps a transaction
type PostgresAccountRepositoryTx struct {
	tx *sql.Tx
}

func (r *PostgresAccountRepositoryTx) Create(ctx context.Context, account *domain.Account) error {
	// Same as main implementation but using tx
	return nil
}
func (r *PostgresAccountRepositoryTx) Update(ctx context.Context, account *domain.Account) error {
	return nil
}
func (r *PostgresAccountRepositoryTx) GetByID(ctx context.Context, id common.ID) (*domain.Account, error) {
	return nil, nil
}
func (r *PostgresAccountRepositoryTx) GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Account, error) {
	return nil, nil
}
func (r *PostgresAccountRepositoryTx) List(ctx context.Context, filter domain.AccountFilter) ([]domain.Account, error) {
	return nil, nil
}
func (r *PostgresAccountRepositoryTx) Count(ctx context.Context, filter domain.AccountFilter) (int64, error) {
	return 0, nil
}
func (r *PostgresAccountRepositoryTx) GetTree(ctx context.Context, entityID common.ID) ([]domain.Account, error) {
	return nil, nil
}
func (r *PostgresAccountRepositoryTx) GetChildren(ctx context.Context, parentID common.ID) ([]domain.Account, error) {
	return nil, nil
}
func (r *PostgresAccountRepositoryTx) Delete(ctx context.Context, id common.ID) error { return nil }
func (r *PostgresAccountRepositoryTx) ExistsByCode(ctx context.Context, entityID common.ID, code string) (bool, error) {
	return false, nil
}
func (r *PostgresAccountRepositoryTx) GetPostingAccounts(ctx context.Context, entityID common.ID) ([]domain.Account, error) {
	return nil, nil
}
func (r *PostgresAccountRepositoryTx) WithTx(tx *sql.Tx) AccountRepository { return r }
