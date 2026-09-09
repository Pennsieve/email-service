# T1 CloudWatch alarms (EPIC 868m2zvjt; standard sets from
# pennsieve-infra-dashboard/docs/alarm-coverage-plan.md). The custom
# rate-limit-exceeded alarm in cloudwatch.tf stays as-is. No alarm_actions
# yet.
module "service_alarms" {
  source = "git@github.com:Pennsieve/terraform-modules.git//service-alarms"

  environment_name = var.environment_name
  service_name     = var.service_name

  lambdas = {
    queue = {
      function_name   = aws_lambda_function.queue_lambda.function_name
      timeout_seconds = aws_lambda_function.queue_lambda.timeout
    }
    bounce = {
      function_name   = aws_lambda_function.bounce_lambda.function_name
      timeout_seconds = aws_lambda_function.bounce_lambda.timeout
    }
  }

  dynamodb_tables = {
    message-templates = aws_dynamodb_table.email_message_templates_table.name
    suppression       = aws_dynamodb_table.email_suppression_table.name
    rate-counter      = aws_dynamodb_table.email_rate_counter_table.name
    message-log       = aws_dynamodb_table.email_message_log_table.name
  }

  queues = {
    email = {
      queue_name      = aws_sqs_queue.email_service_queue.name
      max_age_seconds = 1800
    }
  }

  dlqs = {
    email = aws_sqs_queue.email_service_deadletter_queue.name
  }
}
