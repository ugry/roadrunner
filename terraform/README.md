# Insucar — Terraform (Infrastructure as Code)

Greenfield IaC for the Insucar platform infrastructure: VPC (3 AZ), EKS (managed control plane),
managed nodegroup with autoscaling, EBS CSI + Cluster Autoscaler IRSA, ECR, and S3 buckets.

> The live prototype cluster was originally bootstrapped with `eksctl`; this Terraform is the
> canonical, reproducible definition of the same infrastructure. Applying it creates a NEW cluster,
> so use a distinct `cluster_name`/account or `terraform import` the existing resources first.

## Layout
- `versions.tf`   provider + (optional) S3 remote state
- `variables.tf`  region, environment, cluster/node sizing (min2/desired2/max5)
- `main.tf`       VPC + EKS + nodegroup (HA, autoscaling tags) + addons + IRSA roles
- `ecr.tf`        ECR repo (scan-on-push, lifecycle) + S3 buckets
- `outputs.tf`    cluster endpoint, kubectl command, ECR URL, IRSA role ARNs

## Environments (3 tiers)
Design: dev / uat / prod each in a SEPARATE AWS account. Use one workspace/state per account:
```
terraform workspace new dev   && terraform apply -var environment=dev
terraform workspace new uat   && terraform apply -var environment=uat
terraform workspace new prod  && terraform apply -var environment=prod -var single_nat_gateway=false
```

## Usage
```
cd terraform
terraform init
terraform plan  -var environment=dev
terraform apply -var environment=dev
$(terraform output -raw configure_kubectl)
```

## What Terraform manages vs not
- Manages: VPC, EKS control plane (AWS-managed internally), nodegroup, addons, IRSA, ECR, S3.
- After apply, install the app layer with Helm/kubectl or Spinnaker: Jenkins, Spinnaker operator,
  Postgres, insucar-api, HPA (see `ci/`, `spinnaker/`, `k8s/`).
- Control-plane internals (etcd/apiserver/scheduler/controller-manager) are AWS-managed; control
  plane logs are enabled here and shipped to CloudWatch.
```
