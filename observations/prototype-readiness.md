# Insucar — First-Prototype Readiness: what's missing to actually RUN it

Status of the repo today: design docs (`prompt/`), architecture diagram (`prompt/*.svg|pdf`),
and the database layer (`db/schema.sql`, `db/seed.sql`). Everything below is what still has to
exist or be provided before the real (no-mock) walking-skeleton can run end-to-end.

## A. Code/artifacts not yet created (all still to build)
- [ ] Go services (case, dispatch/PostGIS, provider-integration, telephony-adapter, coverage,
      notification) — no source yet.
- [ ] Go BFF + WebSocket screen-pop channel (`call.ringing`, `call.answered`, `dispatch.updated`).
- [ ] React operator console (screen-pop, case mgmt, dispatch, live ETA map, SLA timers, i18n EN+FR).
- [ ] Customer registration & self-service portal (signup, email/phone verification, consent,
      manage vehicles/policy, case status).
- [ ] Rust inner-core vault (KMS envelope encryption + SHA-256 audit ledger service).
- [ ] API contracts: OpenAPI/proto -> generated TypeScript client types.
- [ ] Migration runner wiring (goose/atlas) around `db/schema.sql` (schema exists; no runner yet).
- [ ] Dockerfiles (distroless) for every service; image build + cosign signing.
- [ ] Helm charts / K8s manifests (Deployments, Services, Ingress, NetworkPolicies, PDBs,
      topologySpread, Patroni/NATS/Redis stateful sets).
- [ ] Terraform for the dev account + quorum HA cluster (EKS/nodes across >=3 AZs).
- [ ] Keycloak realm exports: STAFF realm (operator/supervisor/admin/ops/product_owner) + CUSTOMER
      realm, OIDC clients for console + BFF, seeded demo users.
- [ ] CI pipeline (build/test/scan/SBOM/sign) + Spinnaker pipelines (stood up later, step 10).
- [ ] `make dev-up` / `make demo-reset` bootstrap + the smoke test + node-kill failover test.
- [ ] Provider webhook receiver (status/ETA normalizer -> case timeline).

## B. External prerequisites (supervisor must provide — BLOCKERS)
- [ ] FRESH least-privilege AWS key + account IDs for dev/UAT/prod. (The previously pasted key is
      compromised — must be revoked; do NOT use it.)
- [ ] Amazon Connect instance + a CLAIMED LIVE phone number + contact flow + Lex bot.
- [ ] Amazon Pinpoint project + SMS origination identity (sender ID / long code / short code).
- [ ] At least one REAL provider sandbox credential (AXA Roadside Missioning OAuth2, or Towpal API key).
- [ ] Domain `unysolar.com` DNS access + TLS (ACM wildcard).
- [ ] Map: OpenStreetMap tiles (free) or a keyed tile/routing provider.
- [ ] Node quota for a quorum HA cluster (3 members / 2 data + witness) per tier.

## C. Long-lead / risk items that can block a "live, no-mock" demo
- SMS sender registration is REGULATED in many countries (10DLC/short-code, sender-ID
  pre-registration) and can take days-weeks. Risk to "real SMS on day one." Mitigation: use a
  pre-approved test destination or the email channel for the first run; keep it real, not mocked.
- Amazon Connect number availability varies by country; claim early.
- Provider sandbox access often needs an approval/onboarding step; request early.
- Kerberos/Windows-AD interop cannot be fully proven without a domain controller; for the prototype
  validate OIDC/OAuth/SAML SSO and document the AD/LDAP federation config for a later env.
- Third-party pentest + GDPR/legal sign-off are external and scheduled, not code.

## D. Design decisions to confirm before build
- [ ] Quorum HA topology accepted: 3 members OR 2 data + witness across >=3 AZs (fixes the 2-node
      split-brain trap).
- [ ] Blockchain ledger implementation: pragmatic default is the SHA-256 hash-chained,
      append-only `audit_ledger` table in `db/schema.sql`; upgrade to Hyperledger Fabric only if
      an independent permissioned chain is required. Confirm which.
- [ ] Payment/PCI provider for excess / not-covered flow (needed for Tier-2 outcomes) — in scope
      for the first prototype or deferred?
- [ ] First-run cluster: real AWS dev account, or local k3d 3-node for the very first pass before
      cloud provisioning?

## E. Minimal path to a running first prototype (once B is provided)
1. Terraform dev account + HA cluster (or local k3d 3-node) up.
2. Apply `db/schema.sql` then `db/seed.sql` (Patroni Postgres).
3. Deploy Keycloak + import realms + seed users.
4. Deploy Go services + BFF + Rust vault + React console.
5. Wire Connect Streams -> BFF WebSocket -> console screen-pop.
6. Configure one real provider connector (sandbox) + webhook receiver.
7. Enable Pinpoint SMS (or email fallback if sender not yet approved).
8. Run the acceptance probe (real call -> pop -> dispatch -> ETA -> status link -> Loki logs)
   AND the node-kill failover test. Both green = first working prototype.

## What we ADDED in this pass
- End-user registration + self-service design section in the prompt.
- Full PostgreSQL/PostGIS schema `db/schema.sql` (registration, identity, policies, vehicles,
  cases, location/safety, providers/connectors, missions, notifications, consent, call recordings,
  immutable hash-chained audit ledger, prod-access grants).
- Deterministic real-shaped seed `db/seed.sql`.
