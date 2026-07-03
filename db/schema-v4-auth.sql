-- Insucar schema v4 — app-level auth for the prototype (agents log in with agent_id).
-- Run after schema.sql (+ v3). customers.password_hash already exists.
ALTER TABLE staff ADD COLUMN IF NOT EXISTS agent_id      TEXT UNIQUE;
ALTER TABLE staff ADD COLUMN IF NOT EXISTS password_hash TEXT;
CREATE INDEX IF NOT EXISTS idx_staff_agent_id ON staff(agent_id);
