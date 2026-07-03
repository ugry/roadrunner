# Insucar — Project Milestones (what was done, what failed, how it was resolved)

Account: AWS 326804802908 (eu-west-1) · Repo: https://github.com/ugry/insucar (private)
Last updated: 2026-07-03

## Timeline / done
1. Tooling & repo
   - Installed GitHub CLI to ~/.local (no sudo), authenticated as `ugry`, set git identity.
   - Created private repo `ugry/insucar`.
2. Research & design
   - Europcar recon (Nuxt/Contentful/Akamai/AWS/Dynatrace/legacy Dotcar) -> observations/europcar-analysis.md.
   - Competitor/provider API research via exa.ai (AXA Roadside Missioning, Towpal, Agero/Swoop, HONK,
     Booking.com, CrashBay, ARC Europe, Europ Assistance) + IVR/Amazon Connect + CTI screen-pop.
   - Authored agentic build prompt and iterated: v1 -> v2 gap-hardened -> Spinnaker mandated ->
     no-mocks + quorum HA -> end-user registration + separate hidden operator app.
   - Multi-stakeholder gap analysis + prototype-readiness checklist (observations/).
3. Data layer
   - db/schema.sql (registration, identity, policies, vehicles, cases, PostGIS location, providers/
     connectors, missions, notifications, consent, immutable SHA-256 hash-chained audit ledger,
     prod-access grants) + db/seed.sql. VALIDATED against a real postgis/postgis:15-3.4 container:
     screen-pop lookup works, ledger chains correctly, ledger UPDATE/DELETE rejected.
4. Architecture diagram
   - prompt/insucar-architecture.mmd -> vector SVG + A2 (420x594mm) + A3 (297x420mm) PDFs.
5. First prototype (real, no mocks)
   - Go API (pgx) serving end-user app + hidden operator console (/ops-console-7f3a9c) + REST APIs,
     PostgreSQL/PostGIS via docker-compose. Tested with curl (register/lookup/case/dispatch) and
     headless Chrome (puppeteer-core UI flow) — all passed; hidden path returns 404 to guessers.
6. AWS deploy of prototype
   - EC2 (t3.small) running the stack; verified APIs + headless UI on AWS; Route53 records
     app/api/demo.unysolar.com -> instance.
7. CI/CD on EKS (as requested: EKS + Spinnaker Operator + Jenkins JCasC)
   - EKS cluster `insucar` (k8s 1.30, 2x t3.xlarge).
   - Jenkins via Helm + Config-as-Code: admin user, plugins, seeded `insucar-ci` pipeline job.
   - Spinnaker via spinnaker-operator + SpinnakerService v1.36.1: 9 services OK, S3 persistence,
     Jenkins (igor) integration verified.

## Failures encountered & how resolved
- gh CLI not installed, no sudo/password -> installed prebuilt binary to ~/.local/bin.
- AWS key provided was ROOT + exposed in chat -> flagged; used per explicit user instruction (demo).
- Prototype EC2 #1 failed: user-data `git clone` of the PRIVATE repo -> "could not read Username".
  Resolved by shipping code via a time-limited PRESIGNED S3 URL (repo stays private).
- Jenkins Helm install error: additionalPlugins duplicated chart defaults (configuration-as-code,
  git, workflow-aggregator) -> removed duplicates.
- Jenkins Helm error: `controller.adminUser` renamed -> switched to `controller.admin.username/password`.
- Jenkins pod stuck Pending: EKS 1.30 had NO EBS CSI driver / no default StorageClass -> PVC unbound.
  Resolved: created IRSA role, installed `aws-ebs-csi-driver` addon, set `gp2` as default SC.
- Spinnaker is heavy (many services) -> used spinnaker-operator; all pods reached Running/OK in ~4-10 min.
- Local sandbox DNS resolver (10.72.106.36) intermittently SERVFAIL'd -> kubectl/aws couldn't resolve
  endpoints. This was a SANDBOX issue, not AWS. Confirmed 8.8.8.8 resolved fine; resolver recovered.
  Durable fix if it recurs: point resolver at 8.8.8.8 / add endpoint to hosts (needs root), or wait out.

## Live status (verified)
- EKS nodes: 2/2 Ready. Spinnaker: 9/9 Running (Gate /health UP, Deck 200, igor -> insucar-jenkins).
  Jenkins: 2/2, login 200, job insucar-ci seeded. Prototype: app.unysolar.com:8080 serving.

## What is MISSING / next steps
- Spinnaker Kubernetes deploy target: `SpinnakerService.providers.kubernetes` is currently DISABLED
  (bring-up simplification). Add a SpinnakerAccount (in-cluster SA) so Spinnaker can deploy to EKS.
- No full end-to-end pipeline RUN yet: git push -> Jenkins insucar-ci -> build/push image to ECR ->
  webhook -> Spinnaker deploy. ECR repo not yet created; Spinnaker webhook/pipeline JSON not yet applied.
- Prototype runs on a standalone EC2, NOT yet deployed onto EKS via Spinnaker.
- Not yet built/deployed (per full design): Amazon Connect telephony + Lex, Keycloak SSO, Rust
  inner-core vault, Pinpoint SMS, TLS/HTTPS (ACM+ALB), quorum HA Postgres (Patroni), multi-cloud.
- Auth/TLS not configured on Jenkins/Spinnaker (open via LB) — fine for demo, harden before real use.

## CI/CD end-to-end proven (2026-07-03)
- Created ECR repo insucar-api; built+pushed image :1.
- Fixed Spinnaker k8s account: clouddriver crash on 'raw-resources-endpoint-config' bind bug
  (spinnaker/spinnaker#6840, found via exa.ai) -> added empty kindExpressions/omitKindExpressions.
  Also injected a cluster-admin SA kubeconfig via operator files -> account 'insucar-eks' registered.
- Deployed Postgres (schema+seed) to ns 'insucar' on EKS.
- Spinnaker application 'insucar' + pipeline 'deploy-insucar-api' (deployManifest) created and RUN:
  status SUCCEEDED. App deployed to EKS (2/2 pods), LoadBalancer serving.
- App on EKS (deployed by Spinnaker): http://af9269372141a4fdba7953b3679d6189-59590199.eu-west-1.elb.amazonaws.com/  (healthz OK; /api/lookup returns real data)

## Session 2026-07-03 (evening) — GUIs, domains, TLS, brand
Done:
- Auth prototype: app-level sessions (HMAC cookie); users log in by email, agents by agent_id.
  Seeded real-like users (customers + agents OP-1001/SUP-2001/PO-3001). Protected /api/user/* and
  /api/agent/*; verified via API + headless (login as user AND agent).
- Terraform IaC added (terraform/): VPC 3AZ + EKS + nodegroup autoscale + IRSA + ECR + S3.
- Open Design GUIs generated:
  * Operator console (design system "mission-control", dark) -> design/operator-console.html.
  * Marketing landing (design system "stripe", light, golden-ratio/Fibonacci) -> design/landing.html.
- Wired GUIs to live backend and split by HOST:
  * unysolar.com + www  -> marketing landing ("/"), functional user app at "/app".
  * op.unysolar.com      -> operator console (login -> live queue -> screen-pop -> dispatch).
  Backend handleRoot routes by Host (op.* vs apex).
- HTTPS/443 via Let's Encrypt: ingress-nginx + cert-manager (ClusterIssuer letsencrypt-prod, HTTP-01),
  cert insucar-tls covers unysolar.com/www/app/op. Port 80 -> 308 redirect. insucar-api svc -> ClusterIP;
  Route53 alias records point all hosts to the ingress-nginx ELB.
- Fixed www.unysolar.com (was an OLD record aliasing to the separate "Unysol" ALB in eu-central-1) ->
  repointed to Insucar ingress + added www to TLS SAN.
- Niche logo (golden ratio): shield + perspective road + amber arriving pin + Insu(navy)car(green)
  wordmark. design/insucar-logo.svg + insucar-mark.svg.
- Brand consistency: operator LOGIN restyled to match end-user brand (light green/navy/amber + shield);
  end-user /app restyled to same light brand.
- UX fixes: Log in/Log out moved to top-right bar (end-user); customer "My cases" changed from an
  overflowing table to a responsive wrapping card layout (readable on desktop + mobile).

Failures/fixes this session:
- Jenkinsfile: timestamps() option needed Timestamper plugin -> removed. curlimages/curl "process
  never started" -> switched webhook step to alpine+wget. Kaniko cache repo missing -> dropped --cache.
- go get needed Go >=1.24 for aws-sdk-go-v2 -> bumped build image to golang:1.24-alpine.
- Transient docker build go-get network hiccup -> retried (cache made it pass).

Live now: image tag insucar-api:casecards (deployed to ns insucar). HTTPS on all hosts.
