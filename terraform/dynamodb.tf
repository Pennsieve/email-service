resource "aws_dynamodb_table" "email_message_templates_table" {
  name           = "${var.environment_name}-email-message-templates-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "MessageId"
  range_key      = "TemplateFile"

  attribute {
    name = "MessageId"
    type = "S"
  }

  attribute {
    name = "TemplateFile"
    type = "S"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = merge(
    local.common_tags,
    {
      "Name"         = "${var.environment_name}-email-message-templates-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "name"         = "${var.environment_name}-email-message-templates-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "service_name" = var.service_name
    },
  )

}

resource "aws_dynamodb_table" "email_message_log_table" {
  name           = "${var.environment_name}-email-message-log-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "Id"
  range_key      = "MessageId"

  attribute {
    name = "Id"
    type = "S"
  }

  attribute {
    name = "MessageId"
    type = "S"
  }

  attribute {
    name = "Recipient"
    type = "S"
  }

  global_secondary_index {
    name               = "RecipientMessageIdIndex"
    hash_key           = "Recipient"
    range_key          = "MessageId"
    projection_type    = "ALL"
  }

  point_in_time_recovery {
    enabled = true
  }

  server_side_encryption {
    enabled = true
  }

  tags = merge(
    local.common_tags,
    {
      "Name"         = "${var.environment_name}-email-message-log-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "name"         = "${var.environment_name}-email-message-log-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "service_name" = var.service_name
    },
  )

}