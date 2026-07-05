# Insucar — Global Assistance Platform

> Roadside assistance platform competing with Redion (Europ Assistance/Generali). Human-coordinator-first, emergency-first, multi-tenant B2B white-label. Modern AWS-managed stack with immutable audit ledger.

**LIVE:** `https://unysolar.com/` (landing) · `https://unysolar.com/app` (user app) · `https://op.unysolar.com/` (operator console)  
**CI/CD:** Jenkins → Kaniko → ECR → Spinnaker gated pipeline (dev → UAT → prod)  
**Infra:** EKS eu-west-1, Let's Encrypt TLS, Cognito SSO, PostgreSQL+PostGIS, SNS SMS

## Quick Links

| Resource | URL |
|---|---|
| **End-user app** | https://unysolar.com/app |
| **Operator console** | https://op.unysolar.com/ |
| **GitHub Issues** | https://github.com/ugry/insucar/issues (29 open) |
| **Jenkins CI** | https://jenkins.unysolar.com |
| **Spinnaker Deck** | https://spinnaker.unysolar.com |

## Repository Map

```
├── prototype/backend/     Go API monolith (886 lines main.go)
│   ├── main.go            All handlers + routing + Cognito JWT
│   ├── cognito.go         RS256 JWKS verification + JIT provisioning
│   ├── cache.go           Redis screen-pop cache (disabled — Redis not deployed)
│   ├── events.go          EventBridge integration (disabled)
│   └── web/               HTML/JS frontends (SPA)
│       ├── enduser.html   Consumer app: login · register · incident · cases · GPS · Cognito PKCE
│       ├── operator.html  Dispatch console: queue · screen-pop · triage · dispatch · map · timeline
│       ├── landing.html   Marketing landing (golden-ratio design)
│       ├── status.html    Live tracking page for customer (Leaflet map + ETA countdown)
│       └── cognito-callback.html  OAuth2 PKCE code exchange handler
├── ci/                    Jenkinsfile + Helm values + plugins
├── spinnaker/             SpinnakerService CR + pipeline JSON (gated dev→UAT→prod)
├── k8s/                   Kubernetes manifests + HPA + observability
├── terraform/             13 .tf files — VPC, EKS, RDS, ElastiCache, Cognito, IAM (NOT APPLIED)
├── db/                    PostgreSQL schema + seed files (5 versions)
├── scripts/               cognito-setup.sh (idempotent pool provisioning)
├── observations/          Competitor analysis: Europcar, Redion, gap analysis
├── design/                UI mockups, logo SVGs
├── prompt/                Master build specification + architecture diagrams
├── mermaidschemas/        Current + planned infrastructure diagrams
├── prototype/uitest/      Headless browser QA suite (35 test cases)
├── AGENTS.md              OpenCode agent guide
├── opencode.json          OpenCode project config
├── improvements.md        Feature completeness matrix + P0-P3 improvement register
├── researhresult.md       End-user + operator UX improvement research
├── architectuxobservations.md  AI-implementable specs for all 25 improvements
├── expertobservations.md  Role-play empathy walkthrough (3 perspectives, scored)
├── expertexpert.md        Full platform audit + 4 emergencies + 22-item action plan
├── QAobservations.md      QA test report (26/35 passed, 1 bug fixed)
└── infra-todo.md          Managed services migration checklist (RDS, ElastiCache, IRSA)
```

## Architecture (Current Deployed State)

```
                    End-user app         Operator console (hidden, Cognito SSO)
                         │                        │
                   unysolar.com              op.unysolar.com
                         │                        │
                   Let's Encrypt TLS (ingress-nginx + cert-manager)
                         │                        │
                    ┌────┴────────────────────────┴────┐
                    │     Go API monolith (EKS pod)      │
                    │  /api/user/* · /api/agent/*        │
                    │  Cognito JWT RS256 verification    │
                    │  Mock Connect telephony            │
                    │  Real SMS (SNS) · real provider    │
                    │  Status tracking (/api/status/:t)  │
                    │  Immutable SHA-256 audit ledger    │
                    └────┬────────────┬──────────┬───────┘
                         │            │          │
              PostgreSQL+PostGIS   AWS SNS   provider-axa
              (in-cluster pod)     (SMS)     (HTTP connector)
```

**Stack:**
- **Backend:** Go 1.25 (single binary), pgx, golang-jwt, AWS SDK v2
- **Frontend:** Vanilla HTML/JS (SPA), Leaflet.js maps, Cognito PKCE OAuth2
- **Auth:** Amazon Cognito (3 pools: customer, staff, partner), RS256 JWT, group RBAC
- **Data:** PostgreSQL 15 + PostGIS (in-cluster pod — RDS migration pending)
- **Infra:** EKS 1.30 (2× t3.xlarge), ingress-nginx, cert-manager, Let's Encrypt
- **CI/CD:** Jenkins (Helm JCasC) → Kaniko → ECR → Spinnaker gated pipeline
- **IaC:** Terraform (146 resources defined, not applied — see infra-todo.md)

## API Endpoints (All Live)

### Public
| Method | Path | Description |
|---|---|---|
| GET | `/healthz` | Liveness (DB check) |
| GET | `/` | Marketing landing page |
| GET | `/app`, `/login`, `/register` | End-user app (login/register/incident/cases) |
| GET | `/app/callback` | Cognito OAuth2 PKCE callback handler |
| GET | `/api/auth/config` | Cognito pool config (domain, client IDs) |
| POST | `/api/register` | Customer registration (+ consent + audit ledger) |
| POST | `/api/telephony/mock/incoming` | Mock Amazon Connect inbound call |
| GET | `/api/status/:token` | Live tracking page + JSON API |
| GET | `/ws` | WebSocket endpoint (P0-6 — pending) |

### User (Requires Auth)
| Method | Path | Description |
|---|---|---|
| POST | `/api/user/login` | Email/password login (demo) |
| POST | `/api/user/incident` | Submit help request (creates case) |
| GET | `/api/user/cases` | List own cases |
| POST | `/api/logout` | Clear session |
| GET | `/api/me` | Current identity |

### Agent/Operator (Requires Auth)
| Method | Path | Description |
|---|---|---|
| POST | `/api/agent/login` | Agent ID/password login (demo) |
| GET | `/api/agent/cases` | Case queue (all active) |
| GET | `/api/agent/case?id=` | Case detail + customer + vehicle + policy + mission + driver + safety |
| GET | `/api/agent/lookup?phone=` | ANI screen-pop lookup |
| POST | `/api/agent/dispatch` | Dispatch provider → create mission → send SMS |
| POST | `/api/agent/stats` | Queue statistics |
| GET | `/api/agent/providers` | Ranked provider list |
| PATCH | `/api/agent/case/:id/priority` | Update case priority (P0-4 pending) |
| POST | `/api/agent/case/:id/resolve` | Resolve case (P0-4 pending) |
| POST | `/api/agent/notify` | Send SMS to customer (P1-4 pending) |
| PUT | `/api/agent/case/:id/safety` | Save safety triage (P0-X6 pending) |

## Features — What Works Today

### End-User App (unysolar.com/app)
- Email/password login + Cognito Single Sign-On (PKCE OAuth2)
- Customer registration (first, last, email, phone, country, password, consents)
- 6 incident types: breakdown, accident, flat tyre, EV no charge, medical, other
- GPS auto-detect ("📍 Use my location") — stores lat/lng in sessionStorage
- Case listing with responsive card layout + status pills
- Incident submission creates DB case + audit ledger entry
- Immutable SHA-256 hash-chained audit ledger

### Operator Console (op.unysolar.com)
- Hidden path security (404 for wrong paths)
- Agent ID login + Cognito SSO (staff pool, operator/supervisor/PO roles)
- Live case queue with color-coded priority pills (emergency/high/normal)
- Queue auto-refresh every 8 seconds with SLA timers
- Screen-pop with auto-bind on incoming call (customer, policy, vehicle)
- Coverage decision panel (coverage level, excess, callout limit, service checks)
- Safety triage panel (5 yes/no questions)
- Provider ranking with scores, SLA %, availability windows
- One-click dispatch with real SMS (driver name, plate, ETA, tracking link)
- Driver trust card after dispatch
- Case timeline from mission status events
- Interactive Leaflet.js map with incident + provider markers
- SLA age bar with breach detection (15min warn, 30min alert)
- Provider fallback chain (retries providers in ranked order)
- 112/PSAP emergency warm-transfer button
- Service selector (tow, repair, jump start, tyre, lockout, fuel)
- Macros layout exists (not wired — P1-4)

### Infrastructure
- EKS 1.30 (2 nodes, auto-scaling 2→5, HPA 2→6 pods)
- HTTPS via Let's Encrypt (ingress-nginx + cert-manager)
- Host-based routing (op.* → operator, apex → user)
- Jenkins → Kaniko → ECR → Spinnaker webhook → gated deploy
- Spinnaker pipeline: Deploy DEV → judgment → UAT → PO judgment → PROD
- Daily pg_dump CronJob to S3 (backup band-aid until RDS migration)
- AWS IAM user `insucar-admin` (root keys pending deletion in AWS Console)

### Auth
- Amazon Cognito: 3 user pools (customer, staff, partner) with Hosted UI domains
- Staff pool RBAC groups: operator, supervisor, admin, ops, product_owner
- RS256 JWT verification against Cognito JWKS (hourly refresh)
- Group-based role mapping (staff groups → "agent", otherwise → "user")
- JIT customer provisioning on first Cognito login
- Demo cookie auth as fallback when `COGNITO_ISSUER` is unset

## Known Gaps & Priority Items

See GitHub Issues: https://github.com/ugry/insucar/issues (29 open)

| # | Area | Gap | Severity |
|---|---|---|---|
| 1 | Security | Root AWS keys not deleted from Console | EMERGENCY |
| 2 | Data | PostgreSQL in-cluster pod — no RDS, no failover | EMERGENCY |
| 3 | Security | GitHub PAT + Jenkins password not rotated | EMERGENCY |
| 4 | CI/CD | Spinnaker pipeline fixed (uses `${parameters.imageTag}`) | FIXED |
| 5 | UX | Post-submission tracking view missing (motorist sees silent "triaging" card) | P0-X1 |
| 6 | UX | No guest help path — login wall before help | P0-X4 |
| 7 | Infra | Terraform (146 resources) not applied — no RDS, ElastiCache, IRSA | P0 |
| 8 | UX | Safety triage buttons don't save to DB or auto-escalate priority | P0-X6 |
| 9 | Infra | Redis not deployed — screen-pop cache disabled | P1 |
| 10 | Backend | No input validation, no rate limiting, no API versioning | P1 |

## How to Run

### Local
```bash
cd prototype && docker compose up -d --build
# http://localhost:8080/ (consumer) · /ops-console-7f3a9c (operator)
```

### EKS Access
```bash
aws eks update-kubeconfig --name insucar --region eu-west-1
kubectl -n insucar get deploy,svc,pods
```

### Deploy (CI/CD)
```bash
git push origin main
# → Jenkins insucar-ci builds, pushes to ECR, triggers Spinnaker webhook
# → Spinnaker pipeline: Deploy DEV → manual judgment → UAT → PO judgment → PROD
```

### DB Reset
```bash
psql -h postgres.insucar.svc.cluster.local -U postgres -d insucar \
  -f db/schema.sql -f db/seed.sql -f db/schema-v3-additions.sql \
  -f db/schema-v4-auth.sql -f db/schema-v5-cognito.sql -f db/seed-users.sql
```

### Cognito Setup
```bash
bash scripts/cognito-setup.sh          # Create 3 pools + domains + clients + users
bash scripts/cognito-setup.sh --destroy # Tear down
```

## Documentation Index

| Document | Purpose |
|---|---|
| `prompt/agenticpromptinsucar.md` | Master build specification (v3, Redion-parity) |
| `CONTINUE-HERE.md` | Handoff notes + current state + tooling |
| `expertexpert.md` | Full platform audit — 4 emergencies + 22 action items |
| `improvements.md` | Competitive comparison + feature matrix + P0-P3 register |
| `researhresult.md` | End-user + operator UX improvement research |
| `architectuxobservations.md` | AI-implementable specs (exact code, pixels, routes) |
| `expertobservations.md` | Role-play walkthrough (3 perspectives, scored) |
| `QAobservations.md` | QA test report (35 tests, 26 passed, 1 bug fixed) |
| `infra-todo.md` | RDS/ElastiCache/IRSA migration checklist |
| `ROLLOUT.md` | Step-by-step Terraform apply + deploy runbook |
| `SECURITY-ROTATION.md` | Credential rotation hardening |
| `build-notes.md` | Architectural decisions |
| `milestone.md` | Full project history + failures + resolutions |
| `mermaidschemas/` | Current-state + planned architecture diagrams |

---

*Platform assessed 2026-07-05. Overall readiness: 5.5/10. "It works" ≠ "It helps."*
