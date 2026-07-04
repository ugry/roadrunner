# Insucar — Rollout Runbook (fully AWS-managed)

End-to-end, ordered procedure to stand up (or rebuild) the platform on the
managed stack: EKS + RDS Multi-AZ + ElastiCache + EventBridge/SNS/SQS + Cognito
+ AMP/Managed Grafana, delivered via Jenkins → Spinnaker.

> Verified: `terraform plan -var environment=dev` = **146 to add, 0 change, 0
> destroy** (no errors). Nothing here has been applied yet — this is the
> apply-time sequence. Do it in **dev** first, then repeat for **uat** and
> **prod** (each ideally a separate AWS account).

Legend: `$ENV` ∈ {dev, uat, prod}. Commands assume `region=eu-west-1`.

---

## 0. Prerequisites (once)
- Scoped IAM admin (NOT root keys). Rotate root first — see `SECURITY-ROTATION.md`.
- Tools: `terraform>=1.6`, `awscli v2`, `kubectl`, `helm`, `jq`, `docker`.
- DNS: `unysolar.com` hosted zone (exists: `Z06143773JJ0DPRILPDA0`).
- Decide sizing per env in `terraform/variables.tf` (`db_instance_class`,
  `redis_node_type`, `node_*`, `db_multi_az`, `single_nat_gateway`).

---

## 1. Bootstrap Terraform remote state (once per account)
The S3 backend + lock table must exist before `terraform init`:
```
aws s3api create-bucket --bucket insucar-tfstate-326804802908 \
  --region eu-west-1 --create-bucket-configuration LocationConstraint=eu-west-1
aws s3api put-bucket-versioning --bucket insucar-tfstate-326804802908 \
  --versioning-configuration Status=Enabled
aws dynamodb create-table --table-name insucar-tf-lock \
  --attribute-definitions AttributeName=LockID,AttributeType=S \
  --key-schema AttributeName=LockID,KeyType=HASH --billing-mode PAY_PER_REQUEST \
  --region eu-west-1
```

## 2. Provision infrastructure (Terraform)
```
cd terraform
terraform init
terraform workspace new $ENV        # or: terraform workspace select $ENV
terraform plan  -var environment=$ENV        # prod: add -var single_nat_gateway=false
terraform apply -var environment=$ENV
```
Creates VPC/EKS/RDS/ElastiCache/EventBridge/SNS/SQS/Cognito/IAM/AMP/Grafana/ECR.

> Prereqs the plan can't create for you: **IAM Identity Center enabled** (for
> Managed Grafana `AWS_SSO`), and adequate **service quotas** (EIP, RDS, etc).

## 3. Capture outputs
```
$(terraform output -raw configure_kubectl)      # kubeconfig for the new cluster
terraform output                                 # note the values below
```
Key outputs: `app_irsa_role_arn`, `spinnaker_irsa_role_arn`, `ci_irsa_role_arn`,
`prometheus_irsa_role_arn`, `rds_endpoint`, `rds_master_secret_arn`,
`redis_primary_endpoint`, `redis_auth_secret_arn`, `event_bus_name`,
`sqs_queue_urls`, `cognito_staff_pool_id`, `cognito_staff_issuer`,
`cognito_staff_domain`, `cognito_spinnaker_client_id`, `amp_remote_write_url`,
`grafana_workspace_endpoint`, `ecr_repository_url`.

## 4. Annotate ServiceAccounts for IRSA (no static keys)
```
kubectl create namespace insucar 2>/dev/null || true
kubectl -n insucar   annotate sa insucar-api      eks.amazonaws.com/role-arn=$(terraform output -raw app_irsa_role_arn) --overwrite
kubectl -n spinnaker annotate sa spin-front50     eks.amazonaws.com/role-arn=$(terraform output -raw spinnaker_irsa_role_arn) --overwrite
kubectl -n spinnaker annotate sa spin-clouddriver eks.amazonaws.com/role-arn=$(terraform output -raw spinnaker_irsa_role_arn) --overwrite
kubectl -n jenkins   annotate sa jenkins          eks.amazonaws.com/role-arn=$(terraform output -raw ci_irsa_role_arn) --overwrite
```
(The `insucar-api` SA is defined in `k8s/insucar-api.yaml`; apply it in step 7,
or create the SA now if you annotate first.)

## 5. Database: schema + PostGIS into RDS
Fetch the RDS master password from Secrets Manager and load the schema:
```
RDS=$(terraform output -raw rds_endpoint)
PW=$(aws secretsmanager get-secret-value --secret-id $(terraform output -raw rds_master_secret_arn) \
      --query SecretString --output text | jq -r .password)
export PGPASSWORD="$PW"
psql "host=$RDS user=insucar_admin dbname=insucar sslmode=require" -c "CREATE EXTENSION IF NOT EXISTS postgis;"
psql "host=$RDS user=insucar_admin dbname=insucar sslmode=require" \
  -f db/schema.sql -f db/seed.sql -f db/schema-v3-additions.sql \
  -f db/schema-v4-auth.sql -f db/schema-v5-cognito.sql -f db/seed-users.sql
```

## 6. Kubernetes config + secrets (from Terraform outputs)
```
# Non-secret runtime config
REDIS=$(terraform output -raw redis_primary_endpoint)
REDIS_AUTH=$(aws secretsmanager get-secret-value --secret-id $(terraform output -raw redis_auth_secret_arn) --query SecretString --output text)

kubectl -n insucar create configmap insucar-config \
  --from-literal=AWS_REGION=eu-west-1 \
  --from-literal=EVENT_BUS_NAME=$(terraform output -raw event_bus_name) \
  --from-literal=COGNITO_ISSUER=$(terraform output -raw cognito_staff_issuer) \
  --from-literal=COGNITO_CLIENT_IDS=$(terraform output -raw cognito_spinnaker_client_id) \
  --from-literal=PROVIDER_API_URL=http://provider-axa.insucar.svc.cluster.local:5678 \
  --from-literal=STATUS_LINK_BASE=https://app.unysolar.com/status \
  --from-literal=DISPATCH_QUEUE_URL=$(terraform output -json sqs_queue_urls | jq -r .dispatch) \
  --from-literal=NOTIFICATION_QUEUE_URL=$(terraform output -json sqs_queue_urls | jq -r .notification) \
  --dry-run=client -o yaml | kubectl apply -f -

# Secrets (DB + Redis)
kubectl -n insucar create secret generic insucar-app \
  --from-literal=DATABASE_URL="postgres://insucar_admin:${PW}@${RDS}:5432/insucar?sslmode=require" \
  --from-literal=REDIS_URL="rediss://:${REDIS_AUTH}@${REDIS}:6379" \
  --dry-run=client -o yaml | kubectl apply -f -
```
(Repeat the ConfigMap/Secret in `insucar-dev/uat/prod` — the namespaces the
Spinnaker pipeline deploys into.)

## 7. Cognito users + groups
Groups are created by Terraform. Seed test users and assign groups:
```
POOL=$(terraform output -raw cognito_staff_pool_id)
aws cognito-idp admin-create-user --user-pool-id $POOL --username ops1@insucar.demo
aws cognito-idp admin-add-user-to-group --user-pool-id $POOL --username ops1@insucar.demo --group-name operator
# product-owner (Spinnaker prod approver):
aws cognito-idp admin-add-user-to-group --user-pool-id $POOL --username po@insucar.demo --group-name insucar-product-owners
```

## 8. Spinnaker + Jenkins auth (fill placeholders)
- Edit `spinnaker/spinnakerservice.yml`: set `clientId` = `cognito_spinnaker_client_id`
  and the `*.auth.<region>.amazoncognito.com` URLs = `cognito_staff_domain`.
- Create the OIDC client secret (Cognito app-client secret):
  ```
  kubectl -n spinnaker create secret generic spin-secrets \
    --from-literal=oidc-client-secret=<COGNITO_APP_CLIENT_SECRET>
  ```
- Apply the SpinnakerService + pipeline; confirm the manualJudgment stages are
  role-gated (`insucar-releasers` / `insucar-product-owners`).

## 9. Observability agent
```
kubectl create namespace monitoring 2>/dev/null || true
# fill the two REPLACE_ placeholders in the values file from outputs:
#   prometheus_irsa_role_arn, amp_remote_write_url
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm -n monitoring install prometheus prometheus-community/prometheus \
  -f k8s/observability/prometheus-agent-values.yaml
```
Add the AMP workspace as a data source in Managed Grafana (`grafana_workspace_endpoint`).

## 10. Build + deploy the app (CI/CD)
```
git push        # -> Jenkins insucar-ci: test/scan/build insucar-api + insucar-worker
                #    -> pushes to ECR -> webhook triggers Spinnaker
```
Spinnaker `deploy-insucar-api`:
`Deploy DEV → Smoke DEV → [judge: releasers/PO] → Deploy UAT → Smoke UAT →
[judge: product owner] → Deploy PROD → Smoke PROD → (auto-Rollback if unhealthy) → Verify`.
Each Deploy stage rolls out `insucar-api` (+Service) **and** `insucar-worker`.

## 11. Verify
```
kubectl -n insucar get deploy,po,sa,cm,secret
curl -fsS https://op.unysolar.com/healthz               # {"status":"ok"} (db + redis)
# screen-pop uses ElastiCache cache; dispatch emits EventBridge events consumed by the worker:
kubectl -n insucar logs deploy/insucar-worker | grep handled
```
- App healthy (DB + Redis reachable), Cognito login works, worker logs handled events.
- Metrics landing in AMP (query in Managed Grafana).

## 12. Promote to UAT / PROD
Re-run steps 2–11 with `$ENV=uat` then `$ENV=prod` (separate accounts). Promotion
of a build is gated in the Spinnaker pipeline (product-owner judgment for prod).

---

## Rollback
- App: the pipeline auto-rolls-back prod if `Smoke test PROD` fails
  (`undoRolloutManifest`), or manually: `kubectl -n insucar rollout undo deploy/insucar-api`.
- Infra: `terraform plan` to inspect, revert the change in Git, re-apply. RDS has
  PITR + 14-day backups; Redis has snapshots.

## Post-rollout security (do not skip)
Work through `SECURITY-ROTATION.md`: delete root access keys, rotate the GitHub
PAT + Jenkins password, lock the Jenkins/Spinnaker LBs to admin CIDRs, confirm
no pod carries static AWS keys (`kubectl exec ... -- env | grep AWS_` → only the
IRSA token). Stand up the 3-case IAM groups (`terraform/iam-groups.tf`).

## Teardown (stop cost)
```
# app layer
helm -n monitoring uninstall prometheus
helm -n jenkins uninstall jenkins
kubectl delete -n spinnaker spinnakerservice spinnaker
# managed infra (dev/uat; prod has deletion_protection on RDS — disable first)
cd terraform && terraform destroy -var environment=$ENV
```

## Cross-references
- `terraform/README.md` — module layout + bootstrap
- `SECURITY-ROTATION.md` — credential rotation + LB/TLS hardening
- `spinnaker/PIPELINE-NOTES.md` — deploy strategy, red/black + Kayenta follow-ups
- `CONTINUE-HERE.md` — current live state + handoff
