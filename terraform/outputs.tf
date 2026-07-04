output "cluster_name" {
  value = module.eks.cluster_name
}

output "cluster_endpoint" {
  value = module.eks.cluster_endpoint
}

output "configure_kubectl" {
  value = "aws eks update-kubeconfig --name ${module.eks.cluster_name} --region ${var.region}"
}

output "ecr_repository_url" {
  value = aws_ecr_repository.insucar_api.repository_url
}

output "ebs_csi_irsa_role_arn" {
  value = module.irsa_ebs_csi.iam_role_arn
}

output "cluster_autoscaler_irsa_role_arn" {
  value = module.irsa_cluster_autoscaler.iam_role_arn
}

output "app_irsa_role_arn" {
  description = "Annotate ServiceAccount insucar/insucar-api with this ARN (SNS SMS)."
  value       = module.irsa_app.iam_role_arn
}

output "spinnaker_irsa_role_arn" {
  description = "Annotate SAs spinnaker/spin-front50 + spin-clouddriver with this ARN (S3)."
  value       = module.irsa_spinnaker.iam_role_arn
}

output "ci_irsa_role_arn" {
  description = "Annotate ServiceAccount jenkins/jenkins with this ARN (ECR push)."
  value       = module.irsa_ci.iam_role_arn
}
