resource "aws_sqs_queue" "email_service_queue" {
  name                      = "${var.environment_name}-${var.service_name}-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  delay_seconds             = 5
  max_message_size          = 262144
  message_retention_seconds = 86400
  kms_master_key_id         = "alias/${var.environment_name}-${var.service_name}-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  receive_wait_time_seconds = 10
  # Must be > the queue lambda's function timeout (150s in lambda.tf), or the
  # Lambda event source mapping is rejected: "Queue visibility timeout is less
  # than Function timeout". Set to 3x the lambda timeout so a near-timeout
  # invocation isn't redelivered while still running. Update both together if
  # either changes.
  visibility_timeout_seconds = 450
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.email_service_deadletter_queue.arn
    maxReceiveCount     = 3
  })

  # The redrive_policy references the DLQ's ARN, but on a first apply SQS is
  # eventually consistent: the DLQ's CreateQueue can return before its ARN is
  # resolvable, so the main queue's CreateQueue fails with "Dead letter target
  # does not exist". Force strict ordering so the DLQ is fully created first.
  depends_on = [aws_sqs_queue.email_service_deadletter_queue]
}

resource "aws_sqs_queue" "email_service_deadletter_queue" {
  name                      = "${var.environment_name}-${var.service_name}-deadletter-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  delay_seconds             = 5
  kms_master_key_id         = "alias/${var.environment_name}-${var.service_name}-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  max_message_size          = 262144
  message_retention_seconds = 86400
  receive_wait_time_seconds = 10
}

resource "aws_sqs_queue_policy" "email_service_sqs_queue_policy" {
  queue_url = aws_sqs_queue.email_service_queue.id
  policy    = data.aws_iam_policy_document.email_service_sqs_policy_document.json
}
