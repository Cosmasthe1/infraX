resource "aws_vpc" "main" {
  cidr_block = var.cidr_block

  tags = {
    Name        = "infrax-${var.environment}"
    Environment = var.environment
  }
}
