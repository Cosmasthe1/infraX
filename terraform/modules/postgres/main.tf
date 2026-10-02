locals {
  use_subnet_group = length(var.subnet_ids) > 0
}

resource "aws_db_subnet_group" "this" {
  count     = local.use_subnet_group ? 1 : 0
  name      = "${var.name_prefix}-subnet-group"
  subnet_ids = var.subnet_ids
  tags = {
    Name = "${var.name_prefix}-subnet-group"
  }
}

resource "aws_db_instance" "this" {
  identifier         = "${var.name_prefix}-db"
  allocated_storage  = var.allocated_storage
  engine             = var.engine
  engine_version     = var.engine_version
  instance_class     = var.instance_class
  name               = var.db_name
  username           = var.username
  password           = var.password
  skip_final_snapshot = true

  vpc_security_group_ids = var.vpc_security_group_ids

  db_subnet_group_name = local.use_subnet_group ? aws_db_subnet_group.this[0].name : null

  tags = {
    Name = "${var.name_prefix}-db"
  }
}
