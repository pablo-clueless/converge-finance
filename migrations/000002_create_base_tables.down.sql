DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP TRIGGER IF EXISTS trg_entities_updated_at ON entities;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS user_entity_access;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS exchange_rates;
DROP TABLE IF EXISTS currencies;
DROP TABLE IF EXISTS entities;
