# Insucar — Agent Guide

## Project
Roadside assistance platform (Go backend, React frontend, AWS/EKS/Spinnaker CI/CD).

## Stack
- **Backend:** Go 1.25, PostgreSQL/PostGIS, Redis, AWS SNS/EventBridge
- **Frontend:** Vanilla HTML/JS (SPA), Cognito OAuth2 PKCE
- **Infra:** AWS (ECR, EKS, RDS, ElastiCache, Cognito, Route53)
- **CI/CD:** Jenkins → Kaniko → ECR → Spinnaker (gated dev→uat→prod)
- **IaC:** Terraform (3 accounts: dev/uat/prod)

## Project Layout
```
insucar/
├── prototype/backend/   # Go API (main.go + handlers)
│   └── web/             # HTML/JS frontends (enduser, operator, landing)
├── ci/                  # Jenkinsfile, values, CICD docs
├── spinnaker/           # SpinnakerService, pipeline JSON
├── k8s/                 # Kubernetes manifests (insucar-api.yaml)
├── terraform/           # IaC: VPC, EKS, RDS, Cognito, IAM
├── db/                  # SQL schema + seed files
├── scripts/             # cognito-setup.sh + helpers
├── observations/        # Competitor research, gap analysis
├── design/              # UI mockups, logos
└── prompt/              # Master build prompt, architecture diagrams
```

## CI/CD Flow (MANDATORY)
1. **All changes go through git** — commit, push to `ugry/insucar` main branch
2. **Jenkins `insucar-ci`** builds automatically (or trigger via API)
3. **Kaniko** builds Go binary, pushes to ECR `326804802908.dkr.ecr.eu-west-1.amazonaws.com/insucar-api`
4. **Spinnaker pipeline** `deploy-insucar-api`: Deploy DEV → judgment → UAT → PO judgment → PROD
5. **Live namespace** `insucar` for direct deployment; `insucar-dev/uat/prod` managed by Spinnaker

## Authentication
- **Cognito SSO:** 3 user pools (customer/staff/partner), PKCE OAuth2 flow
- **Demo fallback:** Cookie-based auth preserved when COGNITO_ISSUER is unset
- **JWT verification:** `cognito.go` validates RS256 against JWKS, maps groups to roles

## Key Credentials (access.md — DO NOT EXPOSE)
- AWS Account: 326804802908 (eu-west-1)
- Jenkins: http://a69a0dc446e674657ac3fae06d8dd559-1651454478.eu-west-1.elb.amazonaws.com:8080
- Spinnaker Deck: http://a4977860e39434f278d0b4dedbcd4bb5-449340997.eu-west-1.elb.amazonaws.com
- Spinnaker Gate: http://afac25beae62d4f0cab340b254e5e6f2-1288246793.eu-west-1.elb.amazonaws.com

## Git Conventions
- Branch: `main` (push directly)
- Commit format: `type(scope): description` (e.g. `feat(auth): Cognito SSO`)
- Never commit `access.md` changes (contains live secrets)

## Key Files for Context
- `prompt/agenticpromptinsucar.md` — Master build specification
- `CONTINUE-HERE.md` — Handoff notes with current state + next steps
- `access.md` — All live URLs, credentials, endpoints
- `build-notes.md` — Architectural decisions
- `milestone.md` — Project history, failures, resolutions

## Remaining Priority Tasks (from CONTINUE-HERE.md)
1. ✅ Rich operator console (auto-refresh queue, real screen-pop, SLA timers, provider fallback UI)
2. ✅ Multi-tenant in code (tenant resolution + Row-Level Security)
3. ✅ Real provider connectors (webhook receiver, fallback chain, circuit breaker)
4. ✅ Telephony foundation (PSAP transfer, call state lifecycle — mock Connect)
5. ⬜ HA data (RDS Multi-AZ, separate AWS accounts per tier) — **after-release**
6. ⬜ Spinnaker pipeline improvements (canary/Kayenta, parameterized image tag) — **after-release**
7. ⬜ Security hardening (rotate root keys, SSO+TLS on Jenkins/Spinnaker) — **after-release**
8. ⬜ Brand consistency — **after-release**
