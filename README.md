# Insucar — Global Assistance Platform (prototype)

> A global, multi-line **assistance company** platform competing head-to-head with **Redion**
> (formerly Europ Assistance, Generali group). Human-coordinator-first ("phone fix first"),
> emergency-first, multi-tenant B2B white-label, built on a modern, fully AWS-managed, secure stack.
>
> This README is the entry point. For deeper context read, in order:
> `CONTINUE-HERE.md` (handoff), `build-notes.md` (decisions), `milestone.md` (history),
> `access.md` (all live URLs + credentials), `prompt/agenticpromptinsucar.md` (the master spec),
> `ROLLOUT.md` (step-by-step apply/deploy runbook).
>
> **LIVE (HTTPS / Let's Encrypt):** users at **https://unysolar.com/** (landing) and
> **/app** (login); operators at **https://op.unysolar.com/**. Logins in `access.md`.
> Runs on EKS (ns `insucar`, image `insucar-api:casecards`); TLS via ingress-nginx + cert-manager;
> host-based routing (op.* = operators, apex = users). CI/CD: Jenkins → ECR → Spinnaker (gated).

---

## 1. What we are doing (goals)

Build the software platform for an assistance provider that a customer calls (or apps) when they
are in trouble — a roadside breakdown, an accident, a medical issue, a travel or home emergency —
and get help fast. We are matching Redion's scope **one-to-one** and beating them on technology,
security, and transparency.

Product lines (parity target): **Mobility** (flagship), **Travel** (travel insurance + medical
assistance), **Home & living**, **Health**, **Senior care**, **Concierge** — plus a **B2B
white-label** model where partners embed/brand each line (multi-tenant).

Core principles:
- **Person-first / emergency-first.** A stranded, injured, or ill customer is the priority.
- **"Phone fix first"** — human coordinators try to resolve remotely before dispatching. We
  augment humans, we do not replace them.
- **Functional over flashy** — every demo step works end to end (no mocks in the delivered path,
  except telephony which is explicitly mocked for this phase — see Decisions).
- **Security, reliability, operational simplicity** by design — AWS-managed services first.

Flagship (Mobility) hero flow:
`call → multilingual IVR + spoken intake → safety triage → coordinator "phone fix" → if unresolved,
screen-pop (customer/policy/vehicle) → coverage decision → dispatch nearest provider → live ETA /
geo-tracking → SMS status link with driver identity → resolved`.

---

## 2. Repository map

```
prompt/          Master build spec (agenticpromptinsucar.md v3) + architecture diagram (mmd/svg/A2+A3 pdf)
db/              schema.sql (validated), seed.sql, schema-v3-additions.sql (tenants/RLS/parity)
prototype/       Runnable prototype: Go API (backend/), web pages, docker-compose, DEPLOYMENT.md
k8s/             EKS manifests: postgres.yaml, insucar-api.yaml, insucar-api-hpa.yaml, provider-axa.yaml
ci/              Jenkinsfile, jenkins-values.yaml (JCasC), plugins.txt, README.md, CICD-DEPLOYMENT.md
spinnaker/       spinnakerservice.yml, k8s-account-patch.yaml, pipelines/insucar-deploy.json
observations/    europcar-analysis, redion-analysis (competitor), gap-analysis, prototype-readiness
access.md        ALL live URLs + credentials (private repo, on purpose)
build-notes.md   Decisions taken this phase
milestone.md     What was done / failed / how resolved
CONTINUE-HERE.md Handoff notes (read first when resuming)
README.md        This file
```

---

## 3. Architecture

```
                         Consumer app        Operator console (hidden)     Partner portal (B2B)
                              │                        │                          │
                              └───────────── BFF (Go, REST/gRPC, WebSocket) ──────┘
                                                       │
  Telephony (MOCK Amazon Connect adapter) ──► screen-pop │
                                                       ▼
   Go core services:  case/incident · dispatch/matching(PostGIS) · coverage/policy ·
                      provider-integration · notification · tenant/partner · catalog
                                   │                    │                 │
                     Rust inner-core vault    EventBridge/SNS/SQS      Real provider connectors
                     (PII tokenization +       + ElastiCache (cache)    (HTTP; AXA/Towpal-style)
                      SHA-256 hash-chained ledger)     │                 │
                                   └──── Amazon RDS (PostgreSQL + PostGIS, Multi-AZ) ────┘   Real SMS (AWS SNS)
```

Stack (locked — AWS-managed first):
- **Backend core: Go** (memory-safe, high concurrency, small containers).
- **BFF: Go** (one API tailored to the frontends; generates TS types).
- **Inner-core vault: Rust** — the "crown jewels" zone: PII tokenization + KMS envelope encryption +
  append-only SHA-256 hash-chained audit ledger; network-isolated, mTLS-only, no public route.
- **Frontend: React** — consumer app, operator console, partner portal (i18n EN+FR first).
- **Data: Amazon RDS for PostgreSQL + PostGIS (Multi-AZ)** (Aurora option), **Amazon ElastiCache
  for Redis (Multi-AZ)**, **Amazon EventBridge + SNS/SQS** for async/events (Amazon MSK only if
  Kafka semantics are required).
- **Telephony: Amazon Connect** (MOCKED this phase via an adapter) + Lex; screen-pop via Streams.
- **Auth: Amazon Cognito** — customer / staff / partner user pools; OIDC/OAuth2/SAML federation
  (AD/LDAP via SAML IdP) (planned).
- **Platform: Kubernetes (EKS, AWS-managed control plane)**; **Terraform** IaC; **Jenkins** (CI) +
  **Spinnaker** (CD).
- **Observability: Amazon Managed Service for Prometheus + Amazon Managed Grafana + CloudWatch Logs
  + AWS X-Ray (OpenTelemetry)** (planned wiring).

---

## 4. Backend (how it works today)

The running prototype backend is a single Go service (`prototype/backend/main.go`) that will be
split into the service set above. It uses `pgx` against PostgreSQL/PostGIS and the AWS SDK for SNS.

Endpoints (all real DB-backed):
- `GET  /healthz` — liveness (checks DB).
- `POST /api/register` — end-user registration (+ consent rows + audit-ledger entry).
- `GET  /api/lookup?phone=E164` — ANI → customer/policy/vehicle (the operator screen-pop lookup).
- `POST /api/telephony/mock/incoming` — **MOCK Amazon Connect**: simulates an inbound call and
  returns Connect-shaped contact attributes + screen-pop payload.
- `POST /api/cases` — create a case/incident.
- `POST /api/dispatch` — pick provider, create mission + driver; **calls a REAL provider HTTP
  endpoint** when `PROVIDER_API_URL` is set (dispatch shows `provider_source:"api"`); sends a
  **REAL SMS via AWS SNS** with status link + driver identity.
- `GET  /api/case?id=` — case status + latest mission ETA.
- Serves the consumer page (`/`, `/login`, `/register`) and the **hidden** operator console at an
  obscure path (`/ops-console-7f3a9c`); unknown paths 404 (operator surface non-discoverable).

Data model (see `db/schema.sql` + `db/schema-v3-additions.sql`):
- Identity/registration: `customers`, `verification_tokens`, `consents`, `staff`, `partner_users`.
- Products: `policies` (+territories/entitlements), `coverage_products`, `vehicles`.
- Cases: `cases` (+links/locations/safety), `missions` (+status_events/driver).
- Providers: `providers`, `provider_connectors` (credentials by Secrets Manager ARN), availability.
- Comms/compliance: `notifications`, `call_recordings`, `interaction_log`, `consents`.
- Trust: `audit_ledger` (immutable, SHA-256 hash-chained, UPDATE/DELETE blocked),
  `prod_access_grants`.
- Multi-tenancy: `tenants`, `partners` + `tenant_id` on tenant-scoped tables + **Row-Level
  Security** (validated: each tenant sees only its rows; cross-tenant writes rejected).
- Extended enums for Redion parity: `service_line`, and services like `repair_on_spot`,
  `vehicle_repatriation`, `journey_continuation`, `car_pickup_delivery`, `tyre_protection`,
  `car_swap_ev`, `service_activated_rsa`, `micromobility`.

---

## 5. Security principles

(Demo shortcuts are called out; this is a prototype, not production.)
- **Least privilege + 3-tier separation.** Design: dev / UAT / prod in **separate AWS accounts**;
  developers dev-only, UAT read-biased pre-prod, prod read-only with JIT product-owner-approved
  access. (Prototype currently uses one account + namespaces; #13 immutable prod-access ledger is
  deferred.)
- **Surface isolation.** Consumer / operator / partner are separate apps, hosts, and Amazon Cognito
  user pools. **Operator console is non-discoverable** (obscure path, 404 to strangers), MFA + zero
  trust intended.
- **Multi-tenant isolation.** Row-Level Security by `tenant_id` (validated); RLS enforced for
  non-superuser app roles.
- **Crown-jewels vault.** PII/crypto isolated in a Rust inner-core service, mTLS-only, no public
  route — small, memory-safe attack surface; blast radius contained if an outer service is breached.
- **Tamper-evident audit.** `audit_ledger` is append-only and SHA-256 hash-chained; DB triggers
  reject UPDATE/DELETE.
- **Secrets.** Never in code/logs; provider creds referenced by AWS Secrets Manager ARN. (Demo:
  some creds inlined for speed — see `access.md` ROTATE checklist.)
- **Supply chain / network (planned/partial).** SBOM + cosign signing, SAST/SCA, WAF, DNSSEC,
  default-deny NetworkPolicies, mTLS between services, TLS at edge.
- **GDPR by design.** Consent capture incl. call recording; right-to-erasure; retention; EU
  residency (eu-west-1); `vital_interest` lawful basis for injury/medical.

---

## 6. Functions / features (current vs planned)

Working now (prototype):
- End-user registration + consent; ANI screen-pop lookup; case creation; provider dispatch with
  fallback provider selection; **real provider HTTP call**; **real SMS (SNS)**; **mock Connect**
  screen-pop; hidden operator console; immutable audit ledger; multi-tenant RLS schema.
- Running on **EKS** (2-node HA + HPA + Cluster Autoscaler), deployed via a **full gated Spinnaker
  pipeline** (dev → judgment → UAT → product-owner judgment → prod), triggered from **Jenkins**
  (Kaniko build → ECR → Spinnaker webhook).

Planned (see `prompt/agenticpromptinsucar.md` + `CONTINUE-HERE.md`):
- Split backend into the full Go service set + Rust vault service; OpenAPI/gRPC contracts.
- Amazon Cognito (3 user pools) + real auth on all surfaces; partner portal; consumer Digital-RSA app with
  live geo-tracking.
- Real Amazon Connect + Lex; PSAP warm-transfer; call-drop callback; accessibility/interpreter.
- Full service catalog (repatriation, tyre, EV swap, micromobility, travel/home/health/senior/concierge).
- Amazon RDS for PostgreSQL Multi-AZ (Aurora option; cross-region read replica for DR), multi-region telephony DR.
- Observability wiring (Prometheus/Loki/Grafana), tests (integration/e2e/load/chaos), TLS.

---

## 7. CI/CD & infrastructure

- **EKS cluster `insucar`** (eu-west-1, k8s 1.30), 2× t3.xlarge, nodegroup min2/max5.
- **Autoscaling:** HPA (insucar-api, CPU 60%, 2→6 pods) + **Cluster Autoscaler** (nodes 2→5).
- **Jenkins** (Helm + Config-as-Code) — job `insucar-ci`: checkout → **Kaniko** build+push to
  **ECR** → POST **Spinnaker webhook**. Unit tests run in the image build.
- **Spinnaker** (operator, v1.36.1) — application `insucar`, pipeline `deploy-insucar-api`
  (bake/deploy stages + manual judgment gates) deploying to `insucar-dev/uat/prod` via account
  `insucar-eks`. S3-backed Front50; igor↔Jenkins integration (`insucar-jenkins`).
- **Flow:** `git push → Jenkins (build+push) → Spinnaker webhook → gated deploy` (proven end-to-end).

Control plane note (EKS): etcd, kube-apiserver, scheduler, controller-manager are **AWS-managed**;
we operate only worker nodes + workloads.

---

## 8. How to run / interact

Local (prototype):
```
cd prototype && docker compose up -d --build     # Postgres+PostGIS + Go API
# open http://localhost:8080/  (consumer)  and  /ops-console-7f3a9c (operator)
```

Cluster access:
```
aws eks update-kubeconfig --name insucar --region eu-west-1
kubectl get nodes ; kubectl -n insucar get deploy,hpa,svc
```

GUIs & endpoints — see `access.md` (committed): Jenkins UI (+login), Spinnaker Deck, app PROD LB,
prototype EC2 app, Spinnaker webhook URL, and curl snippets to trigger builds/pipelines and approve
judgment gates.

DB apply (fresh):
```
psql ... -f db/schema.sql -f db/seed.sql -f db/schema-v3-additions.sql -f db/schema-v4-auth.sql -f db/schema-v5-cognito.sql
```

---

## 9. For an agent continuing this work

1. Read `CONTINUE-HERE.md` (tooling in `~/.local/bin`, gotchas & fixes, exa.ai key, live URLs).
2. Check state: `kubectl get nodes,pods -A`; Jenkins/Spinnaker GUIs from `access.md`.
3. Highest-value next steps (from gap analysis): Amazon Cognito + 3 user pools & real auth; split backend
   into tenant-aware Go services with OpenAPI; consumer Digital-RSA UI + partner portal;
   Amazon RDS Multi-AZ PostgreSQL; observability wiring; tests.
4. Deploy changes the proven way: push image to ECR → run/trigger the Spinnaker pipeline
   (`spinnaker/pipelines/insucar-deploy.json`), or `git push` to let Jenkins do it.
5. Keep decisions in `build-notes.md`, history in `milestone.md`, credentials/URLs in `access.md`.

Cost note: EKS + nodes + several ELBs bill ~$1/hr. Teardown steps in `ci/CICD-DEPLOYMENT.md`
and `prototype/DEPLOYMENT.md` (`eksctl delete cluster --name insucar --region eu-west-1`).
