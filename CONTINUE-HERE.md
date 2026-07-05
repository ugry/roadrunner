# Insucar — Continuation / Handoff Notes

> **Single source of truth for live state:** See `README.md` for current endpoints, features, and status.
> Live image tag, deployed features, and known gaps are maintained in README.md only.

Purpose: everything an agent needs to resume this project without re-discovery.
Repo: https://github.com/ugry/insucar (private) · AWS: 326804802908, eu-west-1 · Domain: unysolar.com
Last updated: 2026-07-05

## TL;DR current state
- LIVE over HTTPS: https://unysolar.com/ (landing) · /app (user) · https://op.unysolar.com/ (operator)
- Cognito SSO deployed with PKCE OAuth2 (3 pools: customer/staff/partner)
- CI/CD proven: git push → Jenkins → Kaniko → ECR → Spinnaker gated pipeline
- For current image tag, features list, and API endpoints → see README.md
- IAM user `insucar-admin` active — root keys must be deleted from AWS Console
- Backend: Go service prototype/backend/ (main.go + connector.go + tenant.go + cognito.go + events.go + cache.go).
   Endpoints: /api/user/*, /api/agent/* (added /providers, /stats, /status), /api/webhook/provider,
   /api/telephony/mock/{incoming,psap,call-state}, /api/register. Host-based routing.
- Mock Amazon Connect; REAL provider connector (provider-axa svc); REAL SMS via SNS.
- TLS: ingress-nginx + cert-manager (ClusterIssuer letsencrypt-prod, cert insucar-tls). 80->443 redirect.
  insucar-api svc = ClusterIP; Route53 alias (unysolar/www/app/op) -> ingress ELB.
- CI/CD on EKS PROVEN: git push -> Jenkins(Kaniko->ECR) -> Spinnaker webhook -> gated dev/UAT/prod.
- Design + prompt: prompt/agenticpromptinsucar.md (v3 Redion-parity). Diagrams: prompt/*.svg|pdf,
  mermaidschemas/{current-deployed,planned-design}.svg. Terraform IaC: terraform/.
- DB: db/schema.sql + seed.sql + schema-v3-additions(tenants/RLS) + schema-v4-auth + schema-v5-cognito + schema-v6-operator + seed-users + seed-tenant.
- Brand: green #0a7d5a / navy #0b1f2a / amber #f5a623 / Inter. Logo: design/insucar-logo.svg + mark.
## Local tooling (installed to ~/.local/bin)
- aws (v2), kubectl, eksctl, go 1.24.4, node v22, crane (OCI images), gh, git, jq

## Known gotchas / fixes (do not rediscover)
- Local sandbox DNS resolver (10.72.106.36) intermittently SERVFAILs. If kubectl/aws "cannot resolve",
  it's the sandbox not AWS; 8.8.8.8 works; wait it out or retry. /etc/resolv.conf is root-owned (can't edit).
- EKS 1.30 has NO default StorageClass / EBS CSI by default. Already fixed: aws-ebs-csi-driver addon +
  IRSA role AmazonEKS_EBS_CSI_DriverRole + gp2 set default.
- Jenkins helm chart: use controller.admin.username/password (not adminUser); don't duplicate default
  plugins (configuration-as-code, git, workflow-aggregator) in additionalPlugins.
- Private repo can't be git-cloned by EC2 user-data -> ship code via presigned S3 URL.
- Spinnaker clouddriver crashes binding kubernetes account 'raw-resources-endpoint-config'
  (spinnaker/spinnaker#6840). FIX: add rawResourcesEndpointConfig.{kindExpressions:[],omitKindExpressions:[]}
  to the account. In-cluster deploy uses a cluster-admin SA kubeconfig injected via operator files.

## Live endpoints
- Jenkins: https://jenkins.unysolar.com
- Spinnaker Deck: https://spinnaker.unysolar.com
- Spinnaker Gate: https://gate.unysolar.com
- App endpoints + current image → see README.md

## Known Limitations
- RLS: SET LOCAL doesn't carry across pgxpool connections (needs per-request connection pinning)
- PostgreSQL: in-cluster pod, NOT RDS Multi-AZ (see infra-todo.md for migration plan)
- Redis: not deployed — screen-pop cache, session store disabled
- Telephony: mock Connect only — real Amazon Connect + Lex needs provisioning

## Agent Team Lessons Learned (do NOT repeat)
### 2026-07-05 — Missing JSON struct tags (Issue #79)
- **What:** API handler structs in main.go use Go field names without `json:"..."` tags. Go decoder matches `first`→`First` but NOT `firstName`→`First`. Standard REST clients sending camelCase get silent validation failures.
- **Why Tester missed:** Tester used Go source code as field-name reference instead of an API contract. No OpenAPI spec exists. No field-name mutation/fuzzing tests.
- **Prevention for Developer:** EVERY API struct MUST have explicit `json:"fieldname"` tags. Add `go vet` check or lint rule for missing tags.
- **Prevention for Tester:** Always test API with BOTH Go-native and standard camelCase field names. Create an OpenAPI spec as source of truth BEFORE writing tests.
- **Prevention for Orchestrator:** Require an OpenAPI spec before marking any endpoint as "done." Add field-name fuzzing to the mandatory test checklist.

### 2026-07-05 — Hardcoded/Crippled forgot-password link (Issue #80)
- **What:** `enduser.html:75` has a hardcoded Cognito URL that (a) points to DEV pool on ALL environments, (b) misses the REQUIRED `response_type=code` param causing Cognito error. Also no backend `/forgot-password` route exists (404). The app already fetches dynamic `cognitoCfg` via `/api/auth/config` but the forgot-password link doesn't use it.
- **Why Tester missed:** The tester tested functional flows (login/register/logout) but didn't click every navigation link. No automated broken-link scanner. No per-environment link validation.
- **Prevention for Developer:** NEVER hardcode Cognito URLs in HTML. All Cognito links must be JS-generated from the dynamic `cognitoCfg` object. Add CI lint rule: `grep -r 'amazoncognito.com' web/` must return zero results.
- **Prevention for Tester:** Mandatory: click every `<a href>` on every page, on every environment (DEV/UAT/PROD). Use automated link checker.

## Tester Quality Overhaul — 2026-07-05
> 8 issues (#73-#80) revealed systemic testing gaps. Below is the root cause analysis and the new mandatory testing protocol.

### Tester Miss Pattern Analysis

| Issue | Category | What Was Missed | Root Gap |
|-------|----------|----------------|----------|
| #73 | Auth flow | Operator login returns no cookie | **Single-role testing** — tested user, not operator |
| #74 | Data integrity | Endpoints return 200 but empty body | **No data validation** — checked status codes, not payloads |
| #75 | Security | No CORS headers | **No security scan** — manual curl only |
| #76 | Security | Stored XSS in registration | **No input fuzzing** — didn't inject `<script>` or HTML |
| #77 | Security | Raw SQL errors exposed | **No error path testing** — didn't trigger errors intentionally |
| #78 | Security | No CSRF tokens | **No security checklist** — didn't have a standard OWASP list |
| #79 | API contract | Missing JSON struct tags | **No API spec** — used Go source as field-name reference |
| #80 | UX/links | Forgot password link broken | **No link checker** — didn't click every link on every page |

### New Mandatory Testing Protocol (for Tester agent)

Before reporting "test pass" on any deployment, the Tester MUST complete ALL 5 gates:

#### GATE 1: Multi-Role Authentication
- Test login as user, operator, supervisor, PO — ALL must work
- Verify each role's session cookie has Secure/HttpOnly/SameSite
- Test logout and re-login for each role

#### GATE 2: Every Link, Every Page, Every Environment
- Run automated link scanner against DEV, UAT, and PROD
- No 4xx/5xx responses on any `<a href>`, `<form action>`, or JS-driven redirect
- Reject: any `amazoncognito.com` URL hardcoded in HTML (must be JS-generated)

#### GATE 3: API Contract Validation
- Test EVERY endpoint with BOTH camelCase and Go-native field names
- Verify actual response body content (not just HTTP status)
- Test with missing required fields, extra fields, wrong types
- Test with unauthenticated requests → must return 401 (not 200 empty)

#### GATE 4: Security Baseline (OWASP Top 10 quick scan)
- Inject `<script>alert(1)</script>` in every text input → must be sanitized
- Add `' OR 1=1 --` to query params → must return 401/400 (not raw SQL error)
- Submit cross-origin POST without CSRF token → must be rejected
- Check response headers for CORS, HSTS, X-Content-Type-Options
- Verify error responses never expose DB details (SQLSTATE, constraint names)

#### GATE 5: Link & UX Smoke Test
- Click every `<a>`, `<button>`, and `<form>` on every page
- Verify no 404s, no Cognito parameter errors, no JS console errors
- Test on all 3 environments (DEV → UAT → PROD)

### Automated Testing Required (for Orchestrator to deploy to CI)
- Add `linkcheck.sh` to `ci/` — curls every route from routes.txt, fails on 4xx/5xx
- Add `security-baseline.sh` to `ci/` — OWASP quick scan before deploy
- Add `field-fuzz.sh` to `ci/` — tests endpoints with camelCase/snake_case mutations

## Priority Actions (see expertexpert.md for full list)
1. Delete root AWS keys from Console (IAM user `insucar-admin` created)
2. Apply Terraform for RDS + ElastiCache (see infra-todo.md)
3. Rotate GitHub PAT + Jenkins password
4. Implement P0-X1 through P0-X7 (see GitHub Issues #26-#32)

## Key files map
- README.md — single source of truth for live state, features, endpoints
- expertexpert.md — full platform audit + emergency action plan
- improvements.md — competitive comparison + improvement register
- infra-todo.md — RDS migration checklist
- prompt/agenticpromptinsucar.md — master build spec
- archive/build-notes.md — historical decisions
- archive/milestone.md — historical timeline
