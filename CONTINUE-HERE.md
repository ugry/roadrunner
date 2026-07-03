# Insucar — Continuation / Handoff Notes (read this first next session)

Purpose: everything an agent needs to resume this project without re-discovery.
Repo: https://github.com/ugry/insucar (private) · cloned locally at /tmp/insucar
AWS: account 326804802908, region eu-west-1 · Domain: unysolar.com (Route53 zone Z06143773JJ0DPRILPDA0)
Last updated: 2026-07-03

## TL;DR current state
- Design + prompt: prompt/agenticpromptinsucar.md (v2, gap-hardened, no-mocks, quorum HA, Spinnaker,
  registration, separate hidden operator app). Diagram: prompt/insucar-architecture.{mmd,svg,pdf(A2/A3)}.
- DB: db/schema.sql + db/seed.sql (validated on postgis 15-3.4).
- Prototype (Go API + PostGIS + end-user app + hidden operator console): prototype/.
- Prototype deployed on a standalone EC2 (i-0628af42823122bce) at http://app.unysolar.com:8080
- Full CI/CD on EKS PROVEN end-to-end: ECR image -> Spinnaker deployManifest -> app on EKS.

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

## NEXT STEPS (priority order)
1. Wire Jenkins -> Spinnaker webhook trigger; run full git-push -> build -> ECR -> deploy once.
   (Jenkins agent needs Docker + AWS creds, or use Kaniko for in-cluster builds.)
2. Expand Spinnaker pipeline: bake -> dev -> manual judgment -> UAT -> canary/red-black ->
   product_owner judgment -> prod (per prompt).
3. Move prototype off single EC2 onto EKS as the canonical env (Spinnaker already can).
4. Build out real features from the prompt: Amazon Connect + Lex telephony, Keycloak SSO
   (customer + staff realms, separate apps), Rust inner-core vault, Pinpoint SMS, TLS (ACM+ALB),
   quorum-HA Postgres (Patroni), provider connectors (AXA/Towpal).
5. Harden: rotate root AWS keys + PAT, add OIDC/TLS to Jenkins/Spinnaker, restrict LBs.

## Competitor research
- Task in progress: analyze https://www.redion.com/mobility/ (see observations/ for output).

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
