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
