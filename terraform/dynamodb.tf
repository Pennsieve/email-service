resource "aws_dynamodb_table" "email_message_templates_table" {
  name         = "${var.environment_name}-email-message-templates-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "MessageId"

  # MessageId is the sole key: the handler does a GetItem by messageId to find
  # the template file + default subject. TemplateFile and Subject are plain
  # (non-key) attributes, so they are not declared here.
  attribute {
    name = "MessageId"
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
  name         = "${var.environment_name}-email-message-log-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  billing_mode = "PAY_PER_REQUEST"

  # Id is the dedupe key (DedupeId+recipient, or a hash of the request). It is
  # the sole key so the handler can use a conditional PutItem on it as the
  # idempotency guard and GetItem/UpdateItem by it for the journal status.
  hash_key = "Id"

  attribute {
    name = "Id"
    type = "S"
  }

  attribute {
    name = "Recipient"
    type = "S"
  }

  attribute {
    name = "SentAtKey"
    type = "S"
  }

  # Find all emails sent to a recipient, most-recent first. SentAtKey is a
  # zero-padded Unix-epoch string so lexicographic range ordering matches time
  # ordering; query with ScanIndexForward=false for newest-first. MessageId is
  # projected (ALL) but is not a key, so filter by it client-side or with a
  # FilterExpression. (Only key attributes are declared above; MessageId and the
  # other item attributes do not need attribute{} blocks.)
  global_secondary_index {
    name            = "RecipientSentAtIndex"
    hash_key        = "Recipient"
    range_key       = "SentAtKey"
    projection_type = "ALL"
  }

  # Expire journal rows after journal_ttl_days. The handler writes ExpiresAt as
  # a Unix epoch (seconds); DynamoDB deletes rows shortly after that time.
  ttl {
    attribute_name = "ExpiresAt"
    enabled        = true
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