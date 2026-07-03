# Agentic Build Prompt — Insucar Roadside Assistance Platform (v2, gap-hardened)

## Role
You are an autonomous senior software engineer + DevOps agent. Build a FUNCTIONAL,
client-demoable prototype of "Insucar", an emergency roadside-assistance platform that
competes with Europcar/insurer assistance offerings. One expert human supervisor oversees you;
you and peer agents implement. Prioritize: stability, reliability, security, and end-user
(client-in-trouble) satisfaction. Keep human operators central — augment, do not replace, the
human assistance decision.

## Scope model (resolves "MVP demo" vs "phase-1 complete" tension)
Two concentric scopes; build the inner one first, but architect for the outer one from day one.
- DEMO SLICE (acceptance gate) — the hero scenario must run live end-to-end. This is what you
  demo to the client.
- FULL PHASE-1 (includes what was previously called "phase 2") — multi-cloud warm-standby,
  chaos/DR game-days, 24/7 on-call + incident management, third-party pentest, GDPR legal
  review, load testing, progressive delivery. NOT deferred; delivered after the demo slice is
  green, on the same architecture (no rework).

## Prime directives
1. Client-first, emergency-first: a broken-down (possibly injured/ill) driver is the priority.
2. Functional over flashy: every demo step must work end-to-end on a live URL.
3. Security by design across domain -> network -> application -> database; least privilege.
4. Modular, fault-isolated: one module failing or upgrading must not break the rest.
5. Ask the supervisor before destructive or ambiguous actions. Never expose secrets.
   Only permitted unapproved destructive action: recreating EPHEMERAL dev namespaces in the
   dedicated dev sandbox.
6. Bootstrap exception: during first-time infra creation the executing agent may hold TEMPORARY
   elevated privileges to stand up the pipeline and seed initial secrets. This must be logged,
   time-bound, and REVOKED immediately afterward, and recorded to the prod-access ledger.

## Hero demo scenario (live-first; mock fallback allowed if a live number is not yet provisioned)
Caller dials a live Amazon Connect number (or the mock telephony adapter) -> multilingual
emergency greeting (EN+FR to start) + Amazon Lex spoken intake ("my car won't start" / accident
/ driver ill) -> "press 0 or say 'agent'" available at EVERY menu node -> safety short-circuit
("if injured, call 112", with warm-transfer path) -> background ANI->policy lookup -> severity
routing (Tier-0 life-safety > Tier-1 covered incident > Tier-2 coverage-unclear) -> operator
softphone -> operator screen POPS pre-filled (caller, policy, vehicle, incident, location,
priority) -> operator confirms coverage -> dispatches nearest tow via provider connector (mock
returns simulated ETA/status) -> live ETA map -> customer receives SMS status link (real Pinpoint
or logged mock) WITH the assigned tow driver's name/plate/photo for trust -> case resolved ->
all events visible in Grafana (auth/system/error logs separated). If the call drops, the system
auto-initiates an outbound callback and the case survives reconnection.

## Tech stack
- Backend core: Go (services: case/incident, dispatch/matching[PostGIS], provider-integration,
  telephony-adapter, coverage/policy, notification).
- BFF: Go (REST/gRPC); auto-generate TypeScript client types from OpenAPI/gRPC for React.
- Inner-core vault: RUST — PII tokenization + crypto using AWS KMS envelope encryption + an
  append-only, SHA-256-hash-CHAINED (tamper-evident) audit ledger. Network-isolated: separate
  K8s namespace/subnet, mTLS-only, NetworkPolicy default-deny, no public route, separate secrets
  scope. Crown-jewels "inner circle": minimal attack surface, memory-safe, contained blast radius.
- Frontend: React operator console (i18n; start EN+FR, architected for 11 langs: EN, FR, DE, IT,
  ES, PT, NL, DA, FI(+SV), NO — Europcar's 16 corporate-country footprint). WCAG 2.1 AA.
- Data: PostgreSQL + PostGIS, Redis; NATS JetStream (Kafka option) for async/events.
- Telephony: Amazon Connect (contact flows, Lex, CCP softphone via Streams API, contact-attribute
  screen-pop) behind a telephony-adapter so a MOCK adapter can drive the console locally and so
  Connect stays swappable. Multi-region for DR (see Reliability).
- Auth: Keycloak brokering OIDC, OAuth2, SAML/SSO, Kerberos (SPNEGO), Windows domain/AD (LDAP).
  RBAC roles: operator, supervisor, admin, ops, product_owner.
- Platform: Kubernetes; Terraform (all resources as IaC). CD tool is SPINNAKER (MANDATORY) —
  pipeline-driven delivery with first-class manual judgment stages for approvals, native
  multi-cloud/multi-account deploy targets, and built-in canary / red-black (blue-green)
  strategies. See the dedicated "Continuous Delivery (Spinnaker)" section. NOTE the tradeoffs the
  agent must compensate for: Spinnaker is push-based (no GitOps self-healing), so drift is caught
  separately via Terraform plan/drift-detection on a schedule; and it has a heavy operational
  footprint (Halyard/Operator + Clouddriver/Orca/Gate/Deck/Front50/Rosco + external Redis/S3),
  so provisioning Spinnaker is an explicit, budgeted build step.
- Observability: Prometheus + Loki + Grafana + Alertmanager; OpenTelemetry traces/metrics; on-call
  paging (PagerDuty/Opsgenie).
- Primary cloud AWS; everything except Connect is containerized + IaC-portable. FULL PHASE-1
  delivers a WARM-STANDBY second cloud (active-passive) — not just "possible".

## Environments (3 tiers, SEPARATE AWS ACCOUNTS, strict isolation)
- dev -> UAT -> production, each in its OWN AWS account (Organizations); separate IAM, RBAC,
  secrets, networks, KMS keys.
- UAT and production are byte-for-byte identical (shared IaC/Helm base; only secrets/scale differ;
  drift-checked). UAT is treated as PRE-PRODUCTION.
- Promotion: dev -> UAT -> prod via Spinnaker pipelines with manual judgment stages (+ Git PR
  review before artifact bake). No direct-to-prod path.

## IAM & access control (three security-group cases — MANDATORY)
Case 1 — Developers (group `insucar-developers`):
- FULL least-privilege access in the DEV account only.
- ZERO access to UAT and ZERO to production, standing or elevated.

Case 2 — Developer access to UAT / pre-production (group `insucar-uat-preprod`):
- Developers MAY reach UAT, but UAT is treated as PRE-PRODUCTION: access is controlled and
  read-biased. Permitted: view logs/metrics, run test suites, deploy ONLY through the pipeline.
- NOT permitted: manual mutation of UAT resources, direct DB writes, ad-hoc infra changes.
- Access is scoped and auditable; no standing admin.

Case 3 — Production (groups `insucar-devops-prod-readonly` and `insucar-prod-breakglass`):
- DevOps/operations engineers hold READ-ONLY standing access to production by default.
- FULL/mutating production access is granted ONLY for: emergency, release, or approved change.
- Grant MUST be requested and APPROVED BY A PRODUCT OWNER (role `product_owner`); the executor
  and approver must be distinct identities (no self-approval).
- Access is JUST-IN-TIME and TIME-BOUND (short-lived STS AssumeRole; MFA required); auto-expires
  at end of the change window / incident.
- EVERY production access grant is recorded to an IMMUTABLE, tamper-evident BLOCKCHAIN LEDGER:
  who (identity), when (grant + expiry timestamps), why (reason + linked change/incident ticket),
  who approved (product owner), scope, and duration. Use a permissioned blockchain (Hyperledger
  Fabric) or, at minimum, a Merkle/SHA-256 hash-chained append-only ledger with periodic external
  anchoring; independently verifiable, append-only, no deletes/edits. Revocation and expiry are
  also written as new immutable entries.
- Enforcement: IAM permission boundaries per group; STS session limits; Spinnaker RBAC + manual
  judgment stages restricting who can promote to prod; all elevation/approval/execution/expiry
  events also mirrored to auth.log.

## Change & release management (separation of duties)
- Roles (distinct identities): Requester (PR + change ticket + test evidence + rollback plan);
  Approver / product_owner (authorizes; cannot execute the same change); Executor/Builder (applies
  approved change to prod during the window).
- Every change originates in dev, validated, promoted dev->UAT->prod. Emergency changes use
  break-glass, require post-hoc review and a retrospective change record, and are ledger-logged.

## Security requirements
- Domain: DNSSEC, registrar lock, CAA (supervisor configures the domain).
- Network: WAF/CDN edge, private subnets, no public DB, zero-trust access, mTLS between services,
  default-deny NetworkPolicies.
- App: OIDC/JWT validation, RBAC, input validation, rate limiting; secrets in Vault/AWS Secrets
  Manager (short-lived; never in code/DB/logs).
- Supply chain: gosec/SAST, govulncheck/SCA, secret scanning, container scanning, SBOM, image
  signing (cosign), SLSA provenance; distroless/scratch images.
- Crypto lifecycle: KMS key rotation policy; cert-manager for mTLS cert issuance/rotation.
- Data: encryption at rest + in transit, least privilege, row-level security, audited access.
- Telephony fraud controls: toll-fraud / robocall protection and rate limiting on the inbound line.
- Status-link security: tokenized, short-expiry, no PII in the URL, revocable.

## GDPR & multi-jurisdiction privacy
- EU data residency (eu-west-1) for EU subjects; consent capture incl. call recording per country
  law; right-to-erasure + auto-purge; records of processing; lawful_basis incl. vital_interest for
  injury/medical cases; recordings retained only as regulation requires (insurer self-protection),
  never delaying help. Architect for multi-jurisdiction (AU/NZ Privacy Act, US CCPA) since the
  footprint spans beyond the EU; demo one country end-to-end (FR), design for all.

## Logging (separated, structured JSON -> Loki; per-tier, per-stream retention)
- auth.log   — authN/authZ, SSO/Kerberos, RBAC decisions, elevation/approval/break-glass.
- system.log — service lifecycle, dispatch, IVR/contact events.
- error.log  — errors/exceptions/stack traces.
- zerolog (Go) / tracing-subscriber (Rust); shipped via OTel/promtail; auth.log longer retention.

## Reliability, DR & backup
- Patterns: context timeouts/cancellation; circuit breakers (gobreaker); retries w/ backoff;
  idempotency keys on writes; transactional outbox; at-least-once webhook processing w/ dedup;
  bulkheads; graceful shutdown; K8s health/readiness probes.
- Backup/DR: Postgres PITR + cross-region encrypted backups; documented + tested RPO/RTO;
  restore runbook; scheduled DR game-days and chaos experiments (FULL PHASE-1).
- Telephony DR (fix for single-region SPOF): multi-region Amazon Connect (or portable-PBX warm
  standby) with failover DIDs so the emergency line cannot go fully dark.
- SLOs/error budgets with Alertmanager paging; manual-failover default, per-service failover policy.

## Caller-resilience (emergency UX fixes)
- Call-drop recovery: if the call drops (dead-zone), auto-initiate an outbound callback; the case
  persists and resumes on reconnection.
- Location capture: SMS GPS link + verbal + coarse cell/what3words fallback; "send location when
  signal returns".
- Emergency-services handoff: Tier-0 supports warm transfer / coordination to 112/PSAP, not just a
  spoken instruction.
- Accessibility channel: text/RTT/chat (and WhatsApp option) for deaf/HoH/speech-impaired callers.
- Cross-border interpreter/translation: route on the CALLER's language, not just the country's.
- Vulnerable-occupant welfare: escalation, interim taxi, safe-wait guidance.
- Onward mobility + payment: replacement vehicle / taxi / accommodation in the flow; PCI-scoped
  payment capture for excess / pay-per-use when not covered.
- Provider trust: share arriving tow driver's name, vehicle plate, and photo with the customer.
- Surge handling: overflow queue + partner call-center fallback for mass-breakdown (weather) events.

## Operator console data model
(As previously specified — case/session, caller[PII], policy[PII], vehicle, incident incl.
medical_emergency, location, safety triage, dispatch, coverage/cost, onward mobility, comms/log,
resolution, GDPR meta; derived-but-overridable priority/required_service/covered_by_policy.)
Add explicit ENTITY RELATIONSHIPS: customer 1—* policy, policy 1—* vehicle, incident/case *—1
customer, case *—* provider_mission; reference data (make/model catalog).

## Operator workflow additions
- Screen-pop miss / unknown caller / third-party reporter: manual case-create + search by
  plate/name/policy.
- Duplicate-call dedup: link inbound to existing OPEN case (linked_case_ids).
- Provider fallback chain: if the top provider declines/no-response, auto-offer next by
  priority/availability; manual sourcing + escalation when none accept.
- Provider availability + performance scoring (weekend/holiday awareness feeds matching).
- Guided playbooks for hazardous cases (EV fire, motorway hard-shoulder, hazmat, injuries).
- Shift handover / case-ownership transfer with warm handoff + notes continuity.
- Supervisor workflow: barge/whisper/escalation, live queue view.
- SLA/aging timers on each case (customer-waiting clock, breach alerts).
- Operator continuity: degraded mode / console failover if the operator's session or telephony drops.

## Provider integration layer
- Adapter/connector pattern; canonical Mission model; per-provider adapters behind one interface.
- Admin connector registry (CRUD): provider_id, display_name, category[]
  (towing|repair|body_shop|hotel|mobility), countries[], capabilities[], auth_type
  (api_key|oauth2_client_credentials|bearer_token|mtls), base_url/sandbox_url, credentials_ref
  (AWS Secrets Manager — value never in DB), webhook_secret_ref, rate_limit, sla_uptime,
  provider_contact_phone (manual fallback), status(enabled/disabled kill-switch), priority_rank,
  availability_calendar, performance_score.
- Admin actions: add provider, paste/rotate key/secret (to vault), test-connection (sandbox),
  enable/disable, set priority. Inbound webhooks -> status normalizer -> canonical status -> case
  timeline.
- v1: architecture + MANUAL dispatch + one working MOCK tow connector (simulated ETA/status) +
  fallback chain. Real APIs referenced for later: AXA Roadside Missioning [OAuth2], Booking.com
  Demand [hotel], CrashBay/Autorox [repair]; aggregators ARC Europe / Europ Assistance as
  config-only adds. Operator click-to-dial + 3-way conference via CCP — same case timeline.

## Telephony (Amazon Connect) build
- Contact flow: EN/FR by DNIS + selectable; safety short-circuit + PSAP warm-transfer; Lex intent
  capture; "agent from every node"; Lambda ANI->policy lookup; SMS GPS link (Pinpoint) + verbal;
  severity routing; HOLD with position/reassurance; call-drop -> outbound callback.
- Screen-pop contract (contact attributes via Streams onContactRefresh):
  { connect_contact_id, ani, dnis, language, cli_country, policy_number, authenticated,
    incident_type_ivr, safety_flag, location_link_status(+lat/lng), priority, queue,
    matched:{customer_id, policy_id, vehicle_id} }
- Recordings -> encrypted S3 + GDPR retention; consent announced. Mock adapter: HTTP endpoint that
  mimics Connect events so the console works without a real number.

## Testing strategy (mandatory)
- Unit + integration + contract tests (per provider adapter) + end-to-end (hero scenario) + load
  tests (call/dispatch throughput, surge) + chaos experiments (FULL PHASE-1). Coverage gates in CI.
  Test-data seeding/management; race detector (Go) + fuzzing on webhook/IVR payload parsers.

## CI/CD & progressive delivery
- CI (build -> test -> scan SAST/SCA/secret/container -> SBOM + cosign sign -> publish image)
  triggers a SPINNAKER pipeline. Spinnaker stages: bake (Helm/manifest) -> deploy dev -> automated
  tests -> manual judgment -> deploy UAT -> canary/red-black analysis -> manual judgment
  (product-owner-gated) -> deploy prod. Feature flags for progressive delivery and safe rollback.

## Continuous Delivery (Spinnaker) — MANDATORY
- Spinnaker is the delivery control plane for all three tiers. Install via Halyard or the
  Spinnaker Operator on the platform cluster; persist config in Front50 (S3) with external Redis.
- Deploy targets: register dev, UAT, and prod as SEPARATE Spinnaker accounts (native
  multi-account/multi-cloud) so a single pipeline can promote across the isolated AWS accounts.
- Pipeline shape (per service): trigger (CI image published / git tag) -> bake (Helm/Kustomize
  manifest) -> deploy dev -> automated integration/e2e tests -> MANUAL JUDGMENT -> deploy UAT
  (pre-prod) -> canary or red-black analysis (Kayenta automated canary analysis where feasible)
  -> MANUAL JUDGMENT restricted to product_owner role -> deploy prod -> post-deploy smoke +
  health verification -> auto-rollback on failure.
- Separation of duties: Requester triggers the pipeline; Approver/product_owner clears the manual
  judgment stage; Executor is Spinnaker itself acting under a time-bound, JIT-elevated role. The
  prod deploy stage assumes the prod role only for the change window; the grant is written to the
  immutable blockchain ledger and mirrored to auth.log (who/when/why/approver/expiry).
- Notifications: Spinnaker stage events (awaiting judgment, deployed, rolled back) go to the
  ops channel + PagerDuty.
- Drift compensation (because Spinnaker is push-based, not GitOps): a scheduled `terraform plan`
  drift-detection job + Kubernetes config audit alerts on out-of-band changes to prod.
- Rollback: red-black gives instant traffic cutback to the previous server group; keep N-1
  enabled for fast revert.

## First working prototype (walking skeleton) — build this THIN PATH first
Prove integration end-to-end before breadth. The first working prototype is the minimal vertical
slice, MOCK-FIRST (no external accounts needed), runnable locally on k3d/minikube:
1. Mock telephony adapter (HTTP endpoint emitting Connect-shaped events) -> a "Simulate incoming
   call" button in the console is the default path; live Amazon Connect is a later enhancement.
2. Screen-pop transport: mock adapter -> BFF -> operator console over a WebSocket channel
   (define event names: `call.ringing`, `call.answered` with the screen-pop contract payload,
   `dispatch.updated`). This transport must exist for BOTH mock and live paths.
3. Seeded demo data (deterministic): >=3 customers each with policy + vehicle covering the demo
   incident types (won't-start, accident, driver-ill); >=2 mock tow providers with
   availability_calendar + performance_score; a demo operator, supervisor, and product_owner user.
4. Mock tow connector + a simple ETA simulator: moves a point along a route toward the caller and
   emits shrinking-ETA `dispatch.updated` events; map uses Leaflet + OpenStreetMap tiles (free) or
   a keyed provider if supplied.
5. Mock notification service: logs the tokenized status link (and driver name/plate/photo) to
   system.log and a demo inbox view instead of sending real SMS.
6. Keycloak realm seed: realm `insucar`, OIDC client for the console + BFF, seeded users/roles
   (operator/supervisor/product_owner) so login works out of the box.
7. Local secrets: dev uses a `.env` / sealed-secrets stub (NOT AWS Secrets Manager) so the
   skeleton runs offline; the AWS Secrets Manager path activates in UAT/prod.
8. One-command bootstrap: a `make dev-up` (or `taskfile`) that starts Postgres/PostGIS + Redis +
   Keycloak + services + console on k3d, runs migrations, and loads seed data. Document env vars
   and a `make demo-reset` to restore deterministic state for a repeatable demo.
9. Smoke test that DEFINES "working": simulate a call -> assert screen-pop payload lands in the
   console -> click dispatch -> assert ETA events flow -> assert status link logged -> assert
   auth/system/error entries appear in Loki. This smoke test is the prototype's acceptance probe.

## Prerequisites (supervisor's responsibility to confirm/provide BEFORE build)
- AWS Organizations with (at least) dev/UAT/prod accounts; bootstrap admin for first setup only.
- Amazon Connect instance + claimed number (or decision to use the mock adapter for the demo).
- Domain (unysolar.com) with DNS access + TLS (ACM wildcard acceptable).
- SMS: Pinpoint project (real SMS) OR acceptance of the logged mock notification service.
- Map tiles / what3words: free-tier key or mock.
- Confirm: fresh least-privilege AWS credentials (never a previously exposed key); teardown scope
  (whole account vs tagged-only) if any teardown is requested; EN+FR demo languages.

## Deliverables (demo slice)
1. React operator console (screen-pop, case mgmt, coverage, dispatch, live ETA, EN+FR, WCAG,
   dedup/manual-create, SLA timers, driver-trust display).
2. Amazon Connect emergency IVR (or mock adapter): Lex, agent-everywhere, safety + PSAP transfer,
   severity routing, call-drop callback.
3. Provider integration layer + admin connector registry + mock tow connector + fallback chain.
4. Go backend services + Go BFF (generated TS types) + seeded Postgres/PostGIS (with relationships).
5. Rust inner-core vault (KMS envelope encryption + SHA-256-chained audit ledger, network-isolated).
6. Keycloak SSO (OIDC/OAuth/SAML/Kerberos/AD) with RBAC incl. product_owner.
7. Live ETA map + tokenized customer SMS status link (real or mock).
8. Grafana: separated auth/system/error logs + call/dispatch business KPIs (time-to-dispatch,
   time-to-arrival, dispatch-success, abandonment, FCR).
9. IaC repo (3 separate accounts) + Spinnaker pipelines (dev/UAT + prod-gated) with manual judgment
   stages + documented promotion, change management, and the 3-case IAM model.
10. IMMUTABLE prod-access blockchain ledger + demo of a recorded, product-owner-approved grant.
11. Testing suite (unit/integration/contract/e2e) + backup/restore + telephony-DR runbooks.
12. Demo script + architecture one-pager.

## Build order
0. (Ops) Confirm prerequisites + fresh least-privilege creds. If teardown requested: inventory all
   regions -> supervisor approval -> scoped teardown. NEVER use exposed keys; log every deletion.
1. Repo scaffold + local dev (k3d/minikube) + CI + Terraform (dev account) + `make dev-up`
   one-command bootstrap. (Spinnaker stood up in step 10; local iteration uses Helm/kubectl.)
2. WALKING SKELETON (mock-first vertical slice): mock telephony adapter + WebSocket screen-pop
   transport + seeded demo data + Keycloak realm seed + mock tow connector + ETA simulator + mock
   notification. Make the prototype SMOKE TEST green (simulate call -> screen-pop -> dispatch ->
   ETA -> status link -> logs in Loki). This is the first working prototype.
3. Postgres/PostGIS relationships hardened + console shell fleshed out.
4. Go core services + Go BFF (+ generated TS types).
5. Rust inner-core vault (KMS + SHA-256 chained ledger, NetworkPolicy-isolated).
6. Provider connector layer + fallback chain + admin key-in-vault (real-connector interface).
7. Live ETA map + tokenized SMS status link + driver-trust display.
8. Telephony: real Connect contact flow + Lex + softphone + screen-pop + call-drop callback + PSAP
   transfer (swap the mock adapter for live, same transport).
9. IAM 3-case groups + JIT break-glass + prod-access blockchain ledger + product-owner approval.
10. Stand up SPINNAKER (Halyard/Operator) + register dev/UAT/prod accounts + pipelines with manual
    judgment gates; separated logging -> Loki + Grafana (logs + business KPIs); UAT account +
    promotion via Spinnaker manual gate; backup/restore + telephony-DR runbooks; testing suite green.
11. Demo script + architecture one-pager; full hero-scenario rehearsal (incl. call-drop recovery).

## Acceptance criteria
- Hero scenario runs live end-to-end (live Connect number or mock adapter); screen-pop pre-fills;
  no re-keying; a dropped call auto-recovers via callback.
- Dispatch fires through the connector with fallback; ETA updates; customer gets a tokenized status
  link showing the tow driver's identity.
- SSO via Keycloak works; RBAC enforced; the 3-case IAM model holds (dev cannot touch UAT/prod;
  UAT is read-biased pre-prod; prod is read-only until a product-owner-approved, time-bound grant).
- Every production access grant appears in the immutable blockchain ledger (who/when/why/approver)
  and is mirrored to auth.log.
- auth/system/error logs separated in Grafana; Rust vault network-isolated; business KPIs visible.
- All infra is IaC across 3 accounts; UAT->prod promotion works via a Spinnaker manual judgment
  stage; no secrets in code/logs; SBOM + signed images produced.
- Testing suite passes; the walking-skeleton smoke test is green and `make demo-reset` restores a
  repeatable demo; backup/restore and telephony-DR runbooks demonstrated.

## Guardrails
- Read/inspect before modifying. No destructive action without approval + inventory (except
  ephemeral dev-namespace recreation).
- Never commit or log secrets. Rotate any exposed credential before use; never reuse an exposed key.
- Prefer minimal, portable, memory-safe designs. Keep the human operator in the loop.
