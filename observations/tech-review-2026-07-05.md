# Insucar — Technology & Infrastructure Review
Date: 2026-07-05

## 1. EKS Optimization Strategies (P0)

### Current State

**k8s manifests analysis:**

| Resource | Replicase | CPU Request | CPU Limit | Memory Request | Memory Limit | Image |
|----------|-----------|-------------|-----------|----------------|--------------|-------|
| `insucar-api` | 2 | 100m | 500m | 64Mi | 256Mi | `:1` (Kaniko CI) |
| `insucar-worker` | 2 | 50m | 250m | 32Mi | 128Mi | `:latest` |
| `postgres` | 1 | — | — | — | — | postgis:15-3.4 |
| `provider-axa` | 1 | — | — | — | — | http-echo |
| **HPA** | 2-6 | Target: 60% CPU | — | — | — | — |

### Critical Findings

#### 1. Memory Limits Too Low — Risk of OOMKill
The insucar-api deployment requests only **64Mi memory** with a **256Mi limit**. Go applications with pgxpool connections typically need at least 128-256Mi baseline. Under load, the app could experience OOMKills.

**Recommendation:**
```yaml
resources:
  requests: { cpu: "100m", memory: "128Mi" }
  limits:   { cpu: "500m", memory: "512Mi" }
```

#### 2. Postgres is In-Cluster (SPOF) — NOT Multi-AZ
The `k8s/postgres.yaml` file itself is documented as DEPRECATED (line 1): *"superseded by Amazon RDS Multi-AZ (see terraform/rds.tf)"*. As confirmed in `CONTINUE-HERE.md`, RDS Multi-AZ is not yet deployed.

**This is the highest infrastructure risk.** The current setup has:
- Single Postgres pod — zero HA
- No persistent volume — data lost on pod restart
- No automated backups
- No read replicas

**Recommendation:** Migrate to RDS Multi-AZ immediately (P0). Keep in-cluster only for local dev.

#### 3. Ingress in Wrong Namespace
`insucar-ingress.yaml` (line 5) shows namespace `insucar-prod`. This was fixed per `build-notes.md` — the ingress has been moved. However, the k8s manifests for `insucar-api.yaml`/`insucar-api-hpa.yaml` show namespace `insucar`. This mismatch could cause operational confusion.

#### 4. IRSA Role Not Configured
`insucar-api.yaml` line 9 shows: `eks.amazonaws.com/role-arn: REPLACE_WITH_app_irsa_role_arn`. This placeholder suggests IRSA is not wired — the app may be using static AWS credentials or default node roles.

#### 5. HPA Configuration is Reasonable
```yaml
minReplicas: 2, maxReplicas: 6
CPU target: 60%
Scale-up: stabilizationWindowSeconds: 30, +2 pods in 30s
Scale-down: stabilizationWindowSeconds: 120, -1 pod per 60s
```
This is a good configuration for a moderate-traffic API. Consider adding memory-based scaling for better protection.

#### 6. No PodDisruptionBudget
No PDB is defined. During node maintenance/upgrades, both API pods could be terminated simultaneously.

**Recommendation:**
```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: insucar-api-pdb
  namespace: insucar
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: insucar-api
```

#### 7. Cost Optimization Opportunities

| Strategy | Current | Recommendation | Savings |
|----------|---------|----------------|---------|
| Spot instances | Not used | Use spot for dev/uat namespaces | ~70% on dev/uat |
| Resource right-sizing | 64Mi request (too low) | 128Mi request | Avoid OOM + proper bin packing |
| Idle resource waste | 2 replicas always | Scale to 1 at night if applicable | ~30% |
| Compute savings plans | Not configured | 1-year compute savings plan | ~30% on EC2 |
| Graviton (ARM) instances | Not specified | Switch to ARM (Graviton) | ~20% cheaper + better perf |
| EBS gp3 | Default (probably gp2) | Switch to gp3 | ~20% cheaper per GB |

---

## 2. Spinnaker Deployment Best Practices (P1)

### Current Pipeline Analysis

From `PIPELINE-NOTES.md` and `insucar-deploy.json`:

```
Deploy DEV → Smoke DEV → [judgment] →
Deploy UAT → Smoke UAT → [judgment: PO] →
Deploy PROD → Smoke PROD → [Rollback on failure] → Verify PROD
```

### Strengths
- **Smoke tests:** Green health check at each stage — prevents deploying broken code
- **Auto-rollback:** `undoRolloutManifest` runs when PROD smoke fails — good safety net
- **Parameterized image tag:** Accepts `imageTag`, `gitCommit`, `gitAuthor` from CI webhook
- **Jenkins CI integration:** Webhook trigger from Jenkins at end of pipeline
- **OIDC Auth:** Cognito staff pool for Spinnaker auth via OAuth2
- **Fiat RBAC:** `insucar-developers` (READ), `insucar-releasers/product-owners` (WRITE) — good separation

### Gaps & Recommendations

#### 1. Canary Analysis (Kayenta) — Not Wired
From `PIPELINE-NOTES.md` line 28-34: *"Kayenta automated canary analysis — not yet wired."*

**Recommendation:**
- Requires: Amazon Managed Prometheus + Grafana for metrics
- Add `Canary Analysis` stage between UAT and PROD
- Compare error rates, latency p95, throughput against baseline
- Can block PROD promotion if metrics degrade

#### 2. Red/Black (Blue-Green) Deployment — Not Implemented
Current approach is rolling update with rollback via `undoRolloutManifest` (1 revision back). For true blue-green:
- Option A: Versioned Deployments (`insucar-api-vNNN`) with Service selector cutover
- Option B: Argo Rollouts CR with blue-green strategy

**Recommendation:** For a roadside assistance platform where availability is critical, blue-green provides faster rollback and zero-downtime cutover. Rolling update is acceptable interim.

#### 3. Manifest Duplication Across Namespaces
From `PIPELINE-NOTES.md` line 38-40: Three deploy stages duplicate the same manifest for `insucar-dev/uat/prod`. 

**Recommendation:** Use a pipeline template or `managed-delivery` deliveryConfig with a parameterized namespace.

#### 4. Expose as LoadBalancer Instead of Ingress
From `spinnakerservice.yml` lines 88-96: Deck/Gate exposed as NLB LoadBalancer. While this works, best practice is:
- Set `type: ClusterIP` 
- Route through ingress with TLS termination
- Lock to admin CIDRs via security group

#### 5. Spinnaker Canary Config
`spinnaker/pipelines/insucar-canary-config.json` exists as a stub. This should be the target for canary analysis — but currently blocked on observability stack.

---

## 3. CI/CD Pipeline Improvements (P2)

### Current Jenkins Pipeline Review

From `ci/Jenkinsfile` (168 lines):

**Current stages:**
1. Checkout
2. Test & vet (Go) — `go mod download`, `go vet`, `go test`
3. SAST & SCA — `govulncheck`, `gosec` (non-blocking)
4. Build & push API (Kaniko → ECR)
5. Build & push Worker (Kaniko → ECR) — optional (`&& true`)
6. SBOM (syft) — SPDX JSON
7. Image scan (trivy) — HIGH,CRITICAL
8. Trigger Spinnaker deploy

### Strengths
- **Security gates:** govulncheck + gosec + trivy — comprehensive
- **SBOM generation:** SPDX format — good for compliance
- **Kaniko builds:** No Docker daemon required — secure
- **5 Go containers:** golang:1.25, kaniko, syft, trivy, tools — each specialized
- **Git metadata:** SHA, author propagated to Spinnaker

### Gaps & Recommendations

#### 1. SAST/SCA Is Non-Blocking
Lines 83-89: `govulncheck ./... || true` and `gosec ... || true` — failures are silently ignored. For a mission-critical platform handling emergency calls, these should block the pipeline.

**Recommendation:**
```groovy
sh 'govulncheck ./...'  // remove || true — fail on vulns
sh 'gosec -severity medium -confidence medium ./...'  // fail on medium+
```

#### 2. Test Coverage Gate Missing
Line 66: `go test -coverprofile=coverage.out ./...` runs tests but doesn't enforce a coverage threshold. Current coverage is 1.0%.

**Recommendation:**
```groovy
go test -coverprofile=coverage.out -coverpkg=./... ./...
go tool cover -func=coverage.out | tail -1
// Parse and fail if below threshold (e.g., 60% for core, 30% for handlers)
```

#### 3. No Integration Tests
Only unit tests. No tests against a real database, no API-level E2E tests in CI.

**Recommendation:** Add a `make test-integration` target that spins up test Postgres + runs API tests.

#### 4. Single-Branch Trigger
Currently triggers on any push to `main`. No branch/PR pipeline separation.

**Recommendation:**
- `main`: Full CI → Spinnaker
- `feature/*`: Test + vet + SAST only (no deploy)
- PRs: Test + vet + SAST + comment results on PR

#### 5. No Docker Image Labeling
Images are tagged with `BUILD_NUMBER` and `latest`. No semver, no git SHA tag.

**Recommendation:**
```groovy
--destination="$ECR:$TAG" \
--destination="$ECR:latest" \
--destination="$ECR:sha-${GIT_SHA}"  // add SHA tag for traceability
```

#### 6. Parallel Stages Opportunity
SBOM + Image scan stages could run in parallel with worker build, reducing total pipeline time.

#### 7. Worker Build Optional
`prototype/worker` build uses `&& true` — if it fails, the pipeline still passes. This should be a required stage when worker is production-critical.

---

## 4. RLS Connection Pinning Issue

From `CONTINUE-HERE.md`: *"RLS: SET LOCAL doesn't carry across pgxpool connections (needs per-request connection pinning)"*

### Root Cause
`tenantMiddleware()` (tenant.go:114-117) sets `SET LOCAL app.current_tenant` on whatever connection pgxpool returns. On subsequent queries within the same request, pgxpool may return a **different** connection where the SET LOCAL was never executed — RLS policies then fail or filter incorrectly.

### Fix Options
1. **BeforeQuery hook (pgxpool):** Register an `AfterConnect` hook that always sends `SET app.current_tenant TO '<tid>'` — but this requires the tenant_id to be available at connect time, not request time.
2. **Per-request connection pinning:** Use `db.Acquire(ctx)` to hold a dedicated connection for the request lifecycle, then release it. This reduces pool efficiency but guarantees RLS correctness.
3. **Explicit WHERE clauses:** Add `tenant_id = $X` to every query instead of relying on RLS — more code but most predictable.
4. **Custom pool wrapper:** A middleware that interposes between `db.Exec`/`db.Query` and always prepends `SET LOCAL` before each query — fragile.

**Recommendation:** Option 2 (per-request pinning) is the safest short-term fix. Option 3 is the long-term best practice. The `tenantMiddleware` should be rewritten to pin a connection for the duration of the request.

---

## Summary: Top Infrastructure Actions

| Priority | Action | Impact | Effort |
|----------|--------|--------|--------|
| P0 | Migrate Postgres to RDS Multi-AZ | **Survival** — prevents total data loss | High |
| P0 | Fix RLS connection pinning | **Critical** — security/data integrity | Medium |
| P0 | Increase memory limits (64Mi→128Mi+) | **Critical** — prevents OOMKill in prod | Low |
| P1 | Add PodDisruptionBudget | **High** — prevents full outage during node ops | Low |
| P1 | Enable SAST/SCA gates to block pipeline | **High** — prevents vulnerable code in prod | Low |
| P1 | Add test coverage gate (min 30%) | **High** — code quality | Medium |
| P2 | Blue-green deployment (Argo Rollouts) | **Medium** — faster rollback | High |
| P2 | Canary analysis (Kayenta) | **Medium** — automated quality gate | High |
| P2 | Spot instances for dev/uat | **Medium** — cost savings | Low |
