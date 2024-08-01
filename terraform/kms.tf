resource "aws_kms_alias" "email_service_sqs_kms_key_alias" {
  name          = "alias/${var.environment_name}-${var.service_name}-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  target_key_id = aws_kms_key.email_service_sqs_kms_key.key_id
}

resource "aws_kms_key" "email_service_sqs_kms_key" {
  description             = "${var.environment_name}-${var.service_name}-queue-key-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  deletion_window_in_days = 10
  enable_key_rotation     = true
  policy                  = data.aws_iam_policy_document.email_service_queue_kms_key_policy_document.json
}
