variable "environment" {
  description = "Deployment environment name"
  type        = string
  default     = "dev"
}

variable "cidr_block" {
  description = "Primary VPC CIDR block"
  type        = string
  default     = "10.10.0.0/16"
}

variable "aws_region" {
  description = "AWS region for the VPC"
  type        = string
  default     = "us-east-1"
}
