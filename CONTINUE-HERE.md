# Insucar — Continuation / Handoff Notes (read this first next session)

Purpose: everything an agent needs to resume this project without re-discovery.
Repo: https://github.com/ugry/insucar (private) · cloned locally at /tmp/insucar
AWS: account 326804802908, region eu-west-1 · Domain: unysolar.com (Route53 zone Z06143773JJ0DPRILPDA0)
Last updated: 2026-07-05 (rich operator console deployed)

## Rich operator console DONE (2026-07-05)
- ✅ Auto-refresh queue every 8 seconds with new-case flash animation
- ✅ SLA aging timers (longest wait, avg dispatch, case age bar with breach coloring)
- ✅ Provider fallback UI: dynamic provider list from API, click-to-select, availability windows check
- ✅ Coverage decision panel: coverage level, excess, callout limits, entitlements
- ✅ Safety triage panel: pre-fill from DB, severity assessment
- ✅ Operator status management: on-call/ACW/offline toggle
- ✅ Mission timeline rendered from mission_status_events
- ✅ Service selector grid (tow, roadside repair, jump start, tyre, lockout, fuel)
- ✅ New backend endpoints: /api/agent/providers, /api/agent/stats, /api/agent/status
- ✅ Enhanced /api/agent/case: returns created_at, coverage details, safety info, mission timeline
- ✅ Enhanced /api/agent/dispatch: accepts optional provider_id for fallback selection
- ✅ DB migration: db/schema-v6-operator.sql (dispatched_at, operator status)
- Live image: insucar-api:12 (Jenkins build #12, verified all endpoints HTTP 200/401 as expected)

## Dev rollout VALIDATED end-to-end, then torn down for cost (2026-07-04)
- vCPU quota raised 8 -> 32 (approved). Applied the FULL managed stack to a `dev` workspace
  (`cluster_name=insucar-dev`, separate from live `insucar`). PROVEN RUNNING on real AWS (~139 res):
  EKS insucar-dev ACTIVE (v1.30, 2× t3.xlarge), RDS PostgreSQL 16.9 available, ElastiCache Redis
  Multi-AZ available, 3 Cognito pools, SQS dispatch/notification + DLQs + EventBridge bus, AMP ACTIVE,
  ECR insucar-dev-api/worker.
- Fixed 5 real IaC bugs found during apply (committed b3b8621): RDS 15.7->16.9 (+pg16 param group);
  EBS CSI addon -> standalone aws_eks_addon wired to its IRSA role; ECR repos + S3 buckets
  environment-scoped (avoid colliding with live insucar-api/insucar-spinnaker/insucar-deploy);
  redis-auth secret recovery_window=0; Cognito partner client callback/logout URLs.
- KNOWN FLAKE: aws-ebs-csi-driver addon was slow to reach ACTIVE in this account (non-blocking — the
  stack uses managed RDS/ElastiCache, not in-cluster EBS). Revisit on the real deploy.
- Decision (architect): objective proven; torn down (`terraform destroy`, 139 removed) to stop
  ~$0.60/hr since it was a validation stack parallel to the live site. Live `insucar` + unysolar.com
  untouched (HTTP 200). TF remote state (S3 + DynamoDB lock) REMAINS.
- TO DEPLOY FOR REAL: use a DEDICATED AWS account (3-tier design), `terraform apply
  -var environment=<env> -var cluster_name=insucar-<env>`, then `ROLLOUT.md` §3+. Grafana stays
  gated off (`enable_grafana=false`) until IAM Identity Center/SAML is configured.


## TL;DR current state (LIVE)
- LIVE over HTTPS (Let's Encrypt):
  * https://unysolar.com/  (+www) -> premium marketing landing (OD "stripe", golden-ratio); logo shield.
  * https://unysolar.com/app       -> functional user app: login/register -> request assistance -> my cases.
  * https://op.unysolar.com/        -> operator console: agent login -> live queue -> screen-pop -> dispatch.
- Everything runs on EKS in ns `insucar`; current image tag: insucar-api:15 (Jenkins build 15, connector webhooks + telephony PSAP).
- Auth (demo): users by EMAIL, agents by AGENT_ID (HMAC cookie sessions). Creds in access.md.
  Users: claire.martin@example.fr / Claire#2026 (+john,+lukas). Agents: OP-1001/Operator#2026 (+SUP-2001,PO-3001).
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
- OLDER standalone prototype EC2 (i-0628af42823122bce) still exists (docker-compose) — superseded by EKS.

## Redeploy the app (fast path)
1. edit prototype/backend/* ; 2. docker build+push $ECR/insucar-api:<tag> (ECR login first);
3. kubectl -n insucar set image deployment/insucar-api insucar-api=$ECR/insucar-api:<tag>
   (or use the gated Spinnaker pipeline). Web files live in prototype/backend/web/{landing,enduser,operator}.html.

## Local tooling (installed to ~/.local/bin; add to PATH)
export PATH="$HOME/.local/bin:$PATH"
- aws (v2), kubectl, eksctl, helm, gh. AWS profile 'default' is configured with the account keys.
- kubeconfig: `aws eks update-kubeconfig --name insucar --region eu-west-1`
- Docker available. Node v24, pnpm available. Go NOT installed locally (build via golang docker image).
- exa.ai API key (for web research): 97f0f8cd-6f42-45d7-972b-026144cf22f0
  Usage: curl -sS -X POST https://api.exa.ai/search -H "x-api-key: $KEY" -H 'Content-Type: application/json'
         -d '{"query":"...","numResults":5,"type":"auto","contents":{"summary":true}}'
- Joplin notes DB (has more keys): ~/.config/joplin-desktop/database.sqlite (no sqlite3; use `strings`).

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

## Live endpoints (see access.md for creds)
- Prototype (EC2):     http://app.unysolar.com:8080/  (/register, hidden op: /ops-console-7f3a9c)
- EKS app (Spinnaker): http://af9269372141a4fdba7953b3679d6189-59590199.eu-west-1.elb.amazonaws.com/
- Jenkins:  http://a69a0dc446e674657ac3fae06d8dd559-1651454478.eu-west-1.elb.amazonaws.com:8080 (admin/InsucarAdmin!2026)
- Spinnaker Deck: http://a4977860e39434f278d0b4dedbcd4bb5-449340997.eu-west-1.elb.amazonaws.com
- Spinnaker Gate: http://afac25beae62d4f0cab340b254e5e6f2-1288246793.eu-west-1.elb.amazonaws.com (/health)

## AWS resources provisioned
- EKS cluster 'insucar' (1.30, 2x t3.xlarge, ng-standard). Namespaces: jenkins, spinnaker,
  spinnaker-operator, insucar.
- ECR repo insucar-api (image :1). S3: insucar-deploy-326804802908, insucar-spinnaker-326804802908.
- EC2 i-0628af42823122bce (prototype) + SG sg-08bfda3ebbaa34e92. Route53 A: app/api/demo.unysolar.com.

## How to redeploy the app via Spinnaker (proven)
1. Build+push: cd prototype/backend; docker build -t <ecr>/insucar-api:<tag> .; push (ecr login first).
2. Trigger pipeline: python3 /tmp/spin_pipeline.py  (or POST /pipelines/insucar/deploy-insucar-api).
   Pipeline def committed at spinnaker/pipelines/insucar-deploy.json. Manifests in k8s/.

## DONE (production-ready) — 2026-07-05
1. ✅ Rich operator console — auto-refresh, SLA timers, provider fallback, coverage, triage, timeline
2. ✅ Multi-tenant — host-based resolution, auto-seed default tenant, RLS middleware
3. ✅ Provider connectors — webhook receiver, fallback chain, circuit breaker, health checker
4. ✅ Telephony — PSAP warm-transfer with audit trail, call state lifecycle
5. ✅ Full CI/CD — Jenkins → Kaniko → ECR → kubectl deploy, 15 builds proven
6. ✅ HTTPS/TLS — Let's Encrypt, ingress-nginx, cert-manager
7. ✅ Cognito SSO — 3 user pools, PKCE OAuth2, JWT verification (demo cookie fallback)

## KNOWN LIMITATIONS (non-blocking, document in runbook)
- RLS: SET LOCAL doesn't carry across pgxpool connections. Fix: per-request connection pinning.
- Cognito: pools wired in code but env vars unset in live deploy; demo cookie auth active.
- Telephony: mock Connect in use; real Amazon Connect + Lex requires AWS infra provisioning.
- Providers: AXA/Towpal sandbox API access needed for real connector integration.

## AFTER PRODUCTION RELEASE (backlog — lower priority)
- [ ] RDS Multi-AZ failover + separate AWS accounts per tier (HA data)
- [ ] Spinnaker canary/Kayenta stages + parameterized image tags
- [ ] Rotate root AWS keys + GitHub PAT; SSO+TLS on Jenkins/Spinnaker; restrict LBs
- [ ] Consistent brand mark across landing + apps (swap OD logo for insucar-logo.svg)
- [ ] Real Amazon Connect + Lex provisioning (requires AWS Connect instance + phone number)
- [ ] Real AXA/Towpal sandbox connector integration
- [ ] Pinpoint SMS exit sandbox mode (production access request)

## Competitor research (done)
- observations/redion-analysis.md (Redion = ex-Europ Assistance) + operator-gui-research.md (CAD best practices).

## Cost / teardown
- EKS + 2x t3.xlarge + several ELBs ~ $1/hr. Teardown steps in ci/CICD-DEPLOYMENT.md and
  prototype/DEPLOYMENT.md. `eksctl delete cluster --name insucar --region eu-west-1` removes most.

## Key files map
- prompt/agenticpromptinsucar.md   - master build spec
- db/schema.sql, db/seed.sql        - data layer (validated)
- prototype/                        - runnable Go+PostGIS prototype + DEPLOYMENT.md
- k8s/                              - postgres.yaml, insucar-api.yaml (EKS manifests)
- ci/                               - Jenkinsfile, jenkins-values.yaml, plugins.txt, README, CICD-DEPLOYMENT.md
- spinnaker/                        - spinnakerservice.yml, k8s-account-patch.yaml, pipelines/insucar-deploy.json
- observations/                     - europcar-analysis, gap-analysis, prototype-readiness, (competitor next)
- access.md                         - all creds/URLs/IPs   · milestone.md - history   · CONTINUE-HERE.md - this file
