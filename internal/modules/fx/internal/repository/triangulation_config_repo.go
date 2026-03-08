package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fx/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/lib/pq"
)

type TriangulationConfigRepository interface {
	Create(ctx context.Context, config *domain.TriangulationConfig) error
	Update(ctx context.Context, config *domain.TriangulationConfig) error
	GetByEntityID(ctx context.Context, entityID common.ID) (*domain.TriangulationConfig, error)
	Delete(ctx context.Context, id common.ID) error
}

type CurrencyPairConfigRepository interface {
	Create(ctx context.Context, config *domain.CurrencyPairConfig) error
	Update(ctx context.Context, config *domain.CurrencyPairConfig) error
	GetByID(ctx context.Context, id common.ID) (*domain.CurrencyPairConfig, error)
	GetByPair(ctx context.Context, entityID common.ID, from, to string) (*domain.CurrencyPairConfig, error)
	List(ctx context.Context, entityID common.ID) ([]domain.CurrencyPairConfig, error)
	Delete(ctx context.Context, id common.ID) error
}

type TriangulationLogRepository interface {
	Create(ctx context.Context, log *domain.TriangulationLog) error
	GetByID(ctx context.Context, id common.ID) (*domain.TriangulationLog, error)
	List(ctx context.Context, filter TriangulationLogFilter) ([]domain.TriangulationLog, int, error)
}

type TriangulationLogFilter struct {
	EntityID      common.ID
	FromCurrency  string
	ToCurrency    string
	DateFrom      *time.Time
	DateTo        *time.Time
	ReferenceType string
	ReferenceID   common.ID
	Limit         int
	Offset        int
}

type PostgresTriangulationConfigRepo struct {
	db *database.PostgresDB
}

func NewPostgresTriangulationConfigRepo(db *database.PostgresDB) *PostgresTriangulationConfigRepo {
	return &PostgresTriangulationConfigRepo{db: db}
}

func (r *PostgresTriangulationConfigRepo) Create(ctx context.Context, config *domain.TriangulationConfig) error {
	query := `
		INSERT INTO fx.triangulation_config (
			id, entity_id, base_currency, fallback_currencies, max_legs,
			allow_inverse_rates, rate_tolerance, is_active, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.EntityID,
		config.BaseCurrency.Code,
		pq.Array(config.FallbackCurrencies),
		config.MaxLegs,
		config.AllowInverseRates,
		config.RateTolerance,
		config.IsActive,
		config.CreatedBy,
		config.CreatedAt,
		config.UpdatedAt,
	)

	return err
}

func (r *PostgresTriangulationConfigRepo) Update(ctx context.Context, config *domain.TriangulationConfig) error {
	query := `
		UPDATE fx.triangulation_config SET
			base_currency = $2,
			fallback_currencies = $3,
			max_legs = $4,
			allow_inverse_rates = $5,
			rate_tolerance = $6,
			is_active = $7,
			updated_at = $8
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.BaseCurrency.Code,
		pq.Array(config.FallbackCurrencies),
		config.MaxLegs,
		config.AllowInverseRates,
		config.RateTolerance,
		config.IsActive,
		config.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrConfigNotFound
	}

	return nil
}

func (r *PostgresTriangulationConfigRepo) GetByEntityID(ctx context.Context, entityID common.ID) (*domain.TriangulationConfig, error) {
	query := `
		SELECT id, entity_id, base_currency, fallback_currencies, max_legs,
			   allow_inverse_rates, rate_tolerance, is_active, created_by, created_at, updated_at
		FROM fx.triangulation_config
		WHERE entity_id = $1
	`

	var config domain.TriangulationConfig
	var baseCurrencyCode string
	var fallbackCurrencies []string

	err := r.db.QueryRowContext(ctx, query, entityID).Scan(
		&config.ID,
		&config.EntityID,
		&baseCurrencyCode,
		pq.Array(&fallbackCurrencies),
		&config.MaxLegs,
		&config.AllowInverseRates,
		&config.RateTolerance,
		&config.IsActive,
		&config.CreatedBy,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrConfigNotFound
	}
	if err != nil {
		return nil, err
	}

	config.BaseCurrency = money.MustGetCurrency(baseCurrencyCode)
	config.FallbackCurrencies = fallbackCurrencies

	return &config, nil
}

func (r *PostgresTriangulationConfigRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM fx.triangulation_config WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrConfigNotFound
	}

	return nil
}

type PostgresCurrencyPairConfigRepo struct {
	db *database.PostgresDB
}

func NewPostgresCurrencyPairConfigRepo(db *database.PostgresDB) *PostgresCurrencyPairConfigRepo {
	return &PostgresCurrencyPairConfigRepo{db: db}
}

func (r *PostgresCurrencyPairConfigRepo) Create(ctx context.Context, config *domain.CurrencyPairConfig) error {
	query := `
		INSERT INTO fx.currency_pair_config (
			id, entity_id, from_currency, to_currency, preferred_method,
			via_currency, spread_markup, priority, is_active, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	var viaCurrency *string
	if config.ViaCurrency != nil {
		viaCurrency = &config.ViaCurrency.Code
	}

	_, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.EntityID,
		config.FromCurrency.Code,
		config.ToCurrency.Code,
		config.PreferredMethod,
		viaCurrency,
		config.SpreadMarkup,
		config.Priority,
		config.IsActive,
		config.CreatedBy,
		config.CreatedAt,
		config.UpdatedAt,
	)

	return err
}

func (r *PostgresCurrencyPairConfigRepo) Update(ctx context.Context, config *domain.CurrencyPairConfig) error {
	query := `
		UPDATE fx.currency_pair_config SET
			preferred_method = $2,
			via_currency = $3,
			spread_markup = $4,
			priority = $5,
			is_active = $6,
			updated_at = $7
		WHERE id = $1
	`

	var viaCurrency *string
	if config.ViaCurrency != nil {
		viaCurrency = &config.ViaCurrency.Code
	}

	result, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.PreferredMethod,
		viaCurrency,
		config.SpreadMarkup,
		config.Priority,
		config.IsActive,
		config.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrPairConfigNotFound
	}

	return nil
}

func (r *PostgresCurrencyPairConfigRepo) GetByID(ctx context.Context, id common.ID) (*domain.CurrencyPairConfig, error) {
	query := `
		SELECT id, entity_id, from_currency, to_currency, preferred_method,
			   via_currency, spread_markup, priority, is_active, created_by, created_at, updated_at
		FROM fx.currency_pair_config
		WHERE id = $1
	`

	return r.scanPairConfig(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresCurrencyPairConfigRepo) GetByPair(ctx context.Context, entityID common.ID, from, to string) (*domain.CurrencyPairConfig, error) {
	query := `
		SELECT id, entity_id, from_currency, to_currency, preferred_method,
			   via_currency, spread_markup, priority, is_active, created_by, created_at, updated_at
		FROM fx.currency_pair_config
		WHERE entity_id = $1 AND from_currency = $2 AND to_currency = $3 AND is_active = true
	`

	return r.scanPairConfig(r.db.QueryRowContext(ctx, query, entityID, from, to))
}

func (r *PostgresCurrencyPairConfigRepo) List(ctx context.Context, entityID common.ID) ([]domain.CurrencyPairConfig, error) {
	query := `
		SELECT id, entity_id, from_currency, to_currency, preferred_method,
			   via_currency, spread_markup, priority, is_active, created_by, created_at, updated_at
		FROM fx.currency_pair_config
		WHERE entity_id = $1
		ORDER BY priority DESC, from_currency, to_currency
	`

	rows, err := r.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []domain.CurrencyPairConfig
	for rows.Next() {
		config, err := r.scanPairConfigRow(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *config)
	}

	return configs, rows.Err()
}

func (r *PostgresCurrencyPairConfigRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM fx.currency_pair_config WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrPairConfigNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *PostgresCurrencyPairConfigRepo) scanPairConfig(row rowScanner) (*domain.CurrencyPairConfig, error) {
	var config domain.CurrencyPairConfig
	var fromCode, toCode string
	var viaCurrencyCode *string

	err := row.Scan(
		&config.ID,
		&config.EntityID,
		&fromCode,
		&toCode,
		&config.PreferredMethod,
		&viaCurrencyCode,
		&config.SpreadMarkup,
		&config.Priority,
		&config.IsActive,
		&config.CreatedBy,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPairConfigNotFound
	}
	if err != nil {
		return nil, err
	}

	config.FromCurrency = money.MustGetCurrency(fromCode)
	config.ToCurrency = money.MustGetCurrency(toCode)
	if viaCurrencyCode != nil {
		via := money.MustGetCurrency(*viaCurrencyCode)
		config.ViaCurrency = &via
	}

	return &config, nil
}

func (r *PostgresCurrencyPairConfigRepo) scanPairConfigRow(rows *sql.Rows) (*domain.CurrencyPairConfig, error) {
	var config domain.CurrencyPairConfig
	var fromCode, toCode string
	var viaCurrencyCode *string

	err := rows.Scan(
		&config.ID,
		&config.EntityID,
		&fromCode,
		&toCode,
		&config.PreferredMethod,
		&viaCurrencyCode,
		&config.SpreadMarkup,
		&config.Priority,
		&config.IsActive,
		&config.CreatedBy,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	config.FromCurrency = money.MustGetCurrency(fromCode)
	config.ToCurrency = money.MustGetCurrency(toCode)
	if viaCurrencyCode != nil {
		via := money.MustGetCurrency(*viaCurrencyCode)
		config.ViaCurrency = &via
	}

	return &config, nil
}

type PostgresTriangulationLogRepo struct {
	db *database.PostgresDB
}

func NewPostgresTriangulationLogRepo(db *database.PostgresDB) *PostgresTriangulationLogRepo {
	return &PostgresTriangulationLogRepo{db: db}
}

func (r *PostgresTriangulationLogRepo) Create(ctx context.Context, log *domain.TriangulationLog) error {
	query := `
		INSERT INTO fx.triangulation_log (
			id, entity_id, from_currency, to_currency, original_amount, result_amount,
			effective_rate, legs, legs_count, method_used, conversion_date, rate_type,
			reference_type, reference_id, created_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	legsJSON, err := json.Marshal(log.Legs)
	if err != nil {
		return err
	}

	var refID *string
	if !log.ReferenceID.IsZero() {
		s := log.ReferenceID.String()
		refID = &s
	}

	_, err = r.db.ExecContext(ctx, query,
		log.ID,
		log.EntityID,
		log.FromCurrency,
		log.ToCurrency,
		log.OriginalAmount,
		log.ResultAmount,
		log.EffectiveRate,
		legsJSON,
		log.LegsCount,
		log.MethodUsed,
		log.ConversionDate,
		log.RateType,
		log.ReferenceType,
		refID,
		log.CreatedBy,
		log.CreatedAt,
	)

	return err
}

func (r *PostgresTriangulationLogRepo) GetByID(ctx context.Context, id common.ID) (*domain.TriangulationLog, error) {
	query := `
		SELECT id, entity_id, from_currency, to_currency, original_amount, result_amount,
			   effective_rate, legs, legs_count, method_used, conversion_date, rate_type,
			   reference_type, reference_id, created_by, created_at
		FROM fx.triangulation_log
		WHERE id = $1
	`

	var log domain.TriangulationLog
	var legsJSON []byte
	var refID *string

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&log.ID,
		&log.EntityID,
		&log.FromCurrency,
		&log.ToCurrency,
		&log.OriginalAmount,
		&log.ResultAmount,
		&log.EffectiveRate,
		&legsJSON,
		&log.LegsCount,
		&log.MethodUsed,
		&log.ConversionDate,
		&log.RateType,
		&log.ReferenceType,
		&refID,
		&log.CreatedBy,
		&log.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrLogNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(legsJSON, &log.Legs); err != nil {
		return nil, err
	}
	if refID != nil {
		log.ReferenceID = common.ID(*refID)
	}

	return &log, nil
}

func (r *PostgresTriangulationLogRepo) List(ctx context.Context, filter TriangulationLogFilter) ([]domain.TriangulationLog, int, error) {
	baseQuery := `
		FROM fx.triangulation_log
		WHERE entity_id = $1
	`

	args := []any{filter.EntityID}
	argIdx := 2

	if filter.FromCurrency != "" {
		baseQuery += ` AND from_currency = $` + string(rune('0'+argIdx))
		args = append(args, filter.FromCurrency)
		argIdx++
	}
	if filter.ToCurrency != "" {
		baseQuery += ` AND to_currency = $` + string(rune('0'+argIdx))
		args = append(args, filter.ToCurrency)
		argIdx++
	}
	if filter.DateFrom != nil {
		baseQuery += ` AND conversion_date >= $` + string(rune('0'+argIdx))
		args = append(args, filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		baseQuery += ` AND conversion_date <= $` + string(rune('0'+argIdx))
		args = append(args, filter.DateTo)
		argIdx++
	}
	if filter.ReferenceType != "" {
		baseQuery += ` AND reference_type = $` + string(rune('0'+argIdx))
		args = append(args, filter.ReferenceType)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `SELECT id, entity_id, from_currency, to_currency, original_amount, result_amount,
			   effective_rate, legs, legs_count, method_used, conversion_date, rate_type,
			   reference_type, reference_id, created_by, created_at ` + baseQuery +
		` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		dataQuery += ` LIMIT $` + string(rune('0'+argIdx))
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		dataQuery += ` OFFSET $` + string(rune('0'+argIdx))
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []domain.TriangulationLog
	for rows.Next() {
		var log domain.TriangulationLog
		var legsJSON []byte
		var refID *string

		err := rows.Scan(
			&log.ID,
			&log.EntityID,
			&log.FromCurrency,
			&log.ToCurrency,
			&log.OriginalAmount,
			&log.ResultAmount,
			&log.EffectiveRate,
			&legsJSON,
			&log.LegsCount,
			&log.MethodUsed,
			&log.ConversionDate,
			&log.RateType,
			&log.ReferenceType,
			&refID,
			&log.CreatedBy,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		if err := json.Unmarshal(legsJSON, &log.Legs); err != nil {
			return nil, 0, err
		}
		if refID != nil {
			log.ReferenceID = common.ID(*refID)
		}

		logs = append(logs, log)
	}

	return logs, total, rows.Err()
}
