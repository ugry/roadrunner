########################################
# Per-workload IRSA roles (least privilege)
#
# Replaces two prototype shortcuts:
#   1. granting SNS on the shared node instance role, and
#   2. inlining ROOT AWS keys into the SpinnakerService S3 config.
# Each workload now assumes a scoped role via its ServiceAccount (IRSA),
# so no long-lived keys live in the cluster. After apply, annotate the
# ServiceAccounts with the role ARNs emitted in outputs.tf.
########################################

# --- App (insucar-api): runtime permissions for managed services ---------
# aws_iam_policy.app_sns is declared in main.tf but was never attached.
# The app role now also covers EventBridge/SNS/SQS (messaging) and reading
# the RDS/Redis credentials from Secrets Manager. Least-privilege, scoped to
# this environment's resources.
data "aws_iam_policy_document" "app_runtime" {
  statement {
    sid       = "PublishEvents"
    actions   = ["events:PutEvents"]
    resources = [aws_cloudwatch_event_bus.events.arn]
  }
  statement {
    sid       = "SnsFanout"
    actions   = ["sns:Publish"]
    resources = [aws_sns_topic.events.arn]
  }
  statement {
    sid       = "SqsWork"
    actions   = ["sqs:SendMessage", "sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"]
    resources = [for q in aws_sqs_queue.work : q.arn]
  }
  statement {
    sid       = "ReadRuntimeSecrets"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [
      aws_db_instance.postgres.master_user_secret[0].secret_arn,
      aws_secretsmanager_secret.redis_auth.arn,
    ]
  }
}

resource "aws_iam_policy" "app_runtime" {
  name   = "${var.cluster_name}-${var.environment}-app-runtime"
  policy = data.aws_iam_policy_document.app_runtime.json
}

module "irsa_app" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  role_name = "${var.cluster_name}-${var.environment}-app"
  role_policy_arns = {
    sns     = aws_iam_policy.app_sns.arn     # SMS (Publish to phone numbers)
    runtime = aws_iam_policy.app_runtime.arn # EventBridge/SNS/SQS/Secrets
  }

  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["insucar:insucar-api"]
    }
  }
}

# --- Spinnaker Front50/Clouddriver: scoped S3 (replaces inlined root keys) -
data "aws_iam_policy_document" "spinnaker_s3" {
  statement {
    sid       = "SpinnakerFront50Bucket"
    actions   = ["s3:ListBucket", "s3:GetBucketLocation"]
    resources = [aws_s3_bucket.spinnaker.arn]
  }
  statement {
    sid       = "SpinnakerFront50Objects"
    actions   = ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"]
    resources = ["${aws_s3_bucket.spinnaker.arn}/*"]
  }
}

resource "aws_iam_policy" "spinnaker_s3" {
  name   = "${var.cluster_name}-${var.environment}-spinnaker-s3"
  policy = data.aws_iam_policy_document.spinnaker_s3.json
}

module "irsa_spinnaker" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  role_name        = "${var.cluster_name}-${var.environment}-spinnaker"
  role_policy_arns = { s3 = aws_iam_policy.spinnaker_s3.arn }

  oidc_providers = {
    main = {
      provider_arn = module.eks.oidc_provider_arn
      namespace_service_accounts = [
        "spinnaker:spin-front50",
        "spinnaker:spin-clouddriver",
      ]
    }
  }
}

# --- Jenkins/Kaniko: ECR push to the insucar-api repo ONLY ----------------
data "aws_iam_policy_document" "ci_ecr" {
  statement {
    sid       = "EcrAuth"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }
  statement {
    sid = "EcrPushPull"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
      "ecr:PutImage",
      "ecr:BatchGetImage",
      "ecr:GetDownloadUrlForLayer",
    ]
    resources = [aws_ecr_repository.insucar_api.arn, aws_ecr_repository.insucar_worker.arn]
  }
}

resource "aws_iam_policy" "ci_ecr" {
  name   = "${var.cluster_name}-${var.environment}-ci-ecr"
  policy = data.aws_iam_policy_document.ci_ecr.json
}

module "irsa_ci" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  role_name        = "${var.cluster_name}-${var.environment}-ci"
  role_policy_arns = { ecr = aws_iam_policy.ci_ecr.arn }

  oidc_providers = {
    main = {
      provider_arn = module.eks.oidc_provider_arn
      namespace_service_accounts = [
        "jenkins:jenkins",
      ]
    }
  }
}
