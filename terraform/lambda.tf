resource "aws_lambda_function" "queue_lambda" {
  description   = "A Serverless Service that sends emails for the Pennsieve platform - requests come from SQS queue"
  function_name = "${var.environment_name}-${var.service_name}-queue-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler       = "bootstrap"
  runtime       = "provided.al2"
  architectures = ["arm64"]
  role          = aws_iam_role.queue_lambda_role.arn
  # Sending an email is quick; 150s is generous headroom. The SQS queue's
  # visibility timeout (sqs.tf) must stay greater than this — keep them in sync.
  timeout     = 150
  memory_size = 128
  s3_bucket   = var.lambda_bucket
  s3_key      = "${var.service_name}/${var.service_name}-${var.image_tag}.zip"

  vpc_config {
    subnet_ids         = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)
    security_group_ids = [data.terraform_remote_state.platform_infrastructure.outputs.upload_v2_security_group_id]
  }

  environment {
    variables = {
      ENV              = var.environment_name
      PENNSIEVE_DOMAIN = data.terraform_remote_state.account.outputs.domain_name,
      REGION           = var.aws_region
      S3_BUCKET        = aws_s3_bucket.email_templates_s3_bucket.bucket
      TEMPLATES_TABLE  = aws_dynamodb_table.email_message_templates_table.name
      JOURNAL_TABLE    = aws_dynamodb_table.email_message_log_table.name
      JOURNAL_TTL_DAYS = var.journal_ttl_days
      # slog level read by internal/logging (DEBUG|INFO|WARN|ERROR); defaults to
      # INFO in code, so this just makes it overridable per environment.
      LOG_LEVEL = var.log_level
      # Send controls: SEND_ENABLED=false makes the whole service log-only;
      # SUPPRESSION_TABLE holds per-address suppressions. Template-level control
      # is a SendDisabled attribute on the email-message-templates items.
      SEND_ENABLED      = var.send_enabled
      SUPPRESSION_TABLE = aws_dynamodb_table.email_suppression_table.name
      # Rate safeguard: caps emails handed to SES per minute (protects the SES
      # account from a looping producer). Over-cap sends become log-only.
      RATE_LIMIT_TABLE                  = aws_dynamodb_table.email_rate_counter_table.name
      SEND_RATE_LIMIT_PER_MINUTE        = var.send_rate_limit_per_minute
      PER_MESSAGE_RATE_LIMIT_PER_MINUTE = var.per_message_rate_limit_per_minute
    }
  }
}

# Wire the SQS queue to the queue lambda. ReportBatchItemFailures lets the
# handler return only the records that failed so a single poison message does
# not force the whole batch to be retried (and eventually DLQ'd).
resource "aws_lambda_event_source_mapping" "queue_lambda_sqs_trigger" {
  event_source_arn        = aws_sqs_queue.email_service_queue.arn
  function_name           = aws_lambda_function.queue_lambda.arn
  batch_size              = 10
  function_response_types = ["ReportBatchItemFailures"]
}

# Bounce/complaint handler: SES publishes bounce & complaint notifications to an
# SNS topic; this lambda adds the affected addresses to the suppression table.
resource "aws_lambda_function" "bounce_lambda" {
  description   = "email-service bounce/complaint handler - suppresses addresses SES reports as bounced or complained"
  function_name = "${var.environment_name}-${var.service_name}-bounce-lambda-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  handler       = "bootstrap"
  runtime       = "provided.al2"
  architectures = ["arm64"]
  role          = aws_iam_role.bounce_lambda_role.arn
  timeout       = 60
  memory_size   = 128
  s3_bucket     = var.lambda_bucket
  s3_key        = "${var.service_name}/${var.service_name}-bounce-${var.image_tag}.zip"

  environment {
    variables = {
      ENV               = var.environment_name
      REGION            = var.aws_region
      SUPPRESSION_TABLE = aws_dynamodb_table.email_suppression_table.name
    }
  }
}

# Let SNS invoke the bounce lambda.
resource "aws_lambda_permission" "bounce_lambda_sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.bounce_lambda.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.email_bounce_topic.arn
}

resource "aws_sns_topic_subscription" "bounce_lambda_subscription" {
  topic_arn = aws_sns_topic.email_bounce_topic.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.bounce_lambda.arn
}
