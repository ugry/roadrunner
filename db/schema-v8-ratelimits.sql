-- Insucar schema v8 — persistent rate limits + admin roles
CREATE TABLE IF NOT EXISTS rate_limits (
  endpoint   TEXT PRIMARY KEY,
  rpm        INTEGER NOT NULL DEFAULT 30,
  burst      INTEGER NOT NULL DEFAULT 15,
  enabled    BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

INSERT INTO rate_limits (endpoint, rpm, burst, enabled) VALUES
  ('/api/register', 10, 5, true),
  ('/api/user/login', 20, 10, true),
  ('/api/agent/login', 20, 10, true),
  ('/api/telephony/mock/incoming', 60, 30, true),
  ('/api/user/incident', 30, 15, true)
ON CONFLICT (endpoint) DO NOTHING;
