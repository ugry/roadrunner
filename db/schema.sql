-- Insucar — PostgreSQL 15+ / PostGIS schema (prototype v1)
-- Covers: end-user registration + identity, policies/coverage, vehicles, cases/incidents,
-- location + safety triage, providers/connectors, dispatch missions, notifications,
-- consent/GDPR, call recordings, immutable hash-chained audit ledger, and production-access grants.
-- Auth: staff and customers authenticate via Keycloak (OIDC); we store the IdP subject and
-- keep an optional local password_hash column only for offline/dev fallback.

BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;      -- geography/geometry for dispatch
CREATE EXTENSION IF NOT EXISTS pgcrypto;     -- gen_random_uuid(), digest()
CREATE EXTENSION IF NOT EXISTS citext;       -- case-insensitive email

-- ---------------------------------------------------------------------------
-- Enumerated types
-- ---------------------------------------------------------------------------
CREATE TYPE customer_status     AS ENUM ('pending_verification','active','suspended','closed');
CREATE TYPE verification_channel AS ENUM ('email','phone');
CREATE TYPE consent_purpose     AS ENUM ('terms','privacy','marketing','call_recording','data_processing');
CREATE TYPE lawful_basis        AS ENUM ('contract','consent','vital_interest','legal_obligation','legitimate_interest');

CREATE TYPE policy_status       AS ENUM ('active','expired','suspended','cancelled');
CREATE TYPE product_type        AS ENUM ('comprehensive','roadside_only','membership','rental_addon');
CREATE TYPE coverage_level      AS ENUM ('basic','standard','premium');
CREATE TYPE entitlement_kind    AS ENUM ('towing','roadside_repair','replacement_vehicle','accommodation','onward_travel','home_start','ev_charge');

CREATE TYPE fuel_type           AS ENUM ('petrol','diesel','ev','hybrid','lpg');
CREATE TYPE transmission_type   AS ENUM ('manual','automatic');
CREATE TYPE vehicle_category    AS ENUM ('car','van','motorhome','motorcycle');

CREATE TYPE case_channel        AS ENUM ('phone','app','web','callback');
CREATE TYPE case_status         AS ENUM ('new','triaging','dispatched','en_route','on_site','resolved','closed','cancelled');
CREATE TYPE case_priority       AS ENUM ('low','normal','high','emergency');
CREATE TYPE incident_type       AS ENUM ('breakdown','flat_tyre','battery','lockout','out_of_fuel','wrong_fuel',
                                         'lost_keys','ev_no_charge','accident','collision','theft','medical_emergency','other');
CREATE TYPE resolution_status   AS ENUM ('fixed_on_site','towed','cancelled_by_customer','no_fault_found','unresolved');

CREATE TYPE provider_category   AS ENUM ('towing','repair','body_shop','hotel','mobility');
CREATE TYPE connector_auth_type AS ENUM ('api_key','oauth2_client_credentials','bearer_token','mtls','manual');
CREATE TYPE connector_status    AS ENUM ('enabled','disabled');
CREATE TYPE dispatch_source     AS ENUM ('api','manual');
CREATE TYPE mission_status      AS ENUM ('searching','offered','accepted','en_route','on_site','completed','failed','cancelled');
CREATE TYPE required_service    AS ENUM ('jump_start','tyre_change','fuel_delivery','roadside_repair','lockout','ev_charge','tow_recovery','winching');

CREATE TYPE notification_channel AS ENUM ('sms','email','push');
CREATE TYPE notification_status  AS ENUM ('queued','sent','delivered','failed');

CREATE TYPE staff_role          AS ENUM ('operator','supervisor','admin','ops','product_owner');
CREATE TYPE prod_access_reason  AS ENUM ('emergency','release','change');

-- ---------------------------------------------------------------------------
-- Reference data
-- ---------------------------------------------------------------------------
CREATE TABLE countries (
  code        CHAR(2) PRIMARY KEY,          -- ISO-3166-1 alpha-2
  name        TEXT NOT NULL,
  default_language TEXT NOT NULL
);

CREATE TABLE vehicle_makes (
  id          SERIAL PRIMARY KEY,
  name        TEXT UNIQUE NOT NULL
);

CREATE TABLE vehicle_models (
  id          SERIAL PRIMARY KEY,
  make_id     INT NOT NULL REFERENCES vehicle_makes(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  UNIQUE (make_id, name)
);

-- ---------------------------------------------------------------------------
-- END-USER REGISTRATION & IDENTITY
-- ---------------------------------------------------------------------------
CREATE TABLE customers (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email              CITEXT UNIQUE NOT NULL,
  phone_e164         TEXT UNIQUE,                       -- primary lookup anchor for ANI matching
  first_name         TEXT NOT NULL,
  last_name          TEXT NOT NULL,
  preferred_language TEXT NOT NULL DEFAULT 'en',
  country_code       CHAR(2) REFERENCES countries(code),
  status             customer_status NOT NULL DEFAULT 'pending_verification',
  email_verified     BOOLEAN NOT NULL DEFAULT FALSE,
  phone_verified     BOOLEAN NOT NULL DEFAULT FALSE,
  keycloak_subject   TEXT UNIQUE,                       -- OIDC 'sub' from Keycloak customer realm
  password_hash      TEXT,                              -- dev/offline fallback only; prod uses Keycloak
  marketing_opt_in   BOOLEAN NOT NULL DEFAULT FALSE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_customers_phone ON customers (phone_e164);

-- One-time verification tokens for the registration flow (email/phone confirm, password reset)
CREATE TABLE verification_tokens (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id  UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  channel      verification_channel NOT NULL,
  token_hash   TEXT NOT NULL,                            -- store hash, never the raw token
  expires_at   TIMESTAMPTZ NOT NULL,
  consumed_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_verif_customer ON verification_tokens (customer_id);

-- GDPR consent records captured at registration and thereafter (immutable rows)
CREATE TABLE consents (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id        UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  purpose            consent_purpose NOT NULL,
  granted            BOOLEAN NOT NULL,
  basis              lawful_basis NOT NULL DEFAULT 'consent',
  consent_text_hash  TEXT,                                -- sha-256 of the exact text shown
  ip_address         INET,
  user_agent         TEXT,
  occurred_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_consents_customer ON consents (customer_id, purpose);

-- ---------------------------------------------------------------------------
-- STAFF (operators/supervisors/ops/product owners) — profile mirror of Keycloak
-- ---------------------------------------------------------------------------
CREATE TABLE staff (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  keycloak_subject TEXT UNIQUE NOT NULL,
  display_name     TEXT NOT NULL,
  email            CITEXT UNIQUE NOT NULL,
  role             staff_role NOT NULL,
  languages        TEXT[] NOT NULL DEFAULT '{en}',
  active           BOOLEAN NOT NULL DEFAULT TRUE,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- POLICIES, COVERAGE, VEHICLES
-- ---------------------------------------------------------------------------
CREATE TABLE policies (
  id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  policy_number    TEXT UNIQUE NOT NULL,
  customer_id      UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
  product          product_type NOT NULL,
  coverage         coverage_level NOT NULL,
  status           policy_status NOT NULL DEFAULT 'active',
  valid_from       DATE NOT NULL,
  valid_to         DATE NOT NULL,
  excess_amount    NUMERIC(10,2) NOT NULL DEFAULT 0,
  currency         CHAR(3) NOT NULL DEFAULT 'EUR',
  callout_limit    INT,                                  -- NULL = unlimited
  callout_used     INT NOT NULL DEFAULT 0,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (valid_to >= valid_from)
);
CREATE INDEX idx_policies_customer ON policies (customer_id);

CREATE TABLE policy_territories (
  policy_id    UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
  country_code CHAR(2) NOT NULL REFERENCES countries(code),
  PRIMARY KEY (policy_id, country_code)
);

CREATE TABLE policy_entitlements (
  policy_id     UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
  entitlement   entitlement_kind NOT NULL,
  included      BOOLEAN NOT NULL DEFAULT TRUE,
  limit_amount  NUMERIC(10,2),
  PRIMARY KEY (policy_id, entitlement)
);

CREATE TABLE vehicles (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id    UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  license_plate  TEXT NOT NULL,
  country_code   CHAR(2) REFERENCES countries(code),
  make           TEXT,
  model          TEXT,
  year           INT,
  color          TEXT,
  vin            TEXT,
  fuel           fuel_type,
  transmission   transmission_type,
  category       vehicle_category NOT NULL DEFAULT 'car',
  weight_kg      INT,
  height_m       NUMERIC(4,2),
  length_m       NUMERIC(4,2),
  mileage_km     INT,
  tyre_size      TEXT,
  key_type       TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (license_plate, country_code)
);
CREATE INDEX idx_vehicles_customer ON vehicles (customer_id);
CREATE INDEX idx_vehicles_plate ON vehicles (license_plate);

-- A policy may cover one or more vehicles
CREATE TABLE policy_vehicles (
  policy_id   UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
  vehicle_id  UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
  PRIMARY KEY (policy_id, vehicle_id)
);

-- ---------------------------------------------------------------------------
-- PROVIDERS & CONNECTORS
-- ---------------------------------------------------------------------------
CREATE TABLE providers (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  display_name      TEXT NOT NULL,
  categories        provider_category[] NOT NULL,
  countries         CHAR(2)[] NOT NULL,
  contact_phone     TEXT,                                -- manual-dispatch fallback
  status            connector_status NOT NULL DEFAULT 'enabled',
  priority_rank     INT NOT NULL DEFAULT 100,
  performance_score NUMERIC(4,2),                        -- 0..5, feeds matching
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE provider_connectors (
  id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider_id       UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
  auth_type         connector_auth_type NOT NULL,
  base_url          TEXT,
  sandbox_url       TEXT,
  credentials_ref   TEXT,                                -- AWS Secrets Manager ARN; NEVER the secret value
  webhook_secret_ref TEXT,
  rate_limit_per_hr INT,
  sla_uptime        NUMERIC(5,2),
  capabilities      required_service[] NOT NULL DEFAULT '{}',
  status            connector_status NOT NULL DEFAULT 'enabled',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_connectors_provider ON provider_connectors (provider_id);

CREATE TABLE provider_availability (
  provider_id  UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
  day_of_week  SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
  open_time    TIME NOT NULL,
  close_time   TIME NOT NULL,
  PRIMARY KEY (provider_id, day_of_week, open_time)
);

-- ---------------------------------------------------------------------------
-- CASES / INCIDENTS
-- ---------------------------------------------------------------------------
CREATE TABLE cases (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_number         TEXT UNIQUE NOT NULL,
  customer_id         UUID REFERENCES customers(id) ON DELETE SET NULL,   -- NULL if unknown caller
  policy_id           UUID REFERENCES policies(id) ON DELETE SET NULL,
  vehicle_id          UUID REFERENCES vehicles(id) ON DELETE SET NULL,
  operator_id         UUID REFERENCES staff(id) ON DELETE SET NULL,
  channel             case_channel NOT NULL DEFAULT 'phone',
  connect_contact_id  TEXT,                                               -- Amazon Connect contactId
  status              case_status NOT NULL DEFAULT 'new',
  priority            case_priority NOT NULL DEFAULT 'normal',
  incident            incident_type NOT NULL,
  incident_at         TIMESTAMPTZ,
  symptom_description TEXT,
  vehicle_drivable    BOOLEAN,
  is_accident         BOOLEAN NOT NULL DEFAULT FALSE,
  injuries_reported   BOOLEAN NOT NULL DEFAULT FALSE,
  fire_or_smoke       BOOLEAN NOT NULL DEFAULT FALSE,
  covered_by_policy   BOOLEAN,
  coverage_reason     TEXT,
  -- GDPR meta
  lawful_basis        lawful_basis NOT NULL DEFAULT 'contract',
  data_subject_country CHAR(2) REFERENCES countries(code),
  retention_expiry    DATE,
  erasure_requested   BOOLEAN NOT NULL DEFAULT FALSE,
  -- resolution
  resolution          resolution_status,
  resolution_notes    TEXT,
  linked_claim_number TEXT,
  satisfaction_score  SMALLINT CHECK (satisfaction_score BETWEEN 1 AND 5),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at         TIMESTAMPTZ
);
CREATE INDEX idx_cases_customer ON cases (customer_id);
CREATE INDEX idx_cases_status ON cases (status);
CREATE INDEX idx_cases_contact ON cases (connect_contact_id);

-- Dedup / linking of repeat calls for the same incident
CREATE TABLE case_links (
  case_id        UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
  linked_case_id UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
  reason         TEXT,
  PRIMARY KEY (case_id, linked_case_id),
  CHECK (case_id <> linked_case_id)
);

CREATE TABLE case_locations (
  case_id       UUID PRIMARY KEY REFERENCES cases(id) ON DELETE CASCADE,
  geog          GEOGRAPHY(Point,4326),                    -- lat/lng
  accuracy_m    NUMERIC(8,2),
  address_text  TEXT,
  what3words    TEXT,
  road_name     TEXT,
  location_type TEXT,                                     -- motorway/urban/tunnel/carpark...
  country_code  CHAR(2) REFERENCES countries(code),
  region        TEXT,
  city          TEXT,
  postcode      TEXT,
  capture_method TEXT,                                    -- sms_link / verbal / coarse_cell
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_case_loc_geog ON case_locations USING GIST (geog);

CREATE TABLE case_safety (
  case_id              UUID PRIMARY KEY REFERENCES cases(id) ON DELETE CASCADE,
  is_everyone_safe     BOOLEAN,
  in_live_traffic      BOOLEAN,
  on_hard_shoulder     BOOLEAN,
  vulnerable_occupants BOOLEAN,
  weather              TEXT,
  is_dark              BOOLEAN,
  emergency_services_needed BOOLEAN,
  emergency_services_called BOOLEAN,
  emergency_reference  TEXT
);

-- ---------------------------------------------------------------------------
-- DISPATCH MISSIONS
-- ---------------------------------------------------------------------------
CREATE TABLE missions (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id            UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
  provider_id        UUID REFERENCES providers(id) ON DELETE SET NULL,
  connector_id       UUID REFERENCES provider_connectors(id) ON DELETE SET NULL,
  service            required_service NOT NULL,
  source             dispatch_source NOT NULL DEFAULT 'api',
  external_mission_id TEXT,
  status             mission_status NOT NULL DEFAULT 'searching',
  eta_minutes        INT,
  provider_location  GEOGRAPHY(Point,4326),
  destination_type   TEXT,
  destination_address TEXT,
  tow_distance_km    NUMERIC(8,2),
  estimated_cost     NUMERIC(10,2),
  excess_payable     NUMERIC(10,2),
  idempotency_key    TEXT UNIQUE,                          -- prevents double-dispatch
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_missions_case ON missions (case_id);

-- normalized status timeline from provider webhooks
CREATE TABLE mission_status_events (
  id           BIGSERIAL PRIMARY KEY,
  mission_id   UUID NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
  status       mission_status NOT NULL,
  raw_status   TEXT,
  eta_minutes  INT,
  payload      JSONB,
  occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_mstatus_mission ON mission_status_events (mission_id, occurred_at);

-- driver-trust info shared with the customer
CREATE TABLE mission_driver (
  mission_id    UUID PRIMARY KEY REFERENCES missions(id) ON DELETE CASCADE,
  driver_name   TEXT,
  vehicle_plate TEXT,
  photo_url     TEXT,
  phone_e164    TEXT
);

-- ---------------------------------------------------------------------------
-- NOTIFICATIONS, CALL RECORDINGS, INTERACTION LOG
-- ---------------------------------------------------------------------------
CREATE TABLE notifications (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id        UUID REFERENCES cases(id) ON DELETE CASCADE,
  channel        notification_channel NOT NULL,
  recipient      TEXT NOT NULL,
  template       TEXT NOT NULL,
  status         notification_status NOT NULL DEFAULT 'queued',
  provider_ref   TEXT,                                    -- Pinpoint message id
  status_link_token TEXT,                                 -- tokenized, no PII in URL
  link_expires_at TIMESTAMPTZ,
  sent_at        TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_notifications_case ON notifications (case_id);

CREATE TABLE call_recordings (
  id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  case_id            UUID REFERENCES cases(id) ON DELETE SET NULL,
  connect_contact_id TEXT,
  s3_uri             TEXT NOT NULL,
  consent_captured   BOOLEAN NOT NULL DEFAULT FALSE,
  retention_expiry   DATE,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE interaction_log (
  id           BIGSERIAL PRIMARY KEY,
  case_id      UUID NOT NULL REFERENCES cases(id) ON DELETE CASCADE,
  operator_id  UUID REFERENCES staff(id) ON DELETE SET NULL,
  event_type   TEXT NOT NULL,                              -- note/call_event/dispatch/status
  note         TEXT,
  occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ilog_case ON interaction_log (case_id, occurred_at);

-- ---------------------------------------------------------------------------
-- IMMUTABLE HASH-CHAINED AUDIT LEDGER (tamper-evident)
-- Append-only; each row chains to the previous via SHA-256. No UPDATE/DELETE.
-- ---------------------------------------------------------------------------
CREATE TABLE audit_ledger (
  seq          BIGSERIAL PRIMARY KEY,
  event_type   TEXT NOT NULL,
  actor        TEXT NOT NULL,
  payload      JSONB NOT NULL,
  occurred_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  prev_hash    TEXT,
  entry_hash   TEXT NOT NULL
);

CREATE OR REPLACE FUNCTION audit_ledger_chain() RETURNS TRIGGER AS $$
DECLARE
  last_hash TEXT;
BEGIN
  SELECT entry_hash INTO last_hash FROM audit_ledger ORDER BY seq DESC LIMIT 1;
  NEW.prev_hash := last_hash;
  NEW.entry_hash := encode(digest(
      coalesce(last_hash,'') || NEW.event_type || NEW.actor ||
      NEW.payload::text || NEW.occurred_at::text, 'sha256'), 'hex');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_ledger_chain
  BEFORE INSERT ON audit_ledger
  FOR EACH ROW EXECUTE FUNCTION audit_ledger_chain();

CREATE OR REPLACE FUNCTION audit_ledger_immutable() RETURNS TRIGGER AS $$
BEGIN
  RAISE EXCEPTION 'audit_ledger is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_audit_ledger_no_update
  BEFORE UPDATE OR DELETE ON audit_ledger
  FOR EACH ROW EXECUTE FUNCTION audit_ledger_immutable();

-- ---------------------------------------------------------------------------
-- PRODUCTION-ACCESS GRANTS (JIT, product-owner approved) — mirrored to the ledger
-- ---------------------------------------------------------------------------
CREATE TABLE prod_access_grants (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor          TEXT NOT NULL,                            -- who received access
  reason         prod_access_reason NOT NULL,
  change_ticket  TEXT,                                     -- linked change/incident id
  approved_by    TEXT NOT NULL,                            -- product_owner identity
  scope          TEXT NOT NULL,
  granted_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at     TIMESTAMPTZ NOT NULL,
  revoked_at     TIMESTAMPTZ,
  ledger_seq     BIGINT REFERENCES audit_ledger(seq),
  CHECK (approved_by <> actor)                             -- no self-approval
);
CREATE INDEX idx_grants_actor ON prod_access_grants (actor);

-- ---------------------------------------------------------------------------
-- updated_at maintenance
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS TRIGGER AS $$
BEGIN NEW.updated_at := now(); RETURN NEW; END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_customers_touch  BEFORE UPDATE ON customers          FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_cases_touch      BEFORE UPDATE ON cases              FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_missions_touch   BEFORE UPDATE ON missions           FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_connectors_touch BEFORE UPDATE ON provider_connectors FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

COMMIT;
