-- Insucar schema v7 — admin API endpoint access management
CREATE TABLE IF NOT EXISTS api_endpoints (
  id          UUID DEFAULT gen_random_uuid() PRIMARY KEY,
  endpoint    TEXT NOT NULL UNIQUE,
  methods     TEXT NOT NULL DEFAULT 'GET',
  min_role    TEXT NOT NULL DEFAULT 'user',
  description TEXT DEFAULT '',
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ DEFAULT now(),
  updated_at  TIMESTAMPTZ DEFAULT now()
);

INSERT INTO api_endpoints (endpoint, methods, min_role, description, is_active) VALUES
  ('/api/register', 'POST', 'public', 'End-user self-registration', true),
  ('/api/user/login', 'POST', 'public', 'Customer email/password login', true),
  ('/api/agent/login', 'POST', 'public', 'Operator agent ID login', true),
  ('/api/me', 'GET', 'user', 'Current session identity', true),
  ('/api/user/incident', 'POST', 'user', 'Submit roadside assistance request', true),
  ('/api/user/cases', 'GET', 'user', 'List own cases', true),
  ('/api/agent/cases', 'GET', 'agent', 'Operator case queue', true),
  ('/api/agent/case', 'GET', 'agent', 'Case detail with customer/vehicle/mission', true),
  ('/api/agent/dispatch', 'POST', 'agent', 'Dispatch provider to case', true),
  ('/api/agent/lookup', 'GET', 'agent', 'ANI screen-pop lookup', true),
  ('/api/agent/providers', 'GET', 'agent', 'Ranked provider list', true),
  ('/api/agent/stats', 'GET', 'agent', 'Queue statistics', true),
  ('/api/telephony/mock/incoming', 'POST', 'agent', 'Mock telephony inbound call', true),
  ('/api/auth/config', 'GET', 'public', 'Cognito auth configuration', true),
  ('/api/status', 'GET', 'public', 'Live customer tracking page', true),
  ('/api/admin/rate-limits', 'GET,PUT', 'agent', 'Admin: rate limit configuration', true),
  ('/api/admin/api-access', 'GET,PUT', 'agent', 'Admin: endpoint access management', true),
  ('/api/admin/operators', 'GET,POST,DELETE', 'agent', 'Admin: operator CRUD', true),
  ('/api/admin/stats', 'GET', 'agent', 'Admin: platform statistics', true)
ON CONFLICT (endpoint) DO NOTHING;
