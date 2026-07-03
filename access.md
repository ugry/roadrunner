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
