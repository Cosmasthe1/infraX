output "bucket_name" {
  value       = aws_s3_bucket.tf_state.bucket
  description = "S3 bucket created for Terraform state"
}

output "dynamodb_table" {
  value       = aws_dynamodb_table.tf_locks.name
  description = "DynamoDB table created for Terraform locks"
}
