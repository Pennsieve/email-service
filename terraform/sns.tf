# Bounce/complaint subscriptions.
#
# SES publishes bounce and complaint notifications for the Pennsieve domain
# identity to SNS topics. Those topics and the identity->topic routing are
# managed centrally (SES identity administration is house-controlled — the
# service deploy role is intentionally not granted ses:SetIdentityNotificationTopic),
# so this service does NOT create the topics or touch the identity routing.
# Instead, the bounce lambda (lambda.tf) subscribes to the EXISTING platform
# Bounce and Complaint topics and adds the affected addresses to the suppression
# table.
#
# The topic ARNs are passed in via bounce_topic_arn / complaint_topic_arn. Until
# they are supplied, no subscription is created (the lambda deploys but receives
# nothing) — this keeps the deploy green while the topic ARNs are confirmed with
# the platform owner.

locals {
  # Non-empty existing SES notification topic ARNs to subscribe the bounce lambda
  # to, keyed for stable for_each addressing.
  bounce_topic_arns = {
    for k, v in {
      bounce    = var.bounce_topic_arn
      complaint = var.complaint_topic_arn
    } : k => v if v != ""
  }
}

# Subscribe the bounce lambda to each existing SES notification topic.
resource "aws_sns_topic_subscription" "bounce_lambda_subscription" {
  for_each  = local.bounce_topic_arns
  topic_arn = each.value
  protocol  = "lambda"
  endpoint  = aws_lambda_function.bounce_lambda.arn
}

# Let each subscribed topic invoke the bounce lambda.
resource "aws_lambda_permission" "bounce_lambda_sns" {
  for_each      = local.bounce_topic_arns
  statement_id  = "AllowSNSInvoke-${each.key}"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.bounce_lambda.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = each.value
}
