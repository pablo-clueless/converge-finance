-- FX Schema for Triangulation and Currency Management
-- Handles multi-leg currency conversions and FX rate management

-- Create schema
CREATE SCHEMA IF NOT EXISTS fx;

-- Enums
CREATE TYPE fx.triangulation_method AS ENUM ('direct', 'via_base', 'via_usd', 'via_eur', 'custom');

-- Triangulation Configuration Table
-- Defines entity-level preferences for currency conversion
CREATE TABLE fx.triangulation_config (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    base_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    fallback_currencies TEXT[] NOT NULL DEFAULT '{USD,EUR}',
    max_legs INTEGER NOT NULL DEFAULT 3 CHECK (max_legs BETWEEN 2 AND 5),
    allow_inverse_rates BOOLEAN NOT NULL DEFAULT true,
    rate_tolerance DECIMAL(10,6) NOT NULL DEFAULT 0.0001,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by CHAR(26) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id)
);

-- Currency Pair Configuration Table
-- Defines preferred conversion methods for specific currency pairs
CREATE TABLE fx.currency_pair_config (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    from_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    to_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    preferred_method fx.triangulation_method NOT NULL DEFAULT 'direct',
    via_currency CHAR(3) REFERENCES currencies(code),
    spread_markup DECIMAL(10,6) NOT NULL DEFAULT 0 CHECK (spread_markup >= 0),
    priority INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_by CHAR(26) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, from_currency, to_currency),
    CHECK (from_currency != to_currency),
    CHECK (via_currency IS NULL OR (via_currency != from_currency AND via_currency != to_currency))
);

-- Triangulation Log Table
-- Audit trail of all currency conversions
CREATE TABLE fx.triangulation_log (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    from_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    to_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    original_amount DECIMAL(18,4) NOT NULL,
    result_amount DECIMAL(18,4) NOT NULL,
    effective_rate DECIMAL(18,8) NOT NULL,
    legs JSONB NOT NULL DEFAULT '[]',
    -- Legs structure: [{from: "USD", to: "EUR", rate: 0.92, rate_type: "spot", rate_date: "2024-01-15"}]
    legs_count INTEGER NOT NULL DEFAULT 1,
    method_used fx.triangulation_method NOT NULL,
    conversion_date DATE NOT NULL,
    rate_type VARCHAR(20) NOT NULL,
    reference_type VARCHAR(50),
    reference_id CHAR(26),
    created_by CHAR(26) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_fx_triangulation_config_entity ON fx.triangulation_config(entity_id);
CREATE INDEX idx_fx_triangulation_config_active ON fx.triangulation_config(entity_id, is_active) WHERE is_active = true;

CREATE INDEX idx_fx_currency_pair_entity ON fx.currency_pair_config(entity_id);
CREATE INDEX idx_fx_currency_pair_lookup ON fx.currency_pair_config(entity_id, from_currency, to_currency) WHERE is_active = true;
CREATE INDEX idx_fx_currency_pair_from ON fx.currency_pair_config(from_currency);
CREATE INDEX idx_fx_currency_pair_to ON fx.currency_pair_config(to_currency);

CREATE INDEX idx_fx_triangulation_log_entity ON fx.triangulation_log(entity_id);
CREATE INDEX idx_fx_triangulation_log_date ON fx.triangulation_log(conversion_date);
CREATE INDEX idx_fx_triangulation_log_currencies ON fx.triangulation_log(from_currency, to_currency);
CREATE INDEX idx_fx_triangulation_log_reference ON fx.triangulation_log(reference_type, reference_id);
CREATE INDEX idx_fx_triangulation_log_created ON fx.triangulation_log(created_at);

-- Triggers for updated_at
CREATE TRIGGER update_fx_triangulation_config_updated_at
    BEFORE UPDATE ON fx.triangulation_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_fx_currency_pair_config_updated_at
    BEFORE UPDATE ON fx.currency_pair_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- RLS Policies
ALTER TABLE fx.triangulation_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE fx.currency_pair_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE fx.triangulation_log ENABLE ROW LEVEL SECURITY;

CREATE POLICY fx_triangulation_config_entity_isolation ON fx.triangulation_config
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY fx_currency_pair_config_entity_isolation ON fx.currency_pair_config
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY fx_triangulation_log_entity_isolation ON fx.triangulation_log
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Helper function to find best conversion path
CREATE OR REPLACE FUNCTION fx.find_conversion_path(
    p_entity_id CHAR(26),
    p_from_currency CHAR(3),
    p_to_currency CHAR(3),
    p_rate_date DATE,
    p_rate_type VARCHAR(20) DEFAULT 'spot'
)
RETURNS TABLE (
    path_currencies TEXT[],
    path_rates DECIMAL(18,8)[],
    effective_rate DECIMAL(18,8),
    legs_count INTEGER
) AS $$
DECLARE
    v_direct_rate DECIMAL(18,8);
    v_inverse_rate DECIMAL(18,8);
    v_via_currency CHAR(3);
    v_leg1_rate DECIMAL(18,8);
    v_leg2_rate DECIMAL(18,8);
    v_config fx.triangulation_config%ROWTYPE;
BEGIN
    -- Get entity config
    SELECT * INTO v_config FROM fx.triangulation_config
    WHERE entity_id = p_entity_id AND is_active = true;

    -- Try direct rate first
    SELECT rate INTO v_direct_rate
    FROM exchange_rates
    WHERE from_currency = p_from_currency
    AND to_currency = p_to_currency
    AND rate_type = p_rate_type
    AND effective_date <= p_rate_date
    ORDER BY effective_date DESC
    LIMIT 1;

    IF v_direct_rate IS NOT NULL THEN
        RETURN QUERY SELECT
            ARRAY[p_from_currency, p_to_currency],
            ARRAY[v_direct_rate],
            v_direct_rate,
            1;
        RETURN;
    END IF;

    -- Try inverse rate if allowed
    IF v_config IS NULL OR v_config.allow_inverse_rates THEN
        SELECT rate INTO v_inverse_rate
        FROM exchange_rates
        WHERE from_currency = p_to_currency
        AND to_currency = p_from_currency
        AND rate_type = p_rate_type
        AND effective_date <= p_rate_date
        ORDER BY effective_date DESC
        LIMIT 1;

        IF v_inverse_rate IS NOT NULL THEN
            RETURN QUERY SELECT
                ARRAY[p_from_currency, p_to_currency],
                ARRAY[1.0 / v_inverse_rate],
                1.0 / v_inverse_rate,
                1;
            RETURN;
        END IF;
    END IF;

    -- Try via base currency or fallbacks
    FOR v_via_currency IN
        SELECT unnest(COALESCE(v_config.fallback_currencies, ARRAY['USD', 'EUR']))
    LOOP
        IF v_via_currency = p_from_currency OR v_via_currency = p_to_currency THEN
            CONTINUE;
        END IF;

        -- Get leg 1: from_currency -> via_currency
        SELECT rate INTO v_leg1_rate
        FROM exchange_rates
        WHERE from_currency = p_from_currency
        AND to_currency = v_via_currency
        AND rate_type = p_rate_type
        AND effective_date <= p_rate_date
        ORDER BY effective_date DESC
        LIMIT 1;

        IF v_leg1_rate IS NULL AND (v_config IS NULL OR v_config.allow_inverse_rates) THEN
            SELECT 1.0 / rate INTO v_leg1_rate
            FROM exchange_rates
            WHERE from_currency = v_via_currency
            AND to_currency = p_from_currency
            AND rate_type = p_rate_type
            AND effective_date <= p_rate_date
            ORDER BY effective_date DESC
            LIMIT 1;
        END IF;

        IF v_leg1_rate IS NULL THEN
            CONTINUE;
        END IF;

        -- Get leg 2: via_currency -> to_currency
        SELECT rate INTO v_leg2_rate
        FROM exchange_rates
        WHERE from_currency = v_via_currency
        AND to_currency = p_to_currency
        AND rate_type = p_rate_type
        AND effective_date <= p_rate_date
        ORDER BY effective_date DESC
        LIMIT 1;

        IF v_leg2_rate IS NULL AND (v_config IS NULL OR v_config.allow_inverse_rates) THEN
            SELECT 1.0 / rate INTO v_leg2_rate
            FROM exchange_rates
            WHERE from_currency = p_to_currency
            AND to_currency = v_via_currency
            AND rate_type = p_rate_type
            AND effective_date <= p_rate_date
            ORDER BY effective_date DESC
            LIMIT 1;
        END IF;

        IF v_leg2_rate IS NOT NULL THEN
            RETURN QUERY SELECT
                ARRAY[p_from_currency, v_via_currency, p_to_currency],
                ARRAY[v_leg1_rate, v_leg2_rate],
                v_leg1_rate * v_leg2_rate,
                2;
            RETURN;
        END IF;
    END LOOP;

    -- No path found
    RETURN;
END;
$$ LANGUAGE plpgsql;
