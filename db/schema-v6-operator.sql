-- schema-v6-operator.sql — Rich operator console support
-- Adds: dispatched_at timestamp to cases, operator status to staff, mission_status_events improvements

-- Track when a case was dispatched (separate from generic updated_at)
ALTER TABLE cases ADD COLUMN IF NOT EXISTS dispatched_at TIMESTAMPTZ;

-- Operator availability tracking
ALTER TABLE staff ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'on_call';

-- Track which operator dispatched a case
ALTER TABLE cases ADD COLUMN IF NOT EXISTS dispatched_by TEXT;
