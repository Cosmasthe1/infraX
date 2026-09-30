output "vpc_id" {
  description = "ID of the created VPC"
  value       = aws_vpc.main.id
}

output "cidr_block" {
  description = "CIDR block used by the VPC"
  value       = aws_vpc.main.cidr_block
}
