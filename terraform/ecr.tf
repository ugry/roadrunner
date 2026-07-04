resource "aws_ecr_repository" "insucar_api" {
  name                 = "insucar-${var.environment}-api"
  image_tag_mutability = "MUTABLE"
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "insucar_worker" {
  name                 = "insucar-${var.environment}-worker"
  image_tag_mutability = "MUTABLE"
  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_lifecycle_policy" "insucar_worker" {
  repository = aws_ecr_repository.insucar_worker.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "keep last 20 images"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 20 }
      action       = { type = "expire" }
    }]
  })
}

resource "aws_ecr_lifecycle_policy" "insucar_api" {
  repository = aws_ecr_repository.insucar_api.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "keep last 20 images"
      selection    = { tagStatus = "any", countType = "imageCountMoreThan", countNumber = 20 }
      action       = { type = "expire" }
    }]
  })
}

# S3 buckets used by the platform
resource "aws_s3_bucket" "spinnaker" {
  bucket = "insucar-${var.environment}-spinnaker-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket" "deploy" {
  bucket = "insucar-${var.environment}-deploy-${data.aws_caller_identity.current.account_id}"
}

data "aws_caller_identity" "current" {}
