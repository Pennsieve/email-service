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

output "email_templates_bucket_name" {
  value = aws_s3_bucket.email_templates_s3_bucket.bucket
}
