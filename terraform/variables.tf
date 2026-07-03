variable "region" {
  description = "AWS region"
  type        = string
  default     = "eu-west-1"
}

variable "environment" {
  description = "Environment tier (dev|uat|prod). In production each runs in a SEPARATE AWS account."
  type        = string
  default     = "dev"
}

variable "cluster_name" {
  type    = string
  default = "insucar"
}

variable "kubernetes_version" {
  type    = string
  default = "1.30"
}

variable "node_instance_type" {
  type    = string
  default = "t3.xlarge"
}

variable "node_min" {
  type    = number
  default = 2
}

variable "node_desired" {
  type    = number
  default = 2
}

variable "node_max" {
  type    = number
  default = 5
}

variable "vpc_cidr" {
  type    = string
  default = "10.42.0.0/16"
}
