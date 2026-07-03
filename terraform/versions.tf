terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
  }
  # For real use, store state remotely:
  # backend "s3" { bucket = "insucar-tfstate-326804802908" key = "insucar/terraform.tfstate" region = "eu-west-1" dynamodb_table = "insucar-tf-lock" }
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
