# Bounce/complaint notification pipeline.
#
# SES publishes bounce and complaint notifications for the Pennsieve domain
# identity to this SNS topic; the bounce lambda (lambda.tf) subscribes and adds
# the affected addresses to the suppression table. Continuing to send to
# hard-bouncing or complaining addresses is what drives the bounce/complaint
# rates AWS suspends SES accounts over, so this closes the loop automatically.

resource "aws_sns_topic" "email_bounce_topic" {
  name = "${var.environment_name}-${var.service_name}-bounce-topic-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
}

# Allow SES to publish to the topic.
resource "aws_sns_topic_policy" "email_bounce_topic_policy" {
  arn    = aws_sns_topic.email_bounce_topic.arn
  policy = data.aws_iam_policy_document.email_bounce_topic_policy_document.json
}

data "aws_iam_policy_document" "email_bounce_topic_policy_document" {
  statement {
    sid       = "AllowSESPublish"
    effect    = "Allow"
    actions   = ["sns:Publish"]
    resources = [aws_sns_topic.email_bounce_topic.arn]

    principals {
      type        = "Service"
      identifiers = ["ses.amazonaws.com"]
    }

    # Scope to notifications originating from this account's SES.
    condition {
      test     = "StringEquals"
      variable = "AWS:SourceAccount"
      values   = [data.aws_caller_identity.current.account_id]
    }
  }
}

# Route bounce notifications for the domain identity to the topic. The domain
# identity itself is verified elsewhere in the platform; here we only attach the
# notification routing, which is idempotent and identity-scoped.
resource "aws_ses_identity_notification_topic" "bounce" {
  topic_arn                = aws_sns_topic.email_bounce_topic.arn
  notification_type        = "Bounce"
  identity                 = local.domain_name
  include_original_headers = false
}

resource "aws_ses_identity_notification_topic" "complaint" {
  topic_arn                = aws_sns_topic.email_bounce_topic.arn
  notification_type        = "Complaint"
  identity                 = local.domain_name
  include_original_headers = false
}
