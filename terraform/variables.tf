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

variable "single_nat_gateway" {
  description = "Use one shared NAT gateway (cheaper, dev) vs one per AZ (HA, prod). Set false for prod."
  type        = bool
  default     = true
}

variable "admin_cidrs" {
  description = "CIDRs allowed to reach the Jenkins/Spinnaker LoadBalancers. Lock down before real use (default is intentionally NOT 0.0.0.0/0)."
  type        = list(string)
  default     = []
}

variable "db_instance_class" {
  description = "RDS instance class for the managed PostgreSQL."
  type        = string
  default     = "db.t3.medium"
}

variable "db_multi_az" {
  description = "Run RDS with a synchronous standby in a second AZ (HA). Set false only for throwaway dev."
  type        = bool
  default     = true
}
