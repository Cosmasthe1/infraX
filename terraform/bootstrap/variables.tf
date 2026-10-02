variable "bucket_name" {
  description = "S3 bucket for Terraform state"
  type        = string
  default     = "infrax-terraform-state"
}

variable "dynamodb_table" {
  description = "DynamoDB table name for Terraform locks"
  type        = string
  default     = "infrax-terraform-locks"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}
