-- seed-tenant.sql — Default Insucar tenant for multi-tenant RLS.
-- Run after schema-v3-additions.sql if starting fresh.

INSERT INTO tenants(id, name, subdomain, default_language, countries, enabled_service_lines)
VALUES ('10000000-0000-0000-0000-000000000001', 'Insucar', 'insucar', 'en', '{FR,GB,DE}', '{mobility}')
ON CONFLICT (subdomain) DO NOTHING;

-- Set default tenant on existing rows (backfill for existing data before RLS was enforced)
DO $$
DECLARE
  t_id UUID := '10000000-0000-0000-0000-000000000001';
BEGIN
  UPDATE customers SET tenant_id = t_id WHERE tenant_id IS NULL;
  UPDATE policies  SET tenant_id = t_id WHERE tenant_id IS NULL;
  UPDATE vehicles  SET tenant_id = t_id WHERE tenant_id IS NULL;
  UPDATE providers SET tenant_id = t_id WHERE tenant_id IS NULL;
  UPDATE provider_connectors SET tenant_id = t_id WHERE tenant_id IS NULL;
  UPDATE cases     SET tenant_id = t_id WHERE tenant_id IS NULL;
  UPDATE missions  SET tenant_id = t_id WHERE tenant_id IS NULL;
  UPDATE notifications SET tenant_id = t_id WHERE tenant_id IS NULL;
END $$;
