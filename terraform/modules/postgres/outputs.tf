output "instance_id" {
  value       = aws_db_instance.this.id
  description = "RDS instance id"
}

output "address" {
  value       = aws_db_instance.this.address
  description = "RDS address"
}

output "endpoint" {
  value       = aws_db_instance.this.endpoint
  description = "RDS endpoint"
}
