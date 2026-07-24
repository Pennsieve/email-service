variable "aws_account" {}

variable "aws_region" {}

variable "environment_name" {}

variable "service_name" {}

variable "vpc_name" {}

variable "domain_name" {}

variable "image_tag" {}

variable "lambda_bucket" {
  default = "pennsieve-cc-lambda-functions-use1"
}

# Number of days a row in the email-message-log journal is retained before the
# DynamoDB TTL expires it. Controls how far back "I never got the email"
# troubleshooting can reach.
variable "journal_ttl_days" {
  default = 90
}

# slog level for the queue lambda: DEBUG | INFO | WARN | ERROR.
variable "log_level" {
  default = "INFO"
}

############################################################
# Notifications Postgres database (see rds.tf)
############################################################

variable "notifications_db_instance_class" {
  default = "db.t4g.micro"
}

variable "notifications_db_engine_version" {
  default = "16.4"
}

variable "notifications_db_allocated_storage" {
  default = 20
}

# Ceiling for RDS storage autoscaling; set equal to notifications_db_allocated_storage to disable it.
variable "notifications_db_max_allocated_storage" {
  default = 100
}

variable "notifications_db_multi_az" {
  default = false
}

variable "notifications_db_deletion_protection" {
  default = false
}

locals {
  domain_name = data.terraform_remote_state.account.outputs.domain_name
  hosted_zone = data.terraform_remote_state.account.outputs.public_hosted_zone_id

  email_templates                    = "email-templates"
  email_templates_bucket_name        = "pennsieve-${var.environment_name}-${local.email_templates}-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  email_templates_logs_target_prefix = "${var.environment_name}/email-templates/s3/"

  encryption_algorithm = "AES256"
  common_tags = {
    aws_account      = var.aws_account
    aws_region       = data.aws_region.current_region.name
    environment_name = var.environment_name
  }
}