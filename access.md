# Insucar — Access & Credentials (SENSITIVE — do NOT commit/share)

> This file is git-ignored on purpose. It contains live secrets. Move these into a secrets
> manager (AWS Secrets Manager / Vault) and ROTATE everything below as soon as possible.
> The AWS keys are ROOT account keys — highest risk; replace with a scoped IAM user/role.

Last updated: 2026-07-03

---

## AWS account
- Account ID: 326804802908
- Default region: eu-west-1
- Access key ID: AKIAUYFYVRVODX2N6LHX
- Secret access key: +IPK//KWmEqlz7Jm2hUguJTDqx9VhlBA3GDlkQmw
- Type: ROOT account access keys  (⚠ rotate/delete; create an IAM user instead)
- CLI configured at: ~/.aws/credentials (profile: default)

## GitHub
- Repo: https://github.com/ugry/insucar  (private)
- User: ugry
- Personal Access Token (PAT): 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
- Stored in cluster as k8s secret: namespace `jenkins`, secret `github-pat`, key `token`
- gh CLI auth: logged in as `ugry` (token stored in system keyring)

---

## Prototype (single EC2 + Docker)
- EC2 instance: i-0628af42823122bce (t3.small, eu-west-1)
- Public IP: 108.129.149.127
- Security group: sg-08bfda3ebbaa34e92 (ingress 8080/80/22)
- End-user app:      http://app.unysolar.com:8080/   (also /login, /register)  |  http://108.129.149.127:8080/
- API health:        http://api.unysolar.com:8080/healthz
- Operator console:  http://app.unysolar.com:8080/ops-console-7f3a9c   (HIDDEN path — keep secret)
- App DB (inside container, not public): PostgreSQL  user `postgres` / pass `test` / db `insucar`
- Note: prototype apps have NO login yet (Keycloak not deployed here); operator surface is
  protected only by the obscure path + 404-on-miss.

## Deploy artifact bucket
- S3: s3://insucar-deploy-326804802908 (private; held the prototype tarball via presigned URL)

---

## EKS cluster (Jenkins + Spinnaker)
- Cluster: insucar (k8s 1.30), region eu-west-1
- Get kubeconfig: aws eks update-kubeconfig --name insucar --region eu-west-1
- Nodegroup: ng-standard (2x t3.xlarge)

### Jenkins  (namespace: jenkins)
- URL:  http://a69a0dc446e674657ac3fae06d8dd559-1651454478.eu-west-1.elb.amazonaws.com:8080
- Username: admin
- Password: InsucarAdmin!2026    (⚠ change)
- Seeded pipeline job: insucar-ci  (runs ci/Jenkinsfile from the repo)

### Spinnaker  (namespace: spinnaker; operator ns: spinnaker-operator)
- Deck (UI):  http://a4977860e39434f278d0b4dedbcd4bb5-449340997.eu-west-1.elb.amazonaws.com
- Gate (API): http://afac25beae62d4f0cab340b254e5e6f2-1288246793.eu-west-1.elb.amazonaws.com  (/health)
- Auth: NONE configured (open to anyone with the URL) — ⚠ add OIDC/fiat + restrict the LB before real use.
- Persistence: S3 s3://insucar-spinnaker-326804802908
- Jenkins integration (igor) master name: insucar-jenkins
- AWS creds for S3 are currently inlined in the in-cluster SpinnakerService (⚠ rotate; move to IRSA).

---

## DNS (Route 53)
- Hosted zone: unysolar.com  (Z06143773JJ0DPRILPDA0) — already delegated to AWS
- Records: app / api / demo .unysolar.com  ->  108.129.149.127 (A, TTL 60)

---

## Demo data (DB rows in the prototype — not login credentials)
- Customers (phone = ANI lookup key): Claire Martin +33600000001 / John Smith +447700900002 /
  Lukas Mueller +491600000003
- Staff rows: operator@insucar.demo / supervisor@insucar.demo / po@insucar.demo (Keycloak not wired
  in the prototype, so these are data only — no passwords yet).

---

## ROTATE-NOW checklist
- [ ] AWS root access keys (delete; create IAM user; update ~/.aws + SpinnakerService S3 config)
- [ ] GitHub PAT (regenerate; update `github-pat` k8s secret)
- [ ] Jenkins admin password
- [ ] Add auth (OIDC) + TLS + LB restrictions to Jenkins and Spinnaker

## EKS app (deployed by Spinnaker)
- insucar-api URL: http://af9269372141a4fdba7953b3679d6189-59590199.eu-west-1.elb.amazonaws.com/   (health: /healthz ; lookup: /api/lookup?phone=%2B33600000001 ; operator: /ops-console-7f3a9c)
- Namespace: insucar (Deployment insucar-api x2 + Postgres with schema+seed)
- ECR image: 326804802908.dkr.ecr.eu-west-1.amazonaws.com/insucar-api:1
- Spinnaker app/pipeline: insucar / deploy-insucar-api (deployManifest, account insucar-eks)

## Quick access — GUIs & endpoints (updated 2026-07-03)
- Jenkins UI:      http://a69a0dc446e674657ac3fae06d8dd559-1651454478.eu-west-1.elb.amazonaws.com:8080          (admin / InsucarAdmin!2026)
- Spinnaker Deck:  http://a4977860e39434f278d0b4dedbcd4bb5-449340997.eu-west-1.elb.amazonaws.com              (no login; app "insucar")
- Spinnaker Gate:  http://afac25beae62d4f0cab340b254e5e6f2-1288246793.eu-west-1.elb.amazonaws.com              (API only)
- App PROD (gated pipeline): http://ad4de17a313444704a74f62919bfabc7-1055718284.eu-west-1.elb.amazonaws.com/
- App (first Spinnaker deploy, ns insucar):  http://af9269372141a4fdba7953b3679d6189-59590199.eu-west-1.elb.amazonaws.com/
- Prototype EC2 app: http://app.unysolar.com:8080/  (hidden operator: /ops-console-7f3a9c)
- kubeconfig: aws eks update-kubeconfig --name insucar --region eu-west-1
- Spinnaker webhook (Jenkins->deploy): http://afac25beae62d4f0cab340b254e5e6f2-1288246793.eu-west-1.elb.amazonaws.com/webhooks/webhook/insucar-ci

## Jenkins & Spinnaker — full operational details (updated 2026-07-03)
### Jenkins
- Helm release: jenkins/jenkins 5.9.32 (Jenkins 2.555.3), namespace `jenkins`.
- UI: http://a69a0dc446e674657ac3fae06d8dd559-1651454478.eu-west-1.elb.amazonaws.com:8080
- Login: admin / InsucarAdmin!2026
- Retrieve creds from cluster:
  kubectl -n jenkins get secret jenkins -o jsonpath='{.data.jenkins-admin-user}'     | base64 -d
  kubectl -n jenkins get secret jenkins -o jsonpath='{.data.jenkins-admin-password}' | base64 -d
- Job: insucar-ci (Pipeline from SCM -> ci/Jenkinsfile). Git cred id: github-pat.
- Agents: Kubernetes plugin spawns Kaniko + alpine pods in ns `jenkins` for builds.
- Trigger a build via API:
  CRUMB=$(curl -s -c /tmp/cj --user admin:InsucarAdmin!2026 "$J/crumbIssuer/api/json" | jq -r .crumb)
  curl -b /tmp/cj --user admin:InsucarAdmin!2026 -H "Jenkins-Crumb: $CRUMB" -X POST "$J/job/insucar-ci/build"
- Port-forward (if LB down): kubectl -n jenkins port-forward svc/jenkins 8080:8080

### Spinnaker
- Installed by spinnaker-operator (ns `spinnaker-operator`); services in ns `spinnaker`; version 1.36.1.
- Deck (UI):  http://a4977860e39434f278d0b4dedbcd4bb5-449340997.eu-west-1.elb.amazonaws.com  (no auth; app "insucar")
- Gate (API): http://afac25beae62d4f0cab340b254e5e6f2-1288246793.eu-west-1.elb.amazonaws.com  (/health)
- Application: insucar · Pipeline: deploy-insucar-api · Pipeline id: 584af310-21f5-427d-ac3b-a0d5c065fc18
- Pipeline stages: Deploy DEV -> Promote to UAT? (manualJudgment) -> Deploy UAT ->
  Promote to PROD? (product owner, manualJudgment) -> Deploy PROD.
- Webhook trigger: POST http://<gate>/webhooks/webhook/insucar-ci
- Kubernetes deploy account: insucar-eks (kubeconfig injected via operator files;
  SA spin-deployer cluster-admin). Target namespaces: insucar, insucar-dev, insucar-uat, insucar-prod.
- Persistence (Front50): S3 s3://insucar-spinnaker-326804802908 (rootFolder front50).
- Jenkins integration (igor) master: insucar-jenkins (points to jenkins.jenkins.svc.cluster.local:8080).
- Trigger deploy manually via Gate:
  curl -s -X POST http://<gate>/pipelines/insucar/deploy-insucar-api -H 'Content-Type: application/json' -d '{"type":"manual"}'
- Approve a manual-judgment stage via Gate:
  curl -s -X PATCH http://<gate>/pipelines/<execId>/stages/<stageId> -H 'Content-Type: application/json' -d '{"judgmentStatus":"continue"}'
- Port-forward (if LB down): kubectl -n spinnaker port-forward svc/spin-deck 9000:9000 ; svc/spin-gate 8084:8084

### CI/CD supporting
- ECR repo: 326804802908.dkr.ecr.eu-west-1.amazonaws.com/insucar-api (tags :1, :2, :latest, build numbers)
- Cluster Autoscaler: helm autoscaler/cluster-autoscaler 9.58.0 (ns kube-system); nodegroup ng-standard min2/max5.
- HPA: insucar-api (cpu 60%, 2->6). Node role: eksctl-insucar-nodegroup-ng-standa-NodeInstanceRole-q21hJS3Im7Ph
  (has AmazonSNSFullAccess + AmazonEC2ContainerRegistryPowerUser).
- Provider stub (real HTTP): svc provider-axa.insucar.svc.cluster.local:5678

## Prototype login credentials (app-level auth, demo)
End-user app (login by email) at http://<app-lb>/login :
- claire.martin@example.fr / Claire#2026
- john.smith@example.co.uk / John#2026
- lukas.mueller@example.de / Lukas#2026
Operator console (login by agent ID) at http://<app-lb>/ops-console-7f3a9c :
- OP-1001  / Operator#2026   (operator: Amelie Durand)
- SUP-2001 / Supervisor#2026 (supervisor: Marc Petit)
- PO-3001  / Owner#2026      (product owner: Sophie Bernard)
Current app LB: (see `kubectl -n insucar get svc insucar-api`)

## Live domains (functional)
- Users:     https://unysolar.com/  (also app.unysolar.com) — landing + login/register + request assistance
- Operators: https://op.unysolar.com/ — Mission-Control operator console (agent login only)
- Backend routes by Host: op.* -> operator console; apex -> user landing (users only / operators only).
- Same seeded logins (users by email, agents by agent ID) apply.

## TLS / HTTPS (Let's Encrypt)
- ingress-nginx (ns ingress-nginx) LB terminates TLS on 443; cert-manager issues Let's Encrypt certs.
- ClusterIssuer: letsencrypt-prod (HTTP-01). Cert secret: insucar-tls (ns insucar), hosts
  unysolar.com / app.unysolar.com / op.unysolar.com. Auto-renews.
- Port 80 only 308-redirects to 443 (force-ssl-redirect). insucar-api svc is now ClusterIP
  (only the ingress LB is public). Ingress LB is the Route53 alias target for all 3 hosts.
