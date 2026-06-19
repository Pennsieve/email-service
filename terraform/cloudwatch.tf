// Create log group for queue lambda.
resource "aws_cloudwatch_log_group" "queue_lambda_log_group" {
  name              = "/aws/lambda/${aws_lambda_function.queue_lambda.function_name}"
  retention_in_days = 30
  tags              = local.common_tags
}

// Send logs from queue lambda to Datadog
resource "aws_cloudwatch_log_subscription_filter" "cloudwatch_log_group_subscription" {
  name            = "${aws_cloudwatch_log_group.queue_lambda_log_group.name}-subscription"
  log_group_name  = aws_cloudwatch_log_group.queue_lambda_log_group.name
  filter_pattern  = ""
  destination_arn = data.terraform_remote_state.region.outputs.datadog_delivery_stream_arn
  role_arn        = data.terraform_remote_state.region.outputs.cw_logs_to_datadog_logs_firehose_role_arn
}
