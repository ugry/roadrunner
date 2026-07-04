data "aws_availability_zones" "available" {
  state = "available"
}

locals {
  # 3 AZs so stateful quorum (2 data + witness) is possible — HA with node autoscaling.
  azs = slice(data.aws_availability_zones.available.names, 0, 3)
}

########################################
# VPC
########################################
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.8"

  name = "${var.cluster_name}-${var.environment}"
  cidr = var.vpc_cidr
  azs  = local.azs

  private_subnets = [for i, az in local.azs : cidrsubnet(var.vpc_cidr, 4, i)]
  public_subnets  = [for i, az in local.azs : cidrsubnet(var.vpc_cidr, 4, i + 8)]

  enable_nat_gateway   = true
  single_nat_gateway   = var.single_nat_gateway # dev=true (shared); prod=false (one per AZ)
  enable_dns_hostnames = true

  # Tags required for EKS + Cluster Autoscaler / load balancers
  public_subnet_tags  = { "kubernetes.io/role/elb" = "1" }
  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"                                = "1"
    "kubernetes.io/cluster/${var.cluster_name}"                      = "shared"
  }
}

########################################
# EKS cluster + managed nodegroup (HA + autoscaling)
########################################
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.24"

  cluster_name    = var.cluster_name
  cluster_version = var.kubernetes_version

  cluster_endpoint_public_access = true

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  enable_irsa = true

  # Control plane is AWS-managed (etcd, apiserver, scheduler, controller-manager).
  cluster_addons = {
    coredns                = {}
    kube-proxy             = {}
    vpc-cni                = {}
    aws-ebs-csi-driver     = {} # provides default StorageClass provisioning
    metrics-server         = {} # required for HPA
  }

  eks_managed_node_groups = {
    ng-standard = {
      instance_types = [var.node_instance_type]
      min_size       = var.node_min
      desired_size   = var.node_desired
      max_size       = var.node_max
      # tags for Cluster Autoscaler auto-discovery
      tags = {
        "k8s.io/cluster-autoscaler/enabled"             = "true"
        "k8s.io/cluster-autoscaler/${var.cluster_name}" = "owned"
      }
    }
  }

  # Enable control-plane logs to CloudWatch (health/audit visibility)
  cluster_enabled_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]
}

########################################
# IRSA roles: EBS CSI, Cluster Autoscaler
########################################
module "irsa_ebs_csi" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  role_name             = "${var.cluster_name}-ebs-csi"
  attach_ebs_csi_policy = true
  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:ebs-csi-controller-sa"]
    }
  }
}

module "irsa_cluster_autoscaler" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-role-for-service-accounts-eks"
  version = "~> 5.44"

  role_name                        = "${var.cluster_name}-cluster-autoscaler"
  attach_cluster_autoscaler_policy = true
  cluster_autoscaler_cluster_names = [var.cluster_name]
  oidc_providers = {
    main = {
      provider_arn               = module.eks.oidc_provider_arn
      namespace_service_accounts = ["kube-system:cluster-autoscaler"]
    }
  }
}

# App workload permissions (SNS for SMS). In prod prefer per-service IRSA over node role.
resource "aws_iam_policy" "app_sns" {
  name   = "${var.cluster_name}-${var.environment}-app-sns"
  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{ Effect = "Allow", Action = ["sns:Publish"], Resource = "*" }]
  })
}
