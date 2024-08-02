resource "aws_iam_role" "service_lambda_role" {
  name = "${var.environment_name}-${var.service_name}-lambda-role-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

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

resource "aws_iam_role_policy_attachment" "service_lambda_iam_policy_attachment" {
  role       = aws_iam_role.service_lambda_role.name
  policy_arn = aws_iam_policy.service_lambda_iam_policy.arn
}

resource "aws_iam_policy" "service_lambda_iam_policy" {
  name   = "${var.environment_name}-${var.service_name}-lambda-iam-policy-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  path   = "/"
  policy = data.aws_iam_policy_document.service_iam_policy_document.json
}

data "aws_iam_policy_document" "service_iam_policy_document" {

  statement {
    sid     = "EmailServiceLambdaLogsPermissions"
    effect  = "Allow"
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
    sid     = "EmailServiceLambdaEC2Permissions"
    effect  = "Allow"
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

  statement {
    sid = "EmailServiceSESPermissions"
    effect  = "Allow"
    actions = [
      "ses:SendEmail",
      "ses:SendRawEmail",
    ]
    resources = ["*"]
  }

  source_policy_documents = [
    data.aws_iam_policy_document.email_service_queue_policy_document.json,
    data.aws_iam_policy_document.email_service_queue_kms_key_policy_document.json,
    data.aws_iam_policy_document.email_templates_s3_bucket_policy_document.json,
    data.aws_iam_policy_document.email_service_dynamodb_policy_document.json
  ]
}

data "aws_iam_policy_document" "email_templates_s3_bucket_policy_document" {
  statement {
    sid = "EmailTemplatesS3Permission"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.terraform_remote_state.account.outputs.aws_account_id}:root"]
    }

    resources = [
      "arn:aws:s3:::pennsieve-${local.email_templates_bucket_name}",
      "arn:aws:s3:::pennsieve-${local.email_templates_bucket_name}/*",
    ]

    actions = [
      "s3:GetObject",
      "s3:GetObjectAttributes",
      "s3:ListBucket"
    ]
  }

  statement {
    sid    = "ForceSSLOnlyAccess"
    effect = "Deny"

    resources = [
      "arn:aws:s3:::pennsieve-${local.email_templates_bucket_name}",
      "arn:aws:s3:::pennsieve-${local.email_templates_bucket_name}/*",
    ]

    actions = [
      "s3:*",
    ]

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    condition {
      test     = "Bool"
      values   = ["false"]
      variable = "aws:SecureTransport"
    }
  }
}

data "aws_iam_policy_document" "email_service_queue_kms_key_policy_document" {
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
}

data "aws_iam_policy_document" "email_service_queue_policy_document" {
  statement {
    sid    = "EmailServiceSQSPermissions"
    effect    = "Allow"
    actions   = [
      "sqs:ReceiveMessage",
      "sqs:SendMessage",
      "sqs:DeleteMessage",
    ]
    resources = [aws_sqs_queue.email_service_queue.arn]

    principals {
      type        = "Service"
      identifiers = ["events.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "email_service_dynamodb_policy_document" {
  statement {
    sid = "EmailServiceLambdaDynamoDBPermissions"
    effect = "Allow"

    actions = [
      "dynamodb:DescribeTable",
      "dynamodb:BatchGetItem",
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:DeleteItem",
      "dynamodb:Query",
      "dynamodb:Scan",
      "dynamodb:PartiQLSelect"
    ]

    resources = [
      aws_dynamodb_table.email_message_templates_table.arn,
      "${aws_dynamodb_table.email_message_templates_table.arn}/*",
      aws_dynamodb_table.email_message_log_table.arn,
      "${aws_dynamodb_table.email_message_log_table.arn}/*"
    ]
  }
}