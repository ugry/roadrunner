# Insucar — Build Notes / Decisions (session 2026-07-03)

Decisions taken by the product owner for this build phase:
- TELEPHONY: **Amazon Connect is MOCKED** (a mock telephony-adapter service that emits
  Connect-shaped events / screen-pop). Do NOT provision live Connect for now.
- PROVIDERS: **real provider connections** (HTTP connector calling a real external provider API;
  registry-driven). Plug real AXA/Towpal sandbox creds when available.
- SMS: **real SMS** (AWS SNS/Pinpoint). Note: a new AWS account may be in the SNS SMS sandbox
  (destinations must be verified) — delivery to arbitrary numbers may require moving out of sandbox.
- #13 (immutable prod-access blockchain ledger + JIT IAM approval): **IGNORED for now.**
- KUBERNETES: **2 worker nodes in HA** + **autoscaling** (cluster autoscaler for nodes, HPA for
  pods) to absorb demand/traffic spikes.
  NOTE (quorum caveat, documented): true stateful auto-failover needs an ODD quorum (3, or 2+witness).
  Compute HA at 2 nodes + autoscaling is fine; for HA Postgres add a witness later (Patroni/etcd).
- CI/CD: **Jenkins + Spinnaker must be functional** -> full Spinnaker pipeline
  (bake -> dev -> manual judgment -> UAT -> prod) is a MUST (#14).
- TESTING (#15): unit/integration + smoke tests wired into CI — important, include now.

Action: apply the v3 prompt from this baseline; (re)start the first prototype with the above.

## Executed (2026-07-03, this session)
- Autoscaling: HPA on insucar-api (cpu 60%, 2->6 pods) + Cluster Autoscaler (nodes 2->5,
  IRSA InsucarClusterAutoscaler). Nodegroup min2/max5. 2-node HA confirmed.
- Backend v2 (image :2): MOCK Amazon Connect adapter (/api/telephony/mock/incoming),
  REAL provider connector (PROVIDER_API_URL -> provider-axa svc; dispatch shows provider_source=api),
  REAL SMS via AWS SNS (node role granted AmazonSNSFullAccess; dispatch shows sms=sent).
  Unit tests run in the image build (go vet + go test).
- Full Spinnaker pipeline (#14) PROVEN: Deploy DEV -> manual judgment -> Deploy UAT ->
  product-owner judgment -> Deploy PROD (all SUCCEEDED). Deployed to insucar-dev/uat/prod.
- PROD app (deployed by the gated pipeline): http://ad4de17a313444704a74f62919bfabc7-1055718284.eu-west-1.elb.amazonaws.com/
  Verified: mock-connect screen-pop, real provider call (eta 22 from provider), SMS sent.

## Jenkins -> Spinnaker webhook (git push -> deploy) — PROVEN
- Spinnaker pipeline got a webhook trigger (source: insucar-ci).
- Jenkinsfile: Kubernetes agent -> Kaniko builds+pushes image to ECR (no Docker daemon) ->
  wget POST to Spinnaker webhook. Node role granted ECR PowerUser for Kaniko push.
- Jenkins job insucar-ci build #3 SUCCESS (checkout -> Kaniko build+push -> trigger Spinnaker).
- Spinnaker executed a webhook-triggered run: Deploy DEV -> (judgment) -> UAT -> (product-owner
  judgment) -> PROD, all SUCCEEDED. Full CI/CD chain functional.
Trigger URL: http://<gate-lb>/webhooks/webhook/insucar-ci

## Session 2026-07-03 (evening) — decisions & config changes
- Design systems chosen via Open Design: operator = "mission-control" (dark, CAD-style),
  landing/user = "stripe" (light, premium, golden-ratio/Fibonacci). Distinct on purpose.
- Domain model: users on unysolar.com (+www, +app), operators on op.unysolar.com. Host-based routing
  in the Go backend (op.* -> operator.html; apex -> landing.html at "/", enduser.html at /app,/login,/register).
- TLS: Let's Encrypt via cert-manager + ingress-nginx (HTTP-01). 443 only (80 = 308 redirect + ACME).
  If port 80 must be fully closed later -> switch to DNS-01 (Route53) and drop the :80 listener.
- Brand tokens (light): green #0a7d5a, green-dark #086a4d, navy #0b1f2a, amber #f5a623, bg #f4f8f6,
  line #e3ece8, font Inter. Operator console body stays dark (ops density); its LOGIN is light-branded.
- Auth remains demo-grade (SHA-256 + shared SESSION_SECRET env). Keycloak/MFA still the planned upgrade.
- Image build: golang:1.24-alpine (aws-sdk-go-v2 needs >=1.24); tests run in build (go vet + go test).
- Deploy method this session: kubectl set image (fast). Spinnaker gated pipeline still available for
  formal promotion (spinnaker/pipelines/insucar-deploy.json). Image tags used: auth, domains, landing,
  brandlogin, toprightauth, casecards (current = casecards).
