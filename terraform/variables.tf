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

locals {
  domain_name = data.terraform_remote_state.account.outputs.domain_name
  hosted_zone = data.terraform_remote_state.account.outputs.public_hosted_zone_id

  email_templates                    = "email-templates"
  email_templates_bucket_name        = "pennsieve-${var.environment_name}-${local.email_templates}-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  email_templates_logs_target_prefix = "${var.environment_name}/${local.email_templates}/s3/"

  encryption_algorithm = "AES256"
  common_tags = {
    aws_account      = var.aws_account
    aws_region       = data.aws_region.current_region.name
    environment_name = var.environment_name
  }
}