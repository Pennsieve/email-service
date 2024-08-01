resource "aws_sqs_queue" "email_service_queue" {
  name                       = "${var.environment_name}-${var.service_name}-queue-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  delay_seconds              = 5
  max_message_size           = 262144
  message_retention_seconds  = 86400
  kms_master_key_id          = "alias/${var.environment_name}-${var.service_name}-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  receive_wait_time_seconds  = 10
  visibility_timeout_seconds = 30
  redrive_policy             = "{\"deadLetterTargetArn\":\"${aws_sqs_queue.email_service_deadletter_queue.arn}\",\"maxReceiveCount\":3}"
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
  policy    = data.aws_iam_policy_document.email_service_queue_policy_document.json
}
