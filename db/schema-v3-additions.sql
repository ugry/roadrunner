-- Insucar schema v3 additions — Redion parity: multi-line services, B2B white-label multi-tenancy.
-- Additive migration; run AFTER schema.sql. NOTE: no wrapping transaction, because
-- ALTER TYPE ... ADD VALUE must commit before the new value is used (PostgreSQL 12+).

-- ---------------------------------------------------------------------------
-- New service line + extended enums (match Redion offering)
-- ---------------------------------------------------------------------------
CREATE TYPE service_line AS ENUM
  ('mobility','travel','home_living','health','senior_care','concierge');

ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'repair_on_spot';
ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'vehicle_repatriation';
ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'journey_continuation';
ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'car_pickup_delivery';
ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'tyre_protection';
ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'car_swap_ev';
ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'service_activated_rsa';
ALTER TYPE required_service ADD VALUE IF NOT EXISTS 'micromobility';

ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'vehicle_repatriation';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'journey_continuation';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'pickup_delivery';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'tyre_protection';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'ev_car_swap';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'micromobility';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'medical';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'home';
ALTER TYPE entitlement_kind ADD VALUE IF NOT EXISTS 'concierge';

ALTER TYPE provider_category ADD VALUE IF NOT EXISTS 'rental';
ALTER TYPE provider_category ADD VALUE IF NOT EXISTS 'repatriation_transport';
ALTER TYPE provider_category ADD VALUE IF NOT EXISTS 'micromobility';
ALTER TYPE provider_category ADD VALUE IF NOT EXISTS 'medical';
ALTER TYPE provider_category ADD VALUE IF NOT EXISTS 'home';
ALTER TYPE provider_category ADD VALUE IF NOT EXISTS 'garage';

CREATE TYPE tenant_isolation_mode AS ENUM ('rls','schema','database');

-- ---------------------------------------------------------------------------
-- B2B white-label: tenants + partners
-- ---------------------------------------------------------------------------
CREATE TABLE tenants (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name                TEXT NOT NULL,
  subdomain           TEXT UNIQUE NOT NULL,
  custom_domain       TEXT UNIQUE,
  branding            JSONB NOT NULL DEFAULT '{}',        -- logo, colors, etc.
  enabled_service_lines service_line[] NOT NULL DEFAULT '{mobility}',
  default_language    TEXT NOT NULL DEFAULT 'en',
  countries           CHAR(2)[] NOT NULL DEFAULT '{}',
  isolation_mode      tenant_isolation_mode NOT NULL DEFAULT 'rls',
  status              TEXT NOT NULL DEFAULT 'active',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partners (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  legal_name          TEXT NOT NULL,
  contract_ref        TEXT,
  dpa_signed          BOOLEAN NOT NULL DEFAULT FALSE,     -- data-processing agreement
  status              TEXT NOT NULL DEFAULT 'active',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE partner_users (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  partner_id          UUID NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  keycloak_subject    TEXT UNIQUE NOT NULL,
  email               CITEXT UNIQUE NOT NULL,
  display_name        TEXT NOT NULL,
  role                TEXT NOT NULL DEFAULT 'partner_admin',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-tenant coverage products (what a partner sells under their brand)
CREATE TABLE coverage_products (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  service_line        service_line NOT NULL,
  name                TEXT NOT NULL,
  entitlements        entitlement_kind[] NOT NULL DEFAULT '{}',
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Add tenant scoping + service_line to existing tables
-- ---------------------------------------------------------------------------
ALTER TABLE customers           ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);
ALTER TABLE policies            ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);
ALTER TABLE policies            ADD COLUMN IF NOT EXISTS service_line service_line NOT NULL DEFAULT 'mobility';
ALTER TABLE vehicles            ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);
ALTER TABLE providers           ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);
ALTER TABLE provider_connectors ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);
ALTER TABLE cases               ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);
ALTER TABLE cases               ADD COLUMN IF NOT EXISTS service_line service_line NOT NULL DEFAULT 'mobility';
ALTER TABLE missions            ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);
ALTER TABLE notifications       ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

CREATE INDEX IF NOT EXISTS idx_customers_tenant ON customers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_policies_tenant  ON policies(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cases_tenant     ON cases(tenant_id);
CREATE INDEX IF NOT EXISTS idx_missions_tenant  ON missions(tenant_id);

-- ---------------------------------------------------------------------------
-- Row-Level Security (default isolation). App sets: SET app.current_tenant = '<uuid>'.
-- When unset (NULL), rows are visible (platform-admin/back-office context).
-- ---------------------------------------------------------------------------
DO $$
DECLARE t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['customers','policies','vehicles','providers','provider_connectors','cases','missions','notifications']
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format($f$
      CREATE POLICY tenant_isolation ON %I
        USING (current_setting('app.current_tenant', true) IS NULL
               OR tenant_id::text = current_setting('app.current_tenant', true))
        WITH CHECK (current_setting('app.current_tenant', true) IS NULL
               OR tenant_id::text = current_setting('app.current_tenant', true))
    $f$, t);
  END LOOP;
END $$;
