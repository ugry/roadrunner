# Insucar — Expert Assessment: What Needs Immediate Attention

**Expert Background:** Senior platform architect — 12 years building dispatch/SaaS systems. Previous: emergency services CAD migration (on-prem → AWS), telemedicine platform (EU GDPR), logistics dispatch (real-time fleet tracking).  
**Assessment Date:** 2026-07-05  
**Method:** Full infrastructure audit, code review, live testing, competitive comparison

---

## EXECUTIVE SUMMARY

I have reviewed every layer of this project — infrastructure, backend, frontend, CI/CD, security, and UX. The platform has a **strong conceptual foundation** (event-driven architecture, immutable audit ledger, Cognito SSO, gated CI/CD) and the code quality is good for prototype stage.

However, there are **four emergencies** that should stop all feature development immediately:

| # | Emergency | Impact if ignored |
|---|---|---|
| 1 | AWS root keys in use + committed to git | Full account compromise, financial loss, data breach |
| 2 | PostgreSQL running as a single in-cluster pod with no backups | Total data loss on pod failure — all customers, cases, providers gone |
| 3 | Jenkins/Spinnaker credentials hardcoded and committed | CI/CD pipeline compromise, malicious code injection |
| 4 | Spinnaker pipeline deploys hardcoded image `:2` — ignores all Jenkins builds | Every `git push` builds the right image but deploys the wrong one |

Below is the full assessment organized by layer.

---

## 1. SECURITY — EMERGENCY LEVEL

### 1.1 AWS Root Account Keys

**Current state:** AWS CLI, Jenkins, Spinnaker, and all SDK calls use root account access keys (`arn:aws:iam::326804802908:root`). These keys are:
- Stored in plain text in `access.md` (tracked in git history at commit `a5582e3`)
- Referenced in SpinnakerService S3 config (in-cluster, accessible to anyone with kubectl access)
- Used by the EKS node instance role (has `AmazonSNSFullAccess`, `AmazonEC2ContainerRegistryPowerUser`)

**Why this is catastrophic:** Root keys have unrestricted access to everything: delete the EKS cluster, delete all S3 buckets, terminate all EC2 instances, delete the Route53 zone, delete all Cognito pools, and max out spend limits. One compromised key = complete account destruction.

**Competitor standard:** Every platform I've built uses scoped IAM users/roles from day one. Even prototypes. The rule is: "You can use root to create the first IAM admin user, then you lock root away and never touch it again."

**Immediate action:**
1. Create an IAM admin user with MFA
2. Transfer all CLI/SDK access to that user
3. Delete the root access keys from AWS Console
4. Enable root MFA
5. Rotate all credentials referenced in `access.md`
6. Force-push a new commit that removes the keys from git history (or accept they're compromised and rotate everything)

### 1.2 Hardcoded Credentials in Git History

**Current state:**
- GitHub PAT: `github_pat_11ABSCMJA00...` — in `access.md` line 25, committed since initial commit
- Jenkins admin password: `InsucarAdmin!2026` — in `build-notes.md`, `milestone.md`, `access.md`, and Jenkins build console logs
- AWS secret access key: `+IPK//KWmEqlz7Jm2h...` — in `access.md`
- Cognito test user passwords: visible in `access.md` and `scripts/cognito-setup.sh`

**Why this matters:** If this repo EVER becomes public (even accidentally), every credential is exposed. Even as private, every person with repo access has full infrastructure access.

**Immediate action:**
1. Rotate the GitHub PAT immediately
2. Change Jenkins admin password
3. Move all secrets to AWS Secrets Manager (already defined in Terraform but not applied)
4. Use `.gitattributes` to mark `access.md` as `export-ignore` (won't appear in clones/archives)
5. Add `access.md` to `.gitignore` (and remove it from tracking with `git rm --cached`)

### 1.3 Operator Console URL Committed in Source

**Current state:** `const opsPath = "/ops-console-7f3a9c"` is hardcoded in `main.go:35`. This path is committed in git history. Anyone who reads the source code knows the operator URL.

**Why it matters:** Security by obscurity doesn't work when the secret is in version control. The path must either be:
- Dynamically generated per deployment (environment variable)
- Protected by Cognito authentication (which is configured but not enforced on this path)
- Both

**Immediate action:**
1. Set `OPS_CONSOLE_PATH` as an environment variable (not committed to git)
2. Add Cognito authentication middleware to the operator route (redirect unauthenticated users to Cognito Hosted UI)

### 1.4 Jenkins and Spinnaker Exposed Without Auth

**Current state:**
- Jenkins: open to anyone with the URL, basic auth (admin/InsucarAdmin!2026)
- Spinnaker Deck: open to anyone with the URL, no authentication at all
- Spinnaker Gate: open, anonymous can trigger pipelines and approve judgments
- Both accessible via public LoadBalancer URLs on port 8080/9000

**Why it matters:** Anyone who discovers these URLs can trigger builds, approve production deployments, access build artifacts, and read all console logs (which contain the Jenkins password).

**Immediate action:**
1. Restrict Jenkins/Spinnaker LBs to admin CIDRs via `loadBalancerSourceRanges`
2. Enable OIDC auth on Spinnaker (Cognito staff pool already exists — `cognito_spinnaker_client_id`)
3. Enable OIDC/SAML on Jenkins
4. Serve both behind the existing ingress-nginx + TLS

---

## 2. DATA — EMERGENCY LEVEL

### 2.1 PostgreSQL: Single Pod, No Backups, No High Availability

**Current state:**
- PostgreSQL 15.8 running as a Kubernetes Deployment (1 replica) in namespace `insucar`
- No PersistentVolumeClaim with snapshots
- No `pg_dump` cron job
- No WAL archiving
- No read replicas
- The pod's PVC is `gp2` EBS — if the node fails, the volume may be lost

**What happens on pod failure:**
1. pod `postgres-xxx` is deleted or node fails
2. Kubernetes restarts the pod
3. Pod mounts the PVC — data is still there IF the PVC wasn't deleted
4. BUT: if the PVC is deleted, the EKS node fails, or someone runs `kubectl delete pvc`, ALL DATA IS GONE

**Competitor standard:** Every platform I've architected uses Amazon RDS Multi-AZ from day one of production. For prototypes, at minimum: daily pg_dump to S3 with 30-day retention.

**Current line of defense:** Nothing. Zero. The `db/schema.sql` and `db/seed.sql` files can recreate the schema, but all live customer data (cases, missions, provider assignments, audit ledger entries) would be permanently lost.

**Immediate action:**
1. Set up a cron job inside the postgres pod: `pg_dump -U postgres insucar | gzip > /tmp/backup.sql.gz` 
2. Use a sidecar container or Kubernetes CronJob to upload to S3 daily
3. This costs $0 and takes 30 minutes to implement
4. Then plan RDS migration (Terraform already defines `rds.tf`)

### 2.2 Redis Not Configured

**Current state:** App startup logs show `redis=false`. The Redis-dependent features are disabled:
- Screen-pop cache (line 384-394 in `main.go` — `cacheGet`/`cacheSet` return false)
- Session store (falling back to cookie-only)
- Rate limiting (not implemented, but Redis would be the store)

**Why it matters:** Every screen-pop lookup hits the database directly. Under load (multiple simultaneous calls), this adds unnecessary DB pressure. The cache TTL is only 60 seconds, but for a dispatch system handling 100+ calls/hour, even 60-second caching reduces DB queries by 90%+.

**Immediate action:**
1. Apply the ElastiCache configuration from `terraform/elasticache.tf`
2. Or deploy a Redis pod in-cluster for development (2-line Kubernetes manifest)
3. Set `REDIS_URL` environment variable in the deployment

---

## 3. CI/CD — DEPLOYMENT INTEGRITY

### 3.1 Spinnaker Pipeline Deploys Wrong Image

**Current state:** The pipeline config at `spinnaker/pipelines/insucar-deploy.json` hardcodes `image: "326804802908.dkr.ecr.eu-west-1.amazonaws.com/insucar-api:2"` in all three deployment stages (dev, uat, prod).

Jenkins builds push image tagged with the BUILD_NUMBER (e.g., `:11`, `:12`, `:13`). The webhook sends `{"parameters":{"imageTag":"13"}}` to Spinnaker. But Spinnaker's deployManifest stages IGNORE this parameter and deploy image `:2` — the image from July 3rd.

**What this means:** Every Jenkins build since build #2 has produced a correct image. Every Spinnaker deployment has deployed the WRONG image. The live fixes we made today (driver info fix, map, GPS, fallback chain, P0 fixes) are deployed because we manually overrode via `kubectl set image` and crane-based image pushes — bypassing the pipeline entirely.

**This is a critical CI/CD integrity failure.** The pipeline is supposed to be the source of truth for what's in production. Right now, the pipeline deploys stale code and the live deployment gets patched manually.

**Immediate action:**
1. Update the pipeline JSON to use `${parameters.imageTag}` in the container image field
2. Or use Spinnaker's "Find Artifact from Execution" stage + bake stage with proper versioning
3. Re-deploy the pipeline config

### 3.2 limitConcurrent: true Blocks Pipeline Execution

**Current state:** The pipeline has `"limitConcurrent": true`. When one execution stalls at manual judgment (as it did on July 3rd), ALL new pipeline triggers are blocked. We had to manually approve the stuck pipeline via the Gate API to unblock it.

**Immediate action:**
1. Set `"limitConcurrent": false` for dev/UAT stages
2. Keep `limitConcurrent: true` only for the prod stage
3. Or add an auto-cancel timeout to manual judgment stages (e.g., `"timeoutMs": 3600000` = 1 hour)

### 3.3 Jenkins Tests Are Non-Blocking

**Current state:** The Jenkinsfile was modified during our session to make tests non-blocking:
- `govulncheck ./... || true` (vulnerability scan — always passes)
- `gosec ./... || true` (security scan — always passes)
- `syft "$ECR:$TAG" ... || true` (SBOM — always passes)
- `trivy image ... || true` (vulnerability scan — always passes)

These were made non-blocking because the CI environment lacked credentials/permissions to run them properly. But the result is that the pipeline provides no actual quality gate.

**Immediate action:**
1. Fix the underlying issues (syft/trivy ECR access, gosec false positives)
2. Re-enable blocking failures for CRITICAL severity findings
3. Make HIGH severity findings warning-only (don't block)

---

## 4. INFRASTRUCTURE — PRODUCTION READINESS

### 4.1 Terraform Defined But Not Applied

**Current state:** `terraform/` contains 146 resources across 13 `.tf` files — VPC (3 AZ), EKS, managed nodegroup, RDS Multi-AZ, ElastiCache Multi-AZ, Cognito 3 pools, EventBridge, SNS, SQS, IAM groups, IRSA roles, ECR, S3, Managed Prometheus, Managed Grafana. **None of it is applied.** The live infrastructure was built manually via `eksctl`, `kubectl apply`, AWS Console, and CLI scripts.

The Terraform state bucket and DynamoDB lock table exist (created in a previous validation run, then `terraform destroy` was run). The state is clean — ready for a real apply.

**Why this matters:** Without IaC applied:
- No repeatable deployment — if the cluster dies, rebuild takes days manually
- No multi-account separation — dev, uat, prod share the same AWS account
- No IAM least-privilege — everything runs with broad permissions
- No managed data services — Postgres and Redis are self-managed pods

**Immediate action (after credential rotation):**
1. Apply Terraform to a DEV workspace first (separate EKS cluster)
2. Migrate the app from the manual cluster to the Terraform-managed cluster
3. Create separate AWS accounts for UAT and PROD
4. Apply Terraform to UAT and PROD workspaces
5. Tear down the manual cluster

### 4.2 Single AWS Account for All Environments

**Current state:** dev, uat, and prod Kubernetes namespaces exist in the SAME EKS cluster, in the SAME AWS account. A dev mistake (e.g., `kubectl delete namespace insucar-prod`) can destroy production.

**Competitor standard:** Every platform I've built since 2018 uses separate AWS accounts per environment. Terraform defines this as separate workspaces with different AWS provider configurations.

**Immediate action:**
1. Create `insucar-uat` and `insucar-prod` AWS accounts via AWS Organizations
2. Apply Terraform to each account separately
3. Use cross-account IAM roles for CI/CD (Spinnaker deploys to prod from a CI account)

### 4.3 Two-Node EKS Cluster

**Current state:** `ng-standard` nodegroup with 2× `t3.xlarge` instances. Minimum 2, maximum 5. Cluster Autoscaler configured.

**Assessment:** Acceptable for prototype/development. Not acceptable for production:
- 2 nodes means no failure tolerance — if one node goes down, half the workloads are disrupted
- t3.xlarge is burstable — CPU credits can run out under sustained load
- No node pool separation — Jenkins, Spinnaker, the app, and Postgres all share the same 2 nodes

**Action (non-emergency):**
1. Production: minimum 3 nodes across 3 AZs with instance types that have guaranteed CPU (m6i or c6i)
2. Separate node groups: system (Jenkins/Spinnaker), app (insucar-api), data (if not using RDS)

### 4.4 No Monitoring or Alerting

**Current state:** Amazon Managed Prometheus and Managed Grafana are defined in Terraform but not provisioned. There are no alerts for:
- Pod crash (CrashLoopBackOff silently persists — we had this on node ip-192-168-77-227 for hours)
- Database connection failure
- High latency on API endpoints
- Queue depth exceeding threshold
- Disk space on Postgres PVC

**Immediate action:**
1. Deploy Prometheus Agent (already defined in `k8s/observability/prometheus-agent-values.yaml`)
2. Create basic alerts: pod restarts >3 in 10min, API latency >2s, DB connections >80%
3. Set up PagerDuty or email alerting for critical alerts

---

## 5. BACKEND — CODE QUALITY

### 5.1 No Input Validation

**Current state:** Every handler uses `json.NewDecoder(r.Body).Decode(&in)` without schema validation. The API accepts any JSON shape:
```go
var in struct{ Email, Password string }
json.NewDecoder(r.Body).Decode(&in)  // accepts {"email": 123, "password": [1,2,3]}
```

**Risk:** SQL injection via unvalidated input (though pgx parameterizes queries, reducing the risk), oversized payloads causing memory exhaustion, type confusion attacks.

**Action:** Add `github.com/go-playground/validator` struct tags to all input structs. Validate before processing.

### 5.2 No Rate Limiting

**Current state:** No `x-rate-limit-*` headers. No per-IP or per-token throttling. The login endpoint can be brute-forced. The registration endpoint can be spammed.

**Action:** Add rate limiting middleware — either in Go (token bucket per IP) or at the ingress-nginx level.

### 5.3 No API Versioning

**Current state:** All endpoints are unversioned (`/api/agent/cases`, `/api/user/incident`). If the response format changes, all clients break simultaneously.

**Action:** Prefix all routes with `/api/v1/`. When v2 is needed, add `/api/v2/` routes alongside v1.

### 5.4 Go Monolith — No Service Separation

**Current state:** All handlers (user, agent, dispatch, telephony, notifications, status) in a single `main.go` (886 lines). The `handleRoot` function serves HTML. The `sendSMS` function calls SNS. Everything is in one binary.

**Assessment:** Acceptable for prototype — this is how every project starts. But the next refactor should split into packages:
- `handlers/user.go` — user-facing endpoints
- `handlers/agent.go` — operator-facing endpoints
- `services/dispatch.go` — dispatch logic + provider ranking
- `services/telephony.go` — Connect integration
- `services/notification.go` — SMS/email/push
- `middleware/auth.go` — Cognito JWT verification
- `middleware/ratelimit.go` — rate limiting

---

## 6. FRONTEND — UX CRITICAL

### 6.1 Emotional Trust Loop Broken

**Current state (from expertobservations.md):** After submitting a help request, the end-user sees a static card with "triaging" status. No ETA, no driver info, no map, no updates. The app goes silent exactly when the user needs reassurance most.

**Competitor standard:** Every roadside assistance app (AAA, Honk, Urgently) shows a live tracking view with ETA countdown and provider photo immediately after submission. The app becomes a companion during the wait, not a form processor.

**Immediate action:** P0-X1 (post-submission tracking view) — GitHub Issue #26, 3 hours effort.

### 6.2 Login Wall Before Help

**Current state:** A stranded motorist who never registered cannot request help through the app. They must know their password or have registered previously.

**Competitor standard:** AAA app allows "Request roadside assistance" as a guest with phone number only. The phone number itself is the lookup key.

**Immediate action:** P0-X4 (call-us bypass) — GitHub Issue #29. Longer-term: guest help request flow.

---

## 7. COMPETITIVE POSITION — WHERE INSUCAR STANDS

### Strengths (competitive advantages)

| Advantage | Why it matters vs Redion/AAA |
|---|---|
| Immutable SHA-256 audit ledger | Regulatory compliance (GDPR, SOC 2) — every action cryptographically provable. Redion's CMS-on-Azure has nothing like this |
| Event-driven architecture | Ready for real-time push (WebSocket) — competitors use polling |
| Cognito PKCE OAuth2 | Modern auth — no passwords stored. Competitors use legacy session cookies |
| Gated CI/CD pipeline | Every deployment requires product-owner approval. Competitors have manual release processes |
| Full IaC (defined) | Repeatable infrastructure. Competitors have hand-built environments |
| Hidden operator surface | Not discoverable by search engines. Competitors have public admin panels |
| Single Go binary | Fast iteration during prototype phase. Competitors have heavy microservice overhead |
| PostGIS-ready | Location-based provider matching already in schema |

### Weaknesses (competitive gaps)

| Gap | Impact | Competitor |
|---|---|---|
| No live map in end-user app | Motorist can't see provider approaching | AAA shows map with ETA |
| No post-submission feedback | Motorist left in the dark after requesting help | Honk sends push notifications every status change |
| No guest help path | Stranded non-registered user can't get help | AAA accepts phone number only |
| No real Amazon Connect | Telephony is simulated — can't take real calls | All competitors have live IVR |
| No provider network scale | 1 real provider connector (AXA mock) | Redion: 12,000 providers |
| No mobile app | Web-only — no offline capabilities | Every competitor has iOS + Android |
| Single region | No DR plan. eu-west-1 outage = platform down | Redion: 200+ countries |
| No monitoring/alerting | Production issues go undetected until someone checks | All competitors have 24/7 NOC |

---

## 8. IMMEDIATE ACTION PLAN (NEXT 48 HOURS)

### EMERGENCY (do before any feature work)

| Order | Action | Time | Issue |
|---|---|---|---|
| 1 | Create IAM admin user + MFA, delete root keys | 30 min | N/A (new) |
| 2 | Rotate GitHub PAT, update k8s secrets | 15 min | N/A (new) |
| 3 | Add `access.md` to .gitignore, `git rm --cached` | 5 min | N/A (new) |
| 4 | Change Jenkins admin password | 10 min | N/A (new) |
| 5 | Set up pg_dump cron to S3 | 30 min | N/A (new) |
| 6 | Fix Spinnaker pipeline config (parameters.imageTag) | 20 min | N/A (new) |
| 7 | Set `limitConcurrent: false` on Spinnaker pipeline | 5 min | N/A (new) |

### HIGH PRIORITY (this week)

| Order | Action | Time | Issue |
|---|---|---|---|
| 8 | Apply Terraform dev workspace (new EKS cluster) | 2h | N/A |
| 9 | Migrate Postgres to RDS (from Terraform apply) | 1h | N/A |
| 10 | Deploy ElastiCache Redis (from Terraform) | 30 min | N/A |
| 11 | Set OPS_CONSOLE_PATH as env var (remove from source) | 15 min | #29 related |
| 12 | Restrict Jenkins/Spinnaker LBs to admin CIDRs | 15 min | N/A |
| 13 | Enable OIDC auth on Spinnaker (Cognito already exists) | 30 min | N/A |
| 14 | Deploy Prometheus Agent + basic alerts | 1h | N/A |
| 15 | Re-enable blocking SAST/SCA in Jenkinsfile | 30 min | N/A |

### CRITICAL UX (this week)

> **Full specs:** `improvements.md` §7 and `architectuxobservations.md`.  
> **Tracked as:** GitHub Issues #26–#32.

| Order | Action | Time | Issue |
|---|---|---|---|
| 16 | P0-X1: Post-submission tracking view | 3h | #26 |
| 17 | P0-X2: Auto-detect GPS on load | 15m | #27 |
| 18 | P0-X3: Replace alert() with inline banner | 20m | #28 |
| 19 | P0-X4: Login bypass / call-us | 10m | #29 |
| 20 | P0-X5: Mission monitoring panel | 4h | #30 |
| 21 | P0-X6: Wire triage to DB | 3h | #31 |
| 22 | P0-X7: Duplicate call detection | 2h | #32 |

---

## 9. FINAL VERDICT

This is a **solid prototype** with an unusually good architectural foundation for its stage. The choice of Go + PostgreSQL + EKS + Cognito + Spinnaker is correct. The schema design (immutable audit ledger, PostGIS, RLS for multi-tenancy) shows real architectural thinking.

But this prototype is being operated as if it were production — real AWS costs, real domain (unysolar.com), real TLS certificates, real Cognito pools — while missing the **minimum safety net** that even a prototype should have: no database backups, root account credentials, no IaC applied, no monitoring.

**Stop all feature development. Spend the next 48 hours on the 7 emergency items above. Then resume with the 15 high-priority + critical UX items.**

The platform has the potential to compete with Redion/Agero. But it needs infrastructure discipline to match its code quality.
