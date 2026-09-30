module "network" {
  source      = "../../modules/network"
  environment = "dev"
  cidr_block  = "10.20.0.0/16"
  aws_region  = "us-east-1"
}
