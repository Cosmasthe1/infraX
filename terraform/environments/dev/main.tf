module "network" {
  source      = "../../modules/network"
  environment = "dev"
  cidr_block  = "10.20.0.0/16"
  aws_region  = "us-east-1"
}

module "postgres" {
  source = "../../modules/postgres"

  name_prefix = "dev-postgres"
  instance_class = "db.t4g.micro"
  allocated_storage = 20
  engine = "postgres"
  engine_version = "15"
  db_name = "infrax"
  username = "infrax"
  password = "changeme"

  # optional: wire network outputs when available
  # subnet_ids = module.network.private_subnet_ids
  # vpc_security_group_ids = [module.network.default_sg_id]
}
