terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
  # Remote state + locking. The bucket + DynamoDB lock table must be
  # bootstrapped ONCE before `terraform init` (they cannot live in the same
  # state they back). See terraform/README.md "Remote state bootstrap".
  backend "s3" {
    bucket         = "insucar-tfstate-326804802908"
    key            = "insucar/terraform.tfstate"
    region         = "eu-west-1"
    dynamodb_table = "insucar-tf-lock"
    encrypt        = true
  }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = {
      project     = "insucar"
      environment = var.environment
      managed_by  = "terraform"
    }
  }
}
