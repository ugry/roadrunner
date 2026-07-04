########################################
# Observability — Amazon Managed Service for Prometheus (AMP) + Amazon
# Managed Grafana (AMG). Completes the managed pivot away from self-hosted
# Prometheus/Loki/Grafana. EKS ships metrics to AMP via the ADOT/Prometheus
# add-on (remote_write to the workspace endpoint); Grafana reads from AMP.
########################################

resource "aws_prometheus_workspace" "this" {
  alias = "insucar-${var.environment}"
}

# Optional log group for app/JSON logs shipped via CloudWatch (auth/system/error).
resource "aws_cloudwatch_log_group" "app" {
  name              = "/insucar/${var.environment}/app"
  retention_in_days = 30
}

resource "aws_grafana_workspace" "this" {
  name                     = "insucar-${var.environment}"
  account_access_type      = "CURRENT_ACCOUNT"
  authentication_providers = ["AWS_SSO"]
  permission_type          = "SERVICE_MANAGED"
  data_sources             = ["PROMETHEUS", "CLOUDWATCH"]
  role_arn                 = aws_iam_role.grafana.arn
}

# Role Grafana assumes to read AMP + CloudWatch metrics.
data "aws_iam_policy_document" "grafana_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["grafana.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "grafana" {
  name               = "insucar-${var.environment}-grafana"
  assume_role_policy = data.aws_iam_policy_document.grafana_assume.json
}

data "aws_iam_policy_document" "grafana_read" {
  statement {
    sid = "PrometheusQuery"
    actions = [
      "aps:ListWorkspaces",
      "aps:DescribeWorkspace",
      "aps:QueryMetrics",
      "aps:GetLabels",
      "aps:GetSeries",
      "aps:GetMetricMetadata",
    ]
    resources = ["*"]
  }
  statement {
    sid       = "CloudWatchRead"
    actions   = ["cloudwatch:DescribeAlarms", "cloudwatch:GetMetricData", "cloudwatch:ListMetrics", "logs:GetLogEvents", "logs:FilterLogEvents", "logs:DescribeLogGroups"]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "grafana_read" {
  name   = "insucar-${var.environment}-grafana-read"
  role   = aws_iam_role.grafana.id
  policy = data.aws_iam_policy_document.grafana_read.json
}

output "amp_workspace_id" { value = aws_prometheus_workspace.this.id }
output "amp_remote_write_url" {
  description = "remote_write endpoint for the EKS Prometheus/ADOT agent."
  value       = "${aws_prometheus_workspace.this.prometheus_endpoint}api/v1/remote_write"
}
output "grafana_workspace_endpoint" { value = aws_grafana_workspace.this.endpoint }
