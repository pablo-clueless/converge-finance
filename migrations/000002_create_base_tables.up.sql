-- Entities (Companies/Legal Entities)
CREATE TABLE entities (
    id              CHAR(26) PRIMARY KEY,
    code            VARCHAR(20) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    base_currency   CHAR(3) NOT NULL DEFAULT 'USD',
    fiscal_year_end_month INTEGER NOT NULL DEFAULT 12 CHECK (fiscal_year_end_month BETWEEN 1 AND 12),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    settings        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entities_code ON entities(code);
CREATE INDEX idx_entities_active ON entities(is_active) WHERE is_active = TRUE;

-- Currencies
CREATE TABLE currencies (
    code            CHAR(3) PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    symbol          VARCHAR(10) NOT NULL,
    decimal_places  INTEGER NOT NULL DEFAULT 2 CHECK (decimal_places >= 0 AND decimal_places <= 8),
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert common currencies
INSERT INTO currencies (code, name, symbol, decimal_places) VALUES
    ('USD', 'US Dollar', '$', 2),
    ('EUR', 'Euro', '€', 2),
    ('GBP', 'British Pound', '£', 2),
    ('JPY', 'Japanese Yen', '¥', 0),
    ('CHF', 'Swiss Franc', 'CHF', 2),
    ('CAD', 'Canadian Dollar', 'C$', 2),
    ('AUD', 'Australian Dollar', 'A$', 2),
    ('CNY', 'Chinese Yuan', '¥', 2),
    ('INR', 'Indian Rupee', '₹', 2),
    ('BRL', 'Brazilian Real', 'R$', 2),
    ('MXN', 'Mexican Peso', '$', 2),
    ('SGD', 'Singapore Dollar', 'S$', 2),
    ('HKD', 'Hong Kong Dollar', 'HK$', 2),
    ('KRW', 'South Korean Won', '₩', 0),
    ('ZAR', 'South African Rand', 'R', 2),
    ('NGN', 'Nigerian Naira', '₦', 2);

-- Exchange Rates
CREATE TABLE exchange_rates (
    id              CHAR(26) PRIMARY KEY,
    from_currency   CHAR(3) NOT NULL REFERENCES currencies(code),
    to_currency     CHAR(3) NOT NULL REFERENCES currencies(code),
    rate            DECIMAL(18,8) NOT NULL CHECK (rate > 0),
    rate_type       VARCHAR(20) NOT NULL DEFAULT 'spot' CHECK (rate_type IN ('spot', 'average', 'budget', 'closing')),
    effective_date  DATE NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      CHAR(26),

    CONSTRAINT exchange_rates_uk UNIQUE (from_currency, to_currency, rate_type, effective_date),
    CONSTRAINT exchange_rates_different_currencies CHECK (from_currency != to_currency)
);

CREATE INDEX idx_exchange_rates_lookup ON exchange_rates(from_currency, to_currency, rate_type, effective_date DESC);

-- Users (basic user table for authentication)
CREATE TABLE users (
    id              CHAR(26) PRIMARY KEY,
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    first_name      VARCHAR(100) NOT NULL,
    last_name       VARCHAR(100) NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    is_system       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ
);

CREATE INDEX idx_users_email ON users(email);

-- User Entity Access (which entities a user can access)
CREATE TABLE user_entity_access (
    id              CHAR(26) PRIMARY KEY,
    user_id         CHAR(26) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    roles           TEXT[] NOT NULL DEFAULT '{}',
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_entity_access_uk UNIQUE (user_id, entity_id)
);

CREATE INDEX idx_user_entity_access_user ON user_entity_access(user_id);
CREATE INDEX idx_user_entity_access_entity ON user_entity_access(entity_id);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_entities_updated_at
    BEFORE UPDATE ON entities
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
