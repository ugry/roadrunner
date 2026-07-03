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

## Hero demo scenario (live, no mocks)
Caller dials a LIVE Amazon Connect number -> multilingual emergency greeting (EN+FR to start) +
Amazon Lex spoken intake ("my car won't start" / accident / driver ill) -> "press 0 or say 'agent'"
available at EVERY menu node -> safety short-circuit ("if injured, call 112", with warm-transfer
path) -> background ANI->policy lookup -> severity routing (Tier-0 life-safety > Tier-1 covered
incident > Tier-2 coverage-unclear) -> operator softphone -> operator screen POPS pre-filled
(caller, policy, vehicle, incident, location, priority) -> operator confirms coverage -> dispatches
nearest tow via a REAL provider connector (real provider sandbox/API returning real status/ETA)
-> live ETA map -> customer receives a REAL SMS status link (Amazon Pinpoint) WITH the assigned tow
driver's name/plate/photo for trust -> case resolved -> all events visible in Grafana
(auth/system/error logs separated). If the call drops, the system auto-initiates an outbound
callback and the case survives reconnection. NO MOCKS in any layer of the delivered prototype.

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
  screen-pop) behind a telephony-adapter so Connect stays swappable/portable. LIVE Connect only —
  no mock adapter in the delivered prototype. Multi-region for DR (see High Availability).
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
- Primary cloud AWS; everything except Connect is containerized + IaC-portable. WITHIN a region the
  cluster is ACTIVE-ACTIVE with automatic node-failover (see High Availability). FULL PHASE-1 adds a
  cross-cloud WARM-STANDBY (active-passive) second cloud — not just "possible".

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
- Surface isolation: end-user and operator apps are separate hosts + separate Keycloak realms +
  separate session cookies (no cross-SSO). The operator surface is non-discoverable (noindex, not
  linked, generic 404 to strangers), MFA-mandatory, and zero-trust/IP-restricted.

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

## High Availability & node-failure resilience (NO single point of failure)
Requirement: if ANY one node fails, another node takes over automatically with no data loss and no
manual intervention. This is achievable — but a naive "2 equal nodes" design CANNOT do it safely,
and that specific pitfall must be engineered out:
- THE 2-NODE QUORUM TRAP: consensus/stateful systems (Postgres auto-failover via Patroni+etcd,
  NATS JetStream, Redis Sentinel) need a MAJORITY to elect a new leader. With exactly two equal
  members, losing one leaves 1 of 2 = no majority -> the survivor will NOT safely promote, or worse,
  both act as primary (SPLIT-BRAIN, data corruption). So "2 nodes, one takes over" is exactly the
  design defect to avoid.
- THE FIX (mandatory): quorum must survive a single-node loss. Use an ODD member count for every
  consensus layer -> minimum THREE members, or TWO data nodes PLUS a lightweight WITNESS/arbiter,
  spread across THREE failure domains (AZs). Then losing any one node keeps a 2/3 majority and
  failover is automatic and safe. Applied per layer:
  - Kubernetes control plane: 3 control-plane nodes (or managed EKS control plane) across 3 AZs.
  - Postgres: Patroni with a 3-member etcd/consensus (2 data + witness), synchronous replication,
    automatic leader election; a floating/service endpoint so clients follow the new primary.
  - NATS JetStream: 3-node cluster, R3 streams (raft quorum).
  - Redis: Sentinel/cluster with 3 voting members.
  - Stateless Go/Rust services + BFF + console: >=2 replicas (>=3 recommended) with
    podAntiAffinity across nodes/AZs behind the LB (active-active); any replica loss is transparent.
- WORKLOAD PLACEMENT: PodDisruptionBudgets, topologySpreadConstraints across AZs, anti-affinity so
  no single node holds a whole tier; LB health-checks evict a dead node in seconds.
- HONEST STATEMENT ON "2 NODES": deliver the compute as active-active, but the underlying HA cluster
  MUST be effectively 3 (2 + witness) for stateful quorum. If the client hard-requires exactly two
  physical nodes, place the witness/arbiter in a third small failure domain (separate AZ / tiny
  instance / managed service) — do NOT ship a quorum-less 2-node stateful cluster.
- MUST-PASS test: chaos-kill any single node during an active call+dispatch -> traffic continues,
  Postgres auto-promotes < 30s, zero committed-data loss (RPO=0 on sync replication), case survives.

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

## End-user registration & self-service (customer-facing)
- Customers self-register (name, email, phone in E.164, preferred language, country) via web/app.
  Auth is delegated to a Keycloak CUSTOMER realm (OIDC); a `password_hash` column exists only for
  dev/offline fallback. Store the Keycloak `sub` on the customer record.
- Verification: email + phone one-time tokens (store token HASH only, with expiry). Phone
  verification matters because the phone number is the ANI lookup anchor during an emergency call.
- Registration captures GDPR consents (terms, privacy, call_recording, marketing opt-in) as
  immutable consent rows with lawful_basis + text hash + ip/user-agent.
- Post-registration the customer adds VEHICLES (plate, make/model, fuel incl. EV, category) and
  links/holds a POLICY; a policy may cover multiple vehicles. This is what makes the operator
  screen-pop pre-fill work (ANI -> customer -> policy + vehicles).
- Self-service: view/update profile, manage vehicles, view policy/entitlements, see case history
  and live status of an active incident. Right-to-erasure request is exposed here.
- Data model is realised in `db/schema.sql` (customers, verification_tokens, consents, policies,
  policy_territories, policy_entitlements, vehicles, policy_vehicles, ...).

## UI surfaces & login separation (two distinct apps)
End users and operators use SEPARATE applications on SEPARATE login pages. They must not share a
login screen, and the operator surface must not be discoverable by end users.
- END-USER app: public, discoverable (e.g. `app.unysolar.com` / `/login`, `/register`). Keycloak
  CUSTOMER realm. Self-service scope only.
- OPERATOR/STAFF app: separate, NON-advertised host/path (e.g. `ops.<internal-domain>` or an
  obscure path), Keycloak STAFF realm. NOT linked from the public site, `noindex/nofollow`, not in
  sitemap. Security is by real controls, not just obscurity: zero-trust/VPN or IP allowlist, WAF,
  mandatory MFA, and RBAC (operator/supervisor/admin/ops/product_owner). An end user hitting the
  operator URL gets a generic 404/hidden response — no hint the surface exists.
- Fully separate: different domains/subdomains, different OIDC clients + realms, different session
  cookies (no shared SSO between customer and staff), different rate-limit and WAF policies.

### End-user REGISTRATION page (fields, from the end-user's perspective)
"I broke down once and never want to be stuck re-explaining who I am — let me register calmly now."
- Account: first name, last name, email (verify via OTP), mobile phone in E.164 with country
  selector (verify via SMS OTP — this phone is the emergency ANI match), password (or social/SSO),
  preferred language, country of residence.
- Consents (explicit checkboxes, stored immutably): accept Terms, Privacy Policy, call-recording
  consent, optional marketing opt-in.
- Add vehicle(s): license plate + plate country, make, model, year, colour, fuel type (incl. EV),
  transmission, category (car/van/motorhome/motorcycle), VIN (optional).
- Policy: enter an existing policy number to link, OR choose/purchase a plan (product + coverage
  level shown with entitlements).
- Optional but valued: home address (for home-start + correspondence), an emergency contact,
  accessibility needs (e.g. wheelchair, hearing — routes to text/RTT channel), child-seat/pet flags.
- Registration UX: minimal required set first (name/email/phone/password/consent), then a guided
  "add your car" and "link your policy" step; progress saved; WCAG 2.1 AA; localized.

### End-user DASHBOARD (post-login, self-service)
- Prominent "I NEED HELP NOW" emergency action (start a case / show the assistance number / request
  a call) — always one tap from anywhere.
- My vehicles (add/edit/remove); My policy & coverage (entitlements, excess, validity, callouts
  used); Case history + LIVE status of an active incident (map, ETA, assigned tow driver
  name/plate/photo, status link).
- Profile & language; consent management; right-to-erasure request; notification preferences.

### OPERATOR CONSOLE screen (what the operator wants to see, from the operator's perspective)
"A call just landed and someone is stranded — show me everything in one glance, let me help fast."
- Incoming-call banner + softphone controls (answer/hold/mute/transfer/3-way conference/hangup,
  recording-pause) bound to the CCP; queue view (waiting count, longest wait, SLA).
- SCREEN-POP header: caller identity, matched customer/policy/vehicle, verification badge, and a
  big PRIORITY/severity indicator; SAFETY panel first (is everyone safe? injuries? fire? live
  traffic?) with a one-click 112/PSAP warm-transfer for Tier-0.
- Case workspace (pre-filled, editable): incident details, location map (live pin + what3words),
  vehicle details, coverage-decision panel (covered? entitlements, excess payable, reason if not),
  and a SUGGESTED required service the operator can override (human-in-the-loop).
- Dispatch panel: nearest providers ranked by availability/performance/ETA, one-click dispatch,
  automatic fallback chain, manual click-to-dial + 3-way conference; live mission status timeline
  and shrinking ETA on the map; button to send the customer a tokenized status link + driver-trust
  info.
- Case timeline / interaction log (auto-stamped call + dispatch events + manual notes), SLA/aging
  timer, dedup indicator (linked open cases), and quick actions: schedule callback, escalate to
  supervisor, request interpreter (caller-language mismatch), arrange onward mobility.
- Everything multilingual to the operator's locale; no re-keying of anything the IVR captured.

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
- v1 (NO MOCKS): architecture + MANUAL dispatch + at least one REAL working tow connector against a
  live provider sandbox/API (e.g. AXA Roadside Missioning [OAuth2] or Towpal [api_key + webhooks])
  returning real status/ETA + fallback chain. Additional real APIs added by config: Booking.com
  Demand [hotel], CrashBay/Autorox [repair]; aggregators ARC Europe / Europ Assistance. Operator
  click-to-dial + 3-way conference via CCP — same case timeline. If a provider has no public API,
  the manual click-to-dial + webhook status path is the real (non-mock) fallback.

## Telephony (Amazon Connect) build
- Contact flow: EN/FR by DNIS + selectable; safety short-circuit + PSAP warm-transfer; Lex intent
  capture; "agent from every node"; Lambda ANI->policy lookup; SMS GPS link (Pinpoint) + verbal;
  severity routing; HOLD with position/reassurance; call-drop -> outbound callback.
- Screen-pop contract (contact attributes via Streams onContactRefresh):
  { connect_contact_id, ani, dnis, language, cli_country, policy_number, authenticated,
    incident_type_ivr, safety_flag, location_link_status(+lat/lng), priority, queue,
    matched:{customer_id, policy_id, vehicle_id} }
- Recordings -> encrypted S3 + GDPR retention; consent announced. LIVE Connect only; the
  telephony-adapter boundary exists for portability/testing, but the delivered prototype uses a real
  claimed number end-to-end (no mock adapter).

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

## First working prototype (REAL vertical slice, HA from the start) — build this THIN PATH first
Prove the REAL end-to-end path before breadth. No mocks. It runs on the real HA cluster (dev
account) so node-failover is exercised from day one:
1. LIVE telephony: real Amazon Connect number + contact flow + Lex; real CCP softphone in the
   console. (The telephony-adapter boundary stays for portability, but the path is live.)
2. Screen-pop transport: Connect Streams -> BFF -> operator console over a WebSocket channel
   (event names: `call.ringing`, `call.answered` with the screen-pop payload, `dispatch.updated`).
3. Real data (deterministic seed of REAL records, not fakes): >=3 customers each with policy +
   vehicle covering the incident types (won't-start, accident, driver-ill); >=2 REAL providers
   configured in the connector registry with availability_calendar + performance_score; real
   operator, supervisor, and product_owner users in Keycloak.
4. Real tow connector: dispatch through a live provider sandbox/API (AXA Roadside Missioning or
   Towpal) returning real status/ETA; live ETA map uses Leaflet + OpenStreetMap (or a keyed provider
   if supplied); provider location/ETA come from the provider's real webhooks.
5. Real notifications: Amazon Pinpoint sends the tokenized status link + driver name/plate/photo.
6. Keycloak realm seed: realm `insucar`, OIDC clients for console + BFF, seeded users/roles so SSO
   works immediately.
7. Secrets: AWS Secrets Manager from the start (no local secret stub); short-lived creds only.
8. HA cluster + one-command bootstrap: `make dev-up` provisions the quorum HA cluster (3 members /
   2+witness across AZs), Patroni Postgres, clustered NATS/Redis, runs migrations + real seed.
   `make demo-reset` restores deterministic state for a repeatable demo.
9. Acceptance probe (DEFINES "working"): place a REAL call -> screen-pop lands -> dispatch to a REAL
   provider -> real ETA events flow -> real SMS link delivered -> auth/system/error in Loki. THEN
   chaos-kill one node mid-call and assert the case + call continue and Postgres auto-promotes with
   zero committed-data loss. Both must pass for the prototype to be "working".

## Prerequisites (supervisor's responsibility to confirm/provide BEFORE build)
- AWS Organizations with dev/UAT/prod accounts; bootstrap admin for first setup only; enough node
  quota for a QUORUM HA cluster (3 members / 2 data + witness) across >=3 AZs per tier.
- Amazon Connect instance + a CLAIMED LIVE phone number (required — no mock path).
- At least one REAL provider sandbox/API credential (AXA Roadside Missioning or Towpal) for live
  dispatch.
- Domain (unysolar.com) with DNS access + TLS (ACM wildcard acceptable).
- SMS: Amazon Pinpoint project (required — real SMS, no logged-mock fallback).
- Map tiles / what3words: real key (or use free OpenStreetMap tiles).
- Confirm: fresh least-privilege AWS credentials (never a previously exposed key); teardown scope
  (whole account vs tagged-only) if any teardown is requested; EN+FR demo languages.

## Deliverables (demo slice)
1. React operator console (screen-pop, case mgmt, coverage, dispatch, live ETA, EN+FR, WCAG,
   dedup/manual-create, SLA timers, driver-trust display).
2. LIVE Amazon Connect emergency IVR: Lex, agent-everywhere, safety + PSAP transfer, severity
   routing, call-drop callback.
3. Provider integration layer + admin connector registry + at least one REAL tow connector
   (live sandbox/API) + fallback chain.
4. Go backend services + Go BFF (generated TS types) + Postgres/PostGIS from `db/schema.sql` +
   `db/seed.sql` real-shaped seed (entity relationships enforced by FKs).
4b. Customer registration & self-service portal (SEPARATE app/host, customer Keycloak realm):
    signup + email/phone verification + consent capture + manage vehicles/policy + case status.
4c. Operator/staff console as a SEPARATE, non-discoverable app/host (staff Keycloak realm, MFA,
    zero-trust) — no shared login with the end-user app.
5. Rust inner-core vault (KMS envelope encryption + SHA-256-chained audit ledger, network-isolated).
6. Keycloak SSO (OIDC/OAuth/SAML/Kerberos/AD) with RBAC incl. product_owner.
7. Live ETA map + tokenized customer SMS status link via Amazon Pinpoint (real).
7b. Quorum HA cluster (3 members / 2+witness across >=3 AZs) with automatic node-failover proven.
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
1. Repo scaffold + CI + Terraform (dev account) + QUORUM HA cluster (3 members / 2+witness across
   >=3 AZs) + `make dev-up` one-command bootstrap on the real cluster. (Spinnaker stood up step 10.)
2. REAL VERTICAL SLICE (no mocks): live Connect number + contact flow + Lex; Streams->BFF->console
   WebSocket screen-pop; real-seeded Postgres (Patroni) + Keycloak realm; one real tow connector;
   Pinpoint SMS. Make the acceptance probe green (real call -> screen-pop -> real dispatch -> real
   ETA -> real SMS -> logs in Loki) AND the node-kill failover test pass. This is the first working
   prototype.
3. Postgres/PostGIS relationships hardened + console fleshed out.
4. Go core services + Go BFF (+ generated TS types).
5. Rust inner-core vault (KMS + SHA-256 chained ledger, NetworkPolicy-isolated).
6. Provider connector layer + fallback chain + admin key-in-vault (add more real connectors).
7. Live ETA map + tokenized SMS status link + driver-trust display.
8. Telephony hardening: PSAP warm-transfer + call-drop outbound callback + multi-region Connect DR.
9. IAM 3-case groups + JIT break-glass + prod-access blockchain ledger + product-owner approval.
10. Stand up SPINNAKER (Halyard/Operator) + register dev/UAT/prod accounts + pipelines with manual
    judgment gates; separated logging -> Loki + Grafana (logs + business KPIs); UAT account +
    promotion via Spinnaker manual gate; backup/restore + telephony-DR + node-failover runbooks.
11. Demo script + architecture one-pager; full hero-scenario rehearsal (incl. call-drop + node-kill).

## Acceptance criteria
- Hero scenario runs LIVE end-to-end on a real Connect number (NO mocks anywhere); screen-pop
  pre-fills; no re-keying; a dropped call auto-recovers via outbound callback.
- Dispatch fires through a REAL provider connector with fallback; real ETA updates; customer gets a
  tokenized Pinpoint SMS status link showing the tow driver's identity.
- NODE-FAILOVER PROVEN: killing any single node mid-call keeps traffic + case alive and Postgres
  auto-promotes < 30s with zero committed-data loss (no split-brain, quorum maintained).
- SSO via Keycloak works; RBAC enforced; the 3-case IAM model holds (dev cannot touch UAT/prod;
  UAT is read-biased pre-prod; prod is read-only until a product-owner-approved, time-bound grant).
- Every production access grant appears in the immutable blockchain ledger (who/when/why/approver)
  and is mirrored to auth.log.
- auth/system/error logs separated in Grafana; Rust vault network-isolated; business KPIs visible.
- All infra is IaC across 3 accounts; UAT->prod promotion works via a Spinnaker manual judgment
  stage; no secrets in code/logs; SBOM + signed images produced.
- Testing suite passes; the real-vertical-slice acceptance probe is green, the node-kill failover
  test passes, and `make demo-reset` restores a repeatable demo; backup/restore, telephony-DR, and
  node-failover runbooks demonstrated.

## Guardrails
- Read/inspect before modifying. No destructive action without approval + inventory (except
  ephemeral dev-namespace recreation).
- Never commit or log secrets. Rotate any exposed credential before use; never reuse an exposed key.
- Prefer minimal, portable, memory-safe designs. Keep the human operator in the loop.
