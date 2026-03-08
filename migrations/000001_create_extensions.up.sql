-- Enable required PostgreSQL extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create schemas for each module
CREATE SCHEMA IF NOT EXISTS gl;
CREATE SCHEMA IF NOT EXISTS ap;
CREATE SCHEMA IF NOT EXISTS ar;
CREATE SCHEMA IF NOT EXISTS audit;

-- Set search path to include all schemas
DO $$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path TO public, gl, ap, ar, audit', current_database());
END
$$;
