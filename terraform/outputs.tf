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
