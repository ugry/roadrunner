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
