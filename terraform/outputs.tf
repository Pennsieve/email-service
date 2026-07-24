output "queue_lambda_arn" {
  value = aws_lambda_function.queue_lambda.arn
}

output "queue_lambda_function_name" {
  value = aws_lambda_function.queue_lambda.function_name
}

output "email_service_queue_arn" {
  value = aws_sqs_queue.email_service_queue.arn
}

output "email_service_queue_url" {
  value = aws_sqs_queue.email_service_queue.id
}

# The queue is KMS-encrypted; producers need this key's ARN to grant themselves
# kms:GenerateDataKey/Decrypt so their sqs:SendMessage is not denied.
output "email_service_queue_kms_key_arn" {
  value = aws_kms_key.email_service_sqs_kms_key.arn
}

output "email_templates_bucket_name" {
  value = aws_s3_bucket.email_templates_s3_bucket.bucket
}

# Connection info for the notifications database (rds.tf). No consumer reads
# these yet (data layer only); they're exposed for whichever future lambda or
# migration job needs to connect.
output "notifications_db_endpoint" {
  value = aws_db_instance.notifications_db.endpoint
}

output "notifications_db_name" {
  value = aws_db_instance.notifications_db.db_name
}

# The secret holding {host, port, dbname, username, password} as JSON.
output "notifications_db_secret_arn" {
  value = aws_secretsmanager_secret.notifications_db_secret.arn
}

