# Insucar — Agent Manifestation

> Created: 2026-07-05
> Defines: AI agent team structure, permissions, workflow, and CI/CD gates
> Status: Active

---

## Architecture

```
                     ┌──────────────────────────────────┐
                     │       HUMAN PRODUCT OWNER        │
                     │    (Final PROD approver only)    │
                     │    Spinnaker Deck — manual click │
                     └───────────────┬──────────────────┘
                                     │ PROD judgment (manual only)
                     ┌───────────────┴──────────────────┐
                     │          ORCHESTRATOR            │
                     │    Coordinates team, manages     │
                     │    CI/CD, approves DEV→UAT       │
                     │    insucar / insucar-orchestrator│
                     └──┬──────────┬───────────┬────────┘
                        │          │           │
               ┌────────┴──┐ ┌─────┴──────┐ ┌──┴───────────┐
               │ DEVELOPER │ │   TESTER   │ │  RESEARCHER  │
               │ writes     │ │ tests APIs │ │ analyzes     │
               │ code       │ │ QA issues  │ │ researches   │
               │ commits    │ │ verifies   │ │ reports      │
               │ builds     │ │ deploys    │ │              │
               └───────────┘ └────────────┘ └──────────────┘
```

---

## Agent Definitions

### 1. Orchestrator
| | |
|---|---|
| **File** | `.opencode/agents/insucar.md` |
| **Alias** | `insucar`, `insucar-orchestrator` |
| **Mode** | `subagent` |
| **Permissions** | `edit`, `bash`, `task`, `webfetch`, `websearch` |

**Responsibilities:**
- Read AGENTS.md to determine current milestone and priorities
- Delegate work to Developer, Tester, Researcher via `task` tool
- Trigger Jenkins builds after Developer pushes code
- Monitor Jenkins build status until SUCCESS
- Approve Spinnaker **DEV→UAT** judgment (automatic, continuous)
- **NEVER** approve PROD judgment — reserved for human PO
- Verify deployment health after each deploy
- Report progress at end of each session

**CI/CD Commands:**
```bash
# Trigger Jenkins
J="http://a69a0dc446e674657ac3fae06d8dd559-1651454478.eu-west-1.elb.amazonaws.com:8080"
CRUMB=$(curl -s -c /tmp/cj --user admin:InsucarAdmin!2026 "$J/crumbIssuer/api/json" | jq -r .crumb)
curl -b /tmp/cj --user admin:InsucarAdmin!2026 -H "Jenkins-Crumb: $CRUMB" -X POST "$J/job/insucar-ci/build"

# Monitor Spinnaker
GATE="https://gate.unysolar.com"
curl -s "$GATE/applications/insucar/pipelines?limit=5&expand=false"

# Approve UAT judgment
curl -s -X PATCH "$GATE/pipelines/<execId>/stages/<stageId>" \
  -H 'Content-Type: application/json' -d '{"judgmentStatus":"continue"}'
```

### 2. Developer
| | |
|---|---|
| **File** | `.opencode/agents/insucar-developer.md` |
| **Mode** | `subagent` |
| **Permissions** | `edit`, `bash` |

**Responsibilities:**
- Read relevant source files before making changes
- Write Go, HTML, JS, YAML code following existing patterns
- Run `go build`, `go vet`, `go test` locally before committing
- Commit with conventional commit messages: `type(scope): description`
- Push to GitHub branch `main`
- Report commit hash and changed files to Orchestrator

**Cannot:**
- Approve Spinnaker judgments
- Deploy directly to production (`kubectl apply`)
- Modify infrastructure or AWS resources
- Create GitHub issues (Tester's job)
- Touch `.opencode/` agent configurations

**Tech Stack:**
| Layer | Technology | Location |
|-------|-----------|----------|
| Backend | Go 1.25 | `prototype/backend/` |
| Frontend | Vanilla HTML/JS | `prototype/backend/web/` |
| Database | PostgreSQL | queries in Go handlers |
| Infra | K8s manifests | `k8s/` |
| CI/CD | Jenkinsfile | `ci/` |

### 3. Tester
| | |
|---|---|
| **File** | `.opencode/agents/insucar-tester.md` |
| **Mode** | `subagent` |
| **Permissions** | `edit`, `bash`, `webfetch` |

**Responsibilities:**
- Run end-to-end API tests via `curl` against live endpoints
- Verify TLS certificates, cookie flags (Secure, HttpOnly, SameSite)
- Run `go test ./...` for backend unit tests
- Check Kubernetes pod health (`kubectl get pods`)
- Verify page loads: landing, app, operator, admin
- Create GitHub issues with repro steps, expected vs actual, evidence
- Report test results with pass/fail counts

**Test Environments:**
| Environment | URL |
|------------|-----|
| Production | https://unysolar.com |
| Operator | https://op.unysolar.com |
| Jenkins | https://jenkins.unysolar.com |
| Spinnaker | https://spinnaker.unysolar.com |

**Bug Severity:**
| Level | Label | Example |
|-------|-------|---------|
| CRITICAL | `P0:survival` | App down, data loss, security breach |
| HIGH | `P1:critical` | Core feature broken, users blocked |
| MEDIUM | `P2:important` | Feature degraded, workaround exists |
| LOW | `P3:nice-to-have` | Cosmetic, edge case |

### 4. Researcher
| | |
|---|---|
| **File** | `.opencode/agents/insucar-researcher.md` |
| **Mode** | `subagent` |
| **Permissions** | `edit`, `bash`, `webfetch`, `websearch` |

**Responsibilities:**
- Web search for competitor analysis (Allianz, AXA, AAA, RAC, Swoop, Urgently)
- Security research: OWASP Top 10, Go CVEs, Cognito best practices
- Technology trends: multi-tenant patterns, EKS optimization, Spinnaker improvements
- UX analysis: WCAG 2.1 compliance, mobile-first patterns, competitor UI teardowns
- Document findings in `observations/` directory
- Report key findings and recommendations to Orchestrator

**Cannot:**
- Write application code
- Commit changes outside `observations/`
- Deploy or modify infrastructure
- Create GitHub issues directly (pass findings to Tester or Orchestrator)

---

## Spinnaker Approval Rules

```
Pipeline: deploy-insucar-api (ID: 584af310-21f5-427d-ac3b-a0d5c065fc18)

  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
  │ Deploy DEV   │────▶│ Promote to   │────▶│ Deploy UAT   │────▶│ Promote to   │────▶│ Deploy PROD  │
  │ (auto)       │     │ UAT?         │     │ (auto)       │     │ PROD?        │     │ (auto)       │
  │ insucar-dev  │     │ ORCHESTRATOR │     │ insucar-uat  │     │ HUMAN PO     │     │ insucar-prod │
  └──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
```

| Stage | Trigger | Approver | Automation |
|-------|---------|----------|-----------|
| Deploy DEV | Jenkins webhook (auto) | — | Fully automated |
| Promote to UAT? | Manual judgment | **Orchestrator** | Auto-approved after DEV success |
| Deploy UAT | After judgment | — | Fully automated |
| Promote to PROD? | Manual judgment | **Human PO** | Must wait for human click |
| Deploy PROD | After judgment | — | Fully automated |

**Critical Rule:** The Orchestrator MUST NEVER approve the PROD judgment. That gate exists solely for the human Product Owner.

---

## CI/CD Flow (End-to-End)

```
 1. git push ─────────────▶ GitHub (main branch)
                                │
 2. Jenkins auto-triggered      ▼
    insucar-ci pipeline
       │
       ├─ podTemplate: Kaniko (build container)
       ├─ go vet + go test (compile check)
       ├─ Kaniko build (OCI image)
       ├─ Trivy vulnerability scan
       └─ Push to ECR as :{BUILD_NUMBER}
                                │
 3. Webhook                     ▼
    POST gate.unysolar.com/webhooks/webhook/insucar-ci
                                │
 4. Spinnaker pipeline          ▼
    deploy-insucar-api
       │
       ├─ Stage 1: Deploy DEV (insucar-dev)          AUTO
       ├─ Stage 2: Manual Judgment → UAT             ORCHESTRATOR
       ├─ Stage 3: Deploy UAT (insucar-uat)          AUTO
       ├─ Stage 4: Manual Judgment → PROD            HUMAN PO ← YOU
       └─ Stage 5: Deploy PROD (insucar-prod)        AUTO
                                                        │
 5. Live traffic                                        ▼
    Ingress → insucar-prod/insucar-api → unysolar.com
```

---

## Live Infrastructure Map

```
AWS Account: 326804802908  |  Region: eu-west-1  |  Cluster: insucar (EKS 1.30)

┌─────────────────────────────────────────────────────────────┐
│ Nodes: 2x t3.xlarge (ng-standard)                          │
│ CPU: ~7% used  |  Memory: ~36% used                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  cert-manager (3 pods)         ingress-nginx (1 pod)       │
│  ┌─────────────────┐          ┌─────────────────────┐      │
│  │ LetsEncrypt PROD │          │ TLS termination     │      │
│  │ auto-renew certs │          │ HTTPS → ClusterIP   │      │
│  └─────────────────┘          └─────────┬───────────┘      │
│                                         │                   │
│  ┌──────────────────────────────────────┘                   │
│  │                                                          │
│  ▼                                                          │
│  insucar-prod (LIVE)         insucar-dev     insucar-uat   │
│  ┌─────────────────┐    ┌─────────────┐ ┌─────────────┐   │
│  │ insucar-api x2  │    │ insucar-api │ │ insucar-api │   │
│  │ insucar-worker  │    │ insucar-wkr │ │ insucar-wkr │   │
│  │ ingress ✅      │    └─────────────┘ └─────────────┘   │
│  └─────────────────┘                                        │
│                                                             │
│  jenkins (1 pod)             spinnaker (9 pods)            │
│  ┌─────────────────┐    ┌─────────────────────────────┐   │
│  │ insucar-ci job  │    │ clouddriver, orca, gate,    │   │
│  │ 7 builds, 6✓    │    │ deck, front50, rosco,       │   │
│  └─────────────────┘    │ igor, echo, redis           │   │
│                         └─────────────────────────────┘   │
│                                                             │
│  insucar (retired, 0 replicas)                             │
│  ┌─────────────────┐                                       │
│  │ postgres (1 pod)│  ← shared database                   │
│  │ provider-axa    │  ← mock provider stub                │
│  └─────────────────┘                                       │
└─────────────────────────────────────────────────────────────┘
```

---

## URLs

| Service | URL | TLS |
|---------|-----|-----|
| End-user app | https://unysolar.com/app | ✅ LE |
| Operator console | https://op.unysolar.com/ | ✅ LE |
| Jenkins | https://jenkins.unysolar.com | ✅ LE |
| Spinnaker Deck | https://spinnaker.unysolar.com | ✅ LE |
| Spinnaker Gate | https://gate.unysolar.com | ✅ LE |
| GitHub | https://github.com/ugry/insucar | — |

---

## Agent Workflow Sequence

```
Human PO says: "Fix login bug for user ugur.yardimci@unygms.com"

  ┌─ ORCHESTRATOR ─────────────────────────────────────────────┐
  │ 1. Reads AGENTS.md, understands context                     │
  │ 2. Delegates to RESEARCHER:                                │
  │    task("Investigate login flow for login bounce-back",     │
  │         subagent_type="insucar-researcher")                 │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ RESEARCHER ──────────────▼────────────────────────────────┐
  │ 3. Reads cognito.go, main.go, enduser.html                 │
  │ 4. Discovers: COGNITO_ISSUER points to wrong pool          │
  │ 5. Verifies: different JWKS keys between pools             │
  │ 6. Returns: "Root cause: pool mismatch + cookie gap"       │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ ORCHESTRATOR ────────────▼────────────────────────────────┐
  │ 7. Reviews research findings                                │
  │ 8. Delegates to DEVELOPER:                                 │
  │    task("Fix cognito.go dual-verifier + enduser.html        │
  │          bearerApi cookie fix, commit and push",            │
  │         subagent_type="insucar-developer")                  │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ DEVELOPER ───────────────▼────────────────────────────────┐
  │ 9. Reads cognito.go, main.go, enduser.html                 │
  │ 10. Implements dual-verifier, Secure cookie, bearerApi fix │
  │ 11. go build + go vet passes                               │
  │ 12. Commits, pushes to GitHub                              │
  │ 13. Returns: "✅ Pushed commit abc123"                     │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ ORCHESTRATOR ────────────▼────────────────────────────────┐
  │ 14. Triggers Jenkins: curl -X POST .../build               │
  │ 15. Monitors build #41 → SUCCESS (273s)                    │
  │ 16. Approves Spinnaker DEV→UAT judgment                    │
  │ 17. Delegates to TESTER:                                   │
  │     task("Full E2E test: register, login, logout, re-login",│
  │          subagent_type="insucar-tester")                    │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ TESTER ──────────────────▼────────────────────────────────┐
  │ 18. Runs 7 E2E tests via curl                              │
  │ 19. Verifies Secure cookie, TLS, /api/me                   │
  │ 20. Returns: "✅ All 7 tests pass"                         │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ ORCHESTRATOR ────────────▼────────────────────────────────┐
  │ 21. Reports to HUMAN PO:                                   │
  │     "Fix ready for PROD. Pipeline at UAT, awaiting your    │
  │      approval in Spinnaker Deck."                          │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ HUMAN PO ────────────────▼────────────────────────────────┐
  │ 22. Opens https://spinnaker.unysolar.com                   │
  │ 23. Clicks "Approve" on Promote to PROD? judgment          │
  └────────────────────────────────────────────────────────────┘
                              │
  ┌─ SPINNAKER ───────────────▼────────────────────────────────┐
  │ 24. Deploys to insucar-prod (2 replicas, rolling update)   │
  │ 25. Pipeline: SUCCEEDED → all 5 stages green              │
  └────────────────────────────────────────────────────────────┘
```

---

## File Inventory

```
.opencode/agents/
├── insucar.md                    # Orchestrator (primary entry point)
├── insucar-orchestrator.md       # Orchestrator (detailed reference)
├── insucar-developer.md          # Developer agent
├── insucar-tester.md             # Tester agent
└── insucar-researcher.md         # Researcher agent

Project root:
├── AGENTS.md                     # Project guide + agent team overview
├── agentmanifestation.md         # THIS FILE — complete agent specification
├── build-notes.md                # Architecture decisions + incident log
├── access.md                     # Credentials (git-ignored, DO NOT EXPOSE)
├── CONTINUE-HERE.md              # Handoff notes with current state
└── milestone.md                  # Project history, failures, resolutions
```

---

## Agent Invocation

Agents are invoked through the Orchestrator using the `task` tool:

```
task(
  description: "Fix login bounce-back bug",
  prompt: "Detailed instructions for the agent...",
  subagent_type: "insucar-developer"
)
```

Available subagent types:
- `insucar` or `insucar-orchestrator` — project coordination
- `insucar-developer` — code changes
- `insucar-tester` — testing and QA
- `insucar-researcher` — research and analysis
