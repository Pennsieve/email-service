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

// Rate-limit tripwire: the handler logs {"msg":"rate limit exceeded"} at WARN
// whenever a send is suppressed by the rate safeguard. Turn that into a metric
// and alarm so a looping producer (the SES-reputation risk) pages someone.
resource "aws_cloudwatch_log_metric_filter" "rate_limit_exceeded" {
  name           = "${var.environment_name}-${var.service_name}-rate-limit-exceeded"
  log_group_name = aws_cloudwatch_log_group.queue_lambda_log_group.name
  pattern        = "{ $.msg = \"rate limit exceeded\" }"

  metric_transformation {
    name          = "RateLimitExceeded"
    namespace     = "Pennsieve/EmailService"
    value         = "1"
    default_value = "0"
  }
}

resource "aws_cloudwatch_metric_alarm" "rate_limit_exceeded" {
  alarm_name          = "${var.environment_name}-${var.service_name}-rate-limit-exceeded"
  alarm_description   = "email-service rate limit tripped — a producer may be looping and sends are being held back (log-only) to protect SES."
  namespace           = "Pennsieve/EmailService"
  metric_name         = "RateLimitExceeded"
  statistic           = "Sum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 1
  comparison_operator = "GreaterThanOrEqualToThreshold"
  treat_missing_data  = "notBreaching"
  tags                = local.common_tags
}
