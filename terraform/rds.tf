########################################
# Managed PostgreSQL (RDS) — Multi-AZ
#
# Replaces the single in-cluster `postgres` pod (a SPOF: one replica, one AZ,
# AZ-pinned EBS). RDS Multi-AZ gives an automatic synchronous standby in a
# second AZ with RPO≈0 and managed failover, backups, and PITR.
#
# The platform is fully AWS-managed (no multi-cloud goal), so RDS Multi-AZ is
# the default for data HA — it removes the single-pod SPOF with minimal ops
# (Aurora PostgreSQL is the drop-in upgrade for higher scale/faster failover).
#
# PostGIS: supported on RDS Postgres. After creation, connect and run:
#     CREATE EXTENSION IF NOT EXISTS postgis;
# then apply db/schema.sql + db/seed.sql + schema-v3/v4 as usual.
#
# Repoint the app: change the Spinnaker pipeline env DATABASE_URL from
#   postgres://postgres:test@postgres.insucar.svc.cluster.local:5432/insucar
# to a Secret-backed value using the RDS endpoint + the master user secret
# that RDS manages in Secrets Manager (see output rds_master_secret_arn):
#   kubectl -n <ns> create secret generic insucar-db \
#     --from-literal=DATABASE_URL="postgres://insucar_admin:<pw>@<endpoint>:5432/insucar?sslmode=require"
# and reference it via envFrom/secretKeyRef instead of the inline value.
########################################

resource "aws_db_subnet_group" "this" {
  name       = "insucar-${var.environment}"
  subnet_ids = module.vpc.private_subnets
}

resource "aws_security_group" "rds" {
  name        = "insucar-${var.environment}-rds"
  description = "Postgres access from EKS nodes only"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description     = "Postgres from EKS nodes"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_db_parameter_group" "this" {
  name   = "insucar-${var.environment}-pg16"
  family = "postgres16"

  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }
}

resource "aws_db_instance" "postgres" {
  identifier     = "insucar-${var.environment}"
  engine         = "postgres"
  engine_version = "16.9"
  instance_class = var.db_instance_class

  allocated_storage     = 50
  max_allocated_storage = 200
  storage_type          = "gp3"
  storage_encrypted     = true

  db_name  = "insucar"
  username = "insucar_admin"
  # RDS creates + rotates the master password in Secrets Manager; nothing
  # sensitive is stored in Terraform state or this file.
  manage_master_user_password = true

  multi_az               = var.db_multi_az
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  parameter_group_name   = aws_db_parameter_group.this.name

  backup_retention_period    = 14
  backup_window              = "02:00-03:00"
  maintenance_window         = "sun:03:30-sun:04:30"
  copy_tags_to_snapshot      = true
  auto_minor_version_upgrade = true

  performance_insights_enabled = true

  # Protect prod; let dev/uat be torn down cleanly.
  deletion_protection       = var.environment == "prod"
  skip_final_snapshot       = var.environment != "prod"
  final_snapshot_identifier = "insucar-${var.environment}-final"

  # Never publicly reachable — private subnets + node SG only.
  publicly_accessible = false
}

output "rds_endpoint" {
  value = aws_db_instance.postgres.address
}

output "rds_port" {
  value = aws_db_instance.postgres.port
}

output "rds_master_secret_arn" {
  description = "Secrets Manager ARN holding the RDS master user credentials."
  value       = aws_db_instance.postgres.master_user_secret[0].secret_arn
}
