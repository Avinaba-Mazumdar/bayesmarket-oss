-- ==============================================================================
-- BayesMarket - Least-Privilege Application User Provisioning Script
-- ==============================================================================
-- Target: Neon Serverless PostgreSQL / Cloud PostgreSQL
-- Purpose:
--   Creates a dedicated, unprivileged runtime database role ('bayes_app') for the
--   BayesMarket Go API server.
--
-- Security Guarantees:
--   1. Permitted DML: SELECT, INSERT, UPDATE on application tables.
--   2. Permitted Sequence Usage: USAGE, SELECT on sequence generators.
--   3. PROHIBITED: DROP, TRUNCATE, ALTER, CREATE TABLE, and DELETE operations.
--
-- Instructions:
--   1. Run this script in the Neon SQL Editor or via psql as the database owner ('neondb_owner').
--   2. Replace 'GENERATE_A_STRONG_DB_PASSWORD' with a secure random secret.
--   3. Update your production server's DATABASE_URL:
--      postgres://bayes_app:GENERATE_A_STRONG_DB_PASSWORD@ep-xxx.us-east-2.aws.neon.tech/bayesmarket?sslmode=require
-- ==============================================================================

\set ON_ERROR_STOP on

-- 1. Create the runtime application role if not already created
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'bayes_app') THEN
        CREATE ROLE bayes_app WITH LOGIN PASSWORD 'GENERATE_A_STRONG_DB_PASSWORD';
    ELSE
        ALTER ROLE bayes_app WITH LOGIN PASSWORD 'GENERATE_A_STRONG_DB_PASSWORD';
    END IF;
END
$$;

-- 2. Grant connection and schema traversal rights
GRANT CONNECT ON DATABASE bayesmarket TO bayes_app;
GRANT USAGE ON SCHEMA public TO bayes_app;

-- 3. Explicitly revoke DDL creation rights on schema public
REVOKE CREATE ON SCHEMA public FROM bayes_app;

-- 4. Grant least-privilege DML (SELECT, INSERT, UPDATE) on all current tables
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO bayes_app;

-- 5. Explicitly revoke destructive privileges on all tables
REVOKE DELETE, TRUNCATE, REFERENCES, TRIGGER ON ALL TABLES IN SCHEMA public FROM bayes_app;

-- 6. Grant sequence access (for serials / identity sequences)
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO bayes_app;

-- 7. Ensure future tables created by migrations automatically grant DML to bayes_app
ALTER DEFAULT PRIVILEGES IN SCHEMA public 
GRANT SELECT, INSERT, UPDATE ON TABLES TO bayes_app;

ALTER DEFAULT PRIVILEGES IN SCHEMA public 
GRANT USAGE, SELECT ON SEQUENCES TO bayes_app;

-- 8. Verify assigned permissions
SELECT grantee, table_name, privilege_type
FROM information_schema.role_table_grants
WHERE grantee = 'bayes_app'
ORDER BY table_name, privilege_type;
