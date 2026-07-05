# Insucar — Infrastructure Todo: Managed Services Migration

**Status:** ARCHITECTURAL DEBT — the architecture spec, Terraform code, and rollout runbook all define managed AWS services. None are provisioned. The platform runs on self-managed pods.

**Decision date:** 2026-07-03 (build-notes.md) — "stateful auto-failover is delegated to AWS-managed Multi-AZ services rather than self-managed quorums"

**Why it was not applied:** Dev rollout was validated (139 resources, all working) on 2026-07-04, then `terraform destroy` was run to stop $0.60/hr cost on a validation stack. The plan was to re-apply to a dedicated AWS account. That re-apply never happened.

---

## WHAT TERRAFORM DEFINES (NOT PROVISIONED)

### Amazon RDS for PostgreSQL
**File:** `terraform/rds.tf`
```hcl
resource "aws_db_instance" "postgres" {
  engine         = "postgres"        # Aurora PostgreSQL option available
  engine_version = "16.9"
  instance_class = var.db_instance_class  # default: db.t3.medium
  multi_az       = var.db_multi_az       # default: true (synchronous standby)
  storage_encrypted = true
  backup_retention_period = 14           # 14 days point-in-time recovery
  enabled_cloudwatch_logs_exports = ["postgresql"]
  auto_minor_version_upgrade = true
  deletion_protection = true             # prod only
  manage_master_user_password = true     # auto-rotated, stored in Secrets Manager
}
```

**What we have instead:**
```yaml
# k8s/postgres.yaml — single Deployment, 1 replica, no backup, no encryption
image: postgis/postgis:15-3.4
DATABASE_URL=postgres://postgres:test@postgres:5432/insucar?sslmode=disable
```
- No automated backups (pg_dump cron job attempted today but hit API validation error)
- No point-in-time recovery
- No read replicas
- No encryption at rest (EBS gp2 volume, default key)
- Password `test` hardcoded in deployment env vars
- Single AZ — node failure = database down
- Version 15.8 (not 16.x as Terraform defines)

### Amazon ElastiCache for Redis
**File:** `terraform/elasticache.tf`
```hcl
resource "aws_elasticache_replication_group" "redis" {
  engine         = "redis"
  node_type      = var.redis_node_type   # default: cache.t4g.small
  num_cache_clusters = 2                  # Multi-AZ
  automatic_failover_enabled = true
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token_enabled = true               # stored in Secrets Manager
}
```

**What we have instead:** NOTHING. Redis is not deployed at all. App startup logs show `redis=false`. Features disabled:
- Screen-pop ANI cache (DB hit on every lookup instead of 60s cache)
- Session store (cookies only, no Redis-backed sessions)
- Rate limiting (can't implement without Redis)

### Separate AWS Accounts Per Environment
**Terraform design:** `workspace new dev/uat/prod` — each pointing to a different AWS account
**Current state:** All in one account (326804802908). Namespaces `insucar-dev`, `insucar-uat`, `insucar-prod` share the same EKS cluster, same VPC, same node pool.

### IAM Roles (IRSA)
**Terraform:** `iam.tf` — 4 IRSA roles
- `insucar-<env>-app` — SNS, EventBridge, SQS
- `insucar-<env>-spinnaker` — S3
- `insucar-<env>-ci` — ECR push
- `insucar-<env>-autoscaler` — cluster autoscaler

**Current state:** Node instance role has `AmazonSNSFullAccess` + `AmazonEC2ContainerRegistryPowerUser` — every pod on the node inherits these. No pod-level IAM.

---

## WHY THIS MATTERS

### Current Risk: The PostgreSQL Pod
1. **Pod deleted** → data survives on PVC (if PVC not deleted)
2. **PVC deleted** → ALL DATA GONE. No backup exists.
3. **Node failure** → pod restarts on another node, mounts same PVC. OK if PVC is in same AZ.
4. **AZ failure** → pod can't restart because PVC is in the failed AZ. Database is down until AZ recovers.
5. **Human error** → `kubectl delete pvc data-postgres-0` = instant data loss. No `undo`.

### What RDS Multi-AZ Gives You
1. Synchronous standby in different AZ
2. Automatic failover in <60 seconds
3. Automated daily snapshots (14-day retention)
4. Point-in-time recovery (restore to any second within retention period)
5. Encryption at rest with KMS
6. Auto-rotated master password in Secrets Manager
7. Force SSL enforcement
8. Minor version auto-upgrades

### Cost Comparison
| Service | Current | RDS Multi-AZ |
|---|---|---|
| PostgreSQL | $0 (shared pod on t3.xlarge) | ~$60/mo (db.t3.medium, single-AZ dev) |
| Redis | $0 (not deployed) | ~$28/mo (cache.t4g.small, 2 nodes) |
| Multi-AZ | N/A | Doubles RDS cost in production |
| **Total** | **$0** | **~$88-176/mo** |

---

## MIGRATION PLAN

### Step 1: Provision managed services (1 hour)
```bash
cd terraform
terraform init
terraform workspace new dev    # or select if exists
terraform plan  -var environment=dev -var db_multi_az=false   # single-AZ for dev
terraform apply -var environment=dev -var db_multi_az=false
```

This creates:
- RDS PostgreSQL 16.9 (single-AZ for dev, Multi-AZ for prod)
- ElastiCache Redis (Multi-AZ)
- Proper IAM roles (IRSA)
- Secrets Manager entries for DB password and Redis auth

### Step 2: Extract credentials (5 minutes)
```bash
RDS_ENDPOINT=$(terraform output -raw rds_endpoint)
RDS_SECRET_ARN=$(terraform output -raw rds_master_secret_arn)
DB_PASSWORD=$(aws secretsmanager get-secret-value --secret-id $RDS_SECRET_ARN --query SecretString --output text | jq -r .password)
REDIS_ENDPOINT=$(terraform output -raw redis_primary_endpoint)
REDIS_AUTH=$(aws secretsmanager get-secret-value --secret-id $(terraform output -raw redis_auth_secret_arn) --query SecretString --output text)
```

### Step 3: Load schema into RDS (10 minutes)
```bash
psql "host=$RDS_ENDPOINT user=insucar_admin password=$DB_PASSWORD dbname=insucar sslmode=require" \
  -c "CREATE EXTENSION IF NOT EXISTS postgis;" \
  -f db/schema.sql -f db/seed.sql -f db/schema-v3-additions.sql \
  -f db/schema-v4-auth.sql -f db/schema-v5-cognito.sql -f db/seed-users.sql
```

### Step 4: Export live data from pod → import to RDS (15 minutes)
```bash
# From the running pod
kubectl -n insucar exec deploy/postgres -- pg_dump -U postgres -d insucar --no-owner --no-acl | \
  psql "host=$RDS_ENDPOINT user=insucar_admin password=$DB_PASSWORD dbname=insucar sslmode=require"
```

### Step 5: Update app deployment (5 minutes)
```bash
kubectl -n insucar set env deployment/insucar-api \
  DATABASE_URL="postgres://insucar_admin:${DB_PASSWORD}@${RDS_ENDPOINT}:5432/insucar?sslmode=require"
kubectl -n insucar set env deployment/insucar-api \
  REDIS_URL="rediss://:${REDIS_AUTH}@${REDIS_ENDPOINT}:6379"
```

### Step 6: Annotate ServiceAccounts for IRSA (1 minute)
```bash
kubectl -n insucar annotate sa insucar-api eks.amazonaws.com/role-arn=$(terraform output -raw app_irsa_role_arn) --overwrite
```

### Step 7: Verify (5 minutes)
```bash
kubectl logs deploy/insucar-api | grep "connected to database"
curl https://op.unysolar.com/healthz  # should show {"status":"ok"}
```

### Step 8: Remove in-cluster postgres (after confirming RDS works)
```bash
kubectl -n insucar delete deploy postgres
kubectl -n insucar delete svc postgres
# Keep PVC for 7 days as safety net, then delete
```

---

## IMMEDIATE CHECKLIST

| # | Task | Time | Blocks What |
|---|---|---|---|
| 1 | Apply Terraform dev workspace (RDS + ElastiCache) | 1h | All managed services |
| 2 | Create DEV AWS account (or use existing 326804802908 for dev) | 30m | Account separation |
| 3 | Load schema + data into RDS | 15m | App migration |
| 4 | Update app deployment with RDS/Redis env vars | 5m | App migration |
| 5 | Verify health endpoints | 5m | Go-live |
| 6 | Remove in-cluster postgres pod | 1m | Cleanup |
| 7 | Repeat for UAT (separate account) | 2h | Pre-prod |
| 8 | Repeat for PROD (separate account, Multi-AZ=true) | 2h | Production |

**Total time to managed services:** ~7 hours across all environments

---

*This document supersedes the earlier pg_dump CronJob approach. The CronJob was a temporary band-aid. The architecture spec and Terraform code already define the correct solution — it needs to be applied.*
