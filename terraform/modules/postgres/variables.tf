variable "name_prefix" {
  type        = string
  description = "Prefix for resource names"
}

variable "instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "allocated_storage" {
  type    = number
  default = 20
}

variable "engine" {
  type    = string
  default = "postgres"
}

variable "engine_version" {
  type    = string
  default = "15"
}

variable "db_name" {
  type    = string
  default = "infrax"
}

variable "username" {
  type    = string
  default = "infrax"
}

variable "password" {
  type    = string
  default = "changeme"
}

variable "subnet_ids" {
  type    = list(string)
  default = []
}

variable "vpc_security_group_ids" {
  type    = list(string)
  default = []
}
