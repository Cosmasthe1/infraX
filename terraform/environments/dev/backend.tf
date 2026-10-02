terraform {
  backend "s3" {
    bucket         = "infrax-terraform-state"
    key            = "dev/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "infrax-terraform-locks"
    encrypt        = true
  }
}
