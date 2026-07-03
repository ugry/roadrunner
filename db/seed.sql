-- Insucar — deterministic demo seed (real-shaped records, not mocks).
-- Enough to drive the hero scenario: ANI->policy lookup, coverage, vehicles, providers, dispatch.
-- Run AFTER schema.sql against the same database.

BEGIN;

-- Reference: countries + primary language
INSERT INTO countries (code, name, default_language) VALUES
  ('FR','France','fr'),
  ('GB','United Kingdom','en'),
  ('DE','Germany','de')
ON CONFLICT DO NOTHING;

-- Staff (mirror of Keycloak subjects)
INSERT INTO staff (id, keycloak_subject, display_name, email, role, languages) VALUES
  ('11111111-1111-1111-1111-111111111111','kc-operator-01','Amelie Durand','operator@insucar.demo','operator','{fr,en}'),
  ('22222222-2222-2222-2222-222222222222','kc-supervisor-01','Marc Petit','supervisor@insucar.demo','supervisor','{fr,en}'),
  ('33333333-3333-3333-3333-333333333333','kc-po-01','Sophie Bernard','po@insucar.demo','product_owner','{fr,en}')
ON CONFLICT DO NOTHING;

-- Customers (registered end users) — phone anchors the ANI lookup
INSERT INTO customers (id, email, phone_e164, first_name, last_name, preferred_language, country_code, status, email_verified, phone_verified, keycloak_subject) VALUES
  ('aaaaaaaa-0000-0000-0000-000000000001','claire.martin@example.fr','+33600000001','Claire','Martin','fr','FR','active',TRUE,TRUE,'kc-cust-01'),
  ('aaaaaaaa-0000-0000-0000-000000000002','john.smith@example.co.uk','+447700900002','John','Smith','en','GB','active',TRUE,TRUE,'kc-cust-02'),
  ('aaaaaaaa-0000-0000-0000-000000000003','lukas.mueller@example.de','+491600000003','Lukas','Mueller','de','DE','active',TRUE,TRUE,'kc-cust-03')
ON CONFLICT DO NOTHING;

-- Consents captured at registration
INSERT INTO consents (customer_id, purpose, granted, basis) VALUES
  ('aaaaaaaa-0000-0000-0000-000000000001','terms',TRUE,'contract'),
  ('aaaaaaaa-0000-0000-0000-000000000001','privacy',TRUE,'consent'),
  ('aaaaaaaa-0000-0000-0000-000000000001','call_recording',TRUE,'consent');

-- Policies
INSERT INTO policies (id, policy_number, customer_id, product, coverage, status, valid_from, valid_to, excess_amount, callout_limit) VALUES
  ('bbbbbbbb-0000-0000-0000-000000000001','INS-FR-1001','aaaaaaaa-0000-0000-0000-000000000001','comprehensive','premium','active','2026-01-01','2026-12-31',0,NULL),
  ('bbbbbbbb-0000-0000-0000-000000000002','INS-GB-2002','aaaaaaaa-0000-0000-0000-000000000002','roadside_only','standard','active','2026-01-01','2026-12-31',50,5),
  ('bbbbbbbb-0000-0000-0000-000000000003','INS-DE-3003','aaaaaaaa-0000-0000-0000-000000000003','membership','basic','active','2026-01-01','2026-12-31',25,3)
ON CONFLICT DO NOTHING;

INSERT INTO policy_territories (policy_id, country_code) VALUES
  ('bbbbbbbb-0000-0000-0000-000000000001','FR'),
  ('bbbbbbbb-0000-0000-0000-000000000001','DE'),
  ('bbbbbbbb-0000-0000-0000-000000000002','GB'),
  ('bbbbbbbb-0000-0000-0000-000000000003','DE')
ON CONFLICT DO NOTHING;

INSERT INTO policy_entitlements (policy_id, entitlement, included) VALUES
  ('bbbbbbbb-0000-0000-0000-000000000001','towing',TRUE),
  ('bbbbbbbb-0000-0000-0000-000000000001','roadside_repair',TRUE),
  ('bbbbbbbb-0000-0000-0000-000000000001','replacement_vehicle',TRUE),
  ('bbbbbbbb-0000-0000-0000-000000000002','towing',TRUE),
  ('bbbbbbbb-0000-0000-0000-000000000002','roadside_repair',TRUE),
  ('bbbbbbbb-0000-0000-0000-000000000003','towing',TRUE)
ON CONFLICT DO NOTHING;

-- Vehicles
INSERT INTO vehicles (id, customer_id, license_plate, country_code, make, model, year, color, fuel, transmission, category) VALUES
  ('cccccccc-0000-0000-0000-000000000001','aaaaaaaa-0000-0000-0000-000000000001','AB-123-CD','FR','Renault','Clio',2021,'blue','petrol','manual','car'),
  ('cccccccc-0000-0000-0000-000000000002','aaaaaaaa-0000-0000-0000-000000000002','LT21 XYZ','GB','Tesla','Model 3',2022,'white','ev','automatic','car'),
  ('cccccccc-0000-0000-0000-000000000003','aaaaaaaa-0000-0000-0000-000000000003','M-IN-4567','DE','Volkswagen','Transporter',2020,'grey','diesel','manual','van')
ON CONFLICT DO NOTHING;

INSERT INTO policy_vehicles (policy_id, vehicle_id) VALUES
  ('bbbbbbbb-0000-0000-0000-000000000001','cccccccc-0000-0000-0000-000000000001'),
  ('bbbbbbbb-0000-0000-0000-000000000002','cccccccc-0000-0000-0000-000000000002'),
  ('bbbbbbbb-0000-0000-0000-000000000003','cccccccc-0000-0000-0000-000000000003')
ON CONFLICT DO NOTHING;

-- Providers (real connector targets; secrets live in AWS Secrets Manager, referenced by ARN)
INSERT INTO providers (id, display_name, categories, countries, contact_phone, priority_rank, performance_score) VALUES
  ('dddddddd-0000-0000-0000-000000000001','AXA Roadside FR','{towing,repair}','{FR,DE}','+33100000000',10,4.6),
  ('dddddddd-0000-0000-0000-000000000002','Towpal UK','{towing}','{GB}','+441000000000',20,4.2)
ON CONFLICT DO NOTHING;

INSERT INTO provider_connectors (provider_id, auth_type, base_url, sandbox_url, credentials_ref, capabilities) VALUES
  ('dddddddd-0000-0000-0000-000000000001','oauth2_client_credentials','https://api.axa-roadside.example','https://sandbox.axa-roadside.example','arn:aws:secretsmanager:eu-west-1:000000000000:secret:insucar/axa','{tow_recovery,roadside_repair,jump_start}'),
  ('dddddddd-0000-0000-0000-000000000002','api_key','https://api.towpal.example','https://sandbox.towpal.example','arn:aws:secretsmanager:eu-west-1:000000000000:secret:insucar/towpal','{tow_recovery,tyre_change,jump_start}')
ON CONFLICT DO NOTHING;

INSERT INTO provider_availability (provider_id, day_of_week, open_time, close_time) VALUES
  ('dddddddd-0000-0000-0000-000000000001',0,'00:00','23:59'),
  ('dddddddd-0000-0000-0000-000000000001',6,'00:00','23:59'),
  ('dddddddd-0000-0000-0000-000000000002',6,'08:00','20:00')
ON CONFLICT DO NOTHING;

-- Ledger sanity entry (verifies hash-chaining works)
INSERT INTO audit_ledger (event_type, actor, payload) VALUES
  ('seed.bootstrap','system', '{"note":"initial demo seed loaded"}');

COMMIT;
