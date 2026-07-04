-- Insucar schema v5 — Amazon Cognito subject mapping (managed auth).
-- Run after schema.sql (+ v3, v4). Lets the app map a Cognito token 'sub' to a
-- customer/staff row, and JIT-provision a customer on first Cognito login.
-- Coexists with keycloak_subject (kept for backward compat during migration).

ALTER TABLE customers ADD COLUMN IF NOT EXISTS cognito_subject TEXT UNIQUE;
ALTER TABLE staff     ADD COLUMN IF NOT EXISTS cognito_subject TEXT UNIQUE;

CREATE INDEX IF NOT EXISTS idx_customers_cognito ON customers (cognito_subject);
CREATE INDEX IF NOT EXISTS idx_staff_cognito     ON staff (cognito_subject);
