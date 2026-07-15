############################################################
# Queue Lambda
############################################################
resource "aws_iam_role" "queue_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-queue-lambda-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  assume_role_policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "queue_lambda_iam_policy_attachment" {
  role       = aws_iam_role.queue_lambda_role.name
  policy_arn = aws_iam_policy.queue_lambda_iam_policy.arn
}

resource "aws_iam_policy" "queue_lambda_iam_policy" {
  name   = "${var.environment_name}-${var.service_name}-queue-lambda-iam-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  path   = "/"
  policy = data.aws_iam_policy_document.queue_lambda_iam_policy_document.json
}

# Identity policy attached to the queue lambda role. Note these are identity
# (role) policy statements, so they must NOT contain principals — that is what
# distinguishes them from the bucket/queue resource policies in s3.tf/sqs.tf.
data "aws_iam_policy_document" "queue_lambda_iam_policy_document" {
  statement {
    sid    = "EmailServiceLambdaLogsPermissions"
    effect = "Allow"
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutDestination",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams"
    ]
    resources = ["*"]
  }

  statement {
    sid    = "EmailServiceLambdaEC2Permissions"
    effect = "Allow"
    actions = [
      "ec2:CreateNetworkInterface",
      "ec2:DescribeNetworkInterfaces",
      "ec2:DeleteNetworkInterface",
      "ec2:AssignPrivateIpAddresses",
      "ec2:UnassignPrivateIpAddresses"
    ]
    resources = ["*"]
  }

  statement {
    sid    = "EmailServiceSecretsManagerPermissions"
    effect = "Allow"
    actions = [
      "kms:Decrypt",
      "secretsmanager:GetSecretValue",
    ]
    resources = [
      data.aws_kms_key.ssm_kms_key.arn,
    ]
  }

  statement {
    sid    = "EmailServiceSSMPermissions"
    effect = "Allow"
    actions = [
      "ssm:GetParameter",
      "ssm:GetParameters",
      "ssm:GetParametersByPath",
    ]
    resources = ["arn:aws:ssm:${data.aws_region.current_region.name}:${data.aws_caller_identity.current.account_id}:parameter/${var.environment_name}/${var.service_name}/*"]
  }

  # Consume from the SQS queue. The event source mapping polls on the lambda's
  # behalf, so the role needs Receive/Delete/GetAttributes on the queue.
  statement {
    sid    = "EmailServiceSQSConsume"
    effect = "Allow"
    actions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:GetQueueAttributes",
    ]
    resources = [aws_sqs_queue.email_service_queue.arn]
  }

  # Decrypt SQS messages encrypted with the queue KMS key.
  statement {
    sid    = "EmailServiceSQSDecrypt"
    effect = "Allow"
    actions = [
      "kms:Decrypt",
      "kms:GenerateDataKey",
    ]
    resources = [aws_kms_key.email_service_sqs_kms_key.arn]
  }

  # Read email templates from the S3 bucket.
  statement {
    sid    = "EmailTemplatesS3Read"
    effect = "Allow"
    actions = [
      "s3:GetObject",
      "s3:GetObjectAttributes",
      "s3:ListBucket",
    ]
    resources = [
      aws_s3_bucket.email_templates_s3_bucket.arn,
      "${aws_s3_bucket.email_templates_s3_bucket.arn}/*",
    ]
  }

  # Read the template mapping table and read/write the journal table.
  statement {
    sid    = "EmailServiceDynamoDB"
    effect = "Allow"
    actions = [
      "dynamodb:DescribeTable",
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      # Query is for the RecipientSentAtIndex GSI (troubleshooting / future code);
      # the handler itself uses only GetItem/PutItem/UpdateItem today.
      "dynamodb:Query",
    ]
    resources = [
      aws_dynamodb_table.email_message_templates_table.arn,
      "${aws_dynamodb_table.email_message_templates_table.arn}/*",
      aws_dynamodb_table.email_message_log_table.arn,
      "${aws_dynamodb_table.email_message_log_table.arn}/*",
      aws_dynamodb_table.email_suppression_table.arn,
      "${aws_dynamodb_table.email_suppression_table.arn}/*",
    ]
  }

  # Send email through SES.
  statement {
    sid    = "EmailServiceSESSend"
    effect = "Allow"
    actions = [
      "ses:SendEmail",
    ]
    resources = ["*"]
  }
}

############################################################
# Resource policy documents (these DO carry principals)
############################################################

# KMS key policy for the SQS queue key (referenced by kms.tf).
data "aws_iam_policy_document" "email_service_kms_key_policy_document" {
  statement {
    sid       = "EnableIAMUserPermissions"
    effect    = "Allow"
    actions   = ["kms:*"]
    resources = ["*"]

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.terraform_remote_state.account.outputs.aws_account_id}:root"]
    }
  }

  statement {
    sid    = "EnableCloudwatchEventPermissions"
    effect = "Allow"
    actions = [
      "kms:GenerateDataKey",
      "kms:Decrypt",
    ]
    resources = ["*"]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
  }

  # Let the SQS service use the key when delivering messages to/from the
  # encrypted queue. The lambda role itself decrypts via its identity policy
  # (EmailServiceSQSDecrypt), which the account-root kms:* grant above enables.
  statement {
    sid    = "AllowSQSServiceUseOfKey"
    effect = "Allow"
    actions = [
      "kms:GenerateDataKey",
      "kms:Decrypt",
    ]
    resources = ["*"]

    principals {
      type        = "Service"
      identifiers = ["sqs.amazonaws.com"]
    }
  }
}

# SQS queue resource policy (referenced by sqs.tf).
data "aws_iam_policy_document" "email_service_sqs_policy_document" {
  statement {
    sid    = "EmailServiceSQSPermissions"
    effect = "Allow"
    actions = [
      "sqs:SendMessage",
    ]
    resources = [aws_sqs_queue.email_service_queue.arn]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
  }
}
