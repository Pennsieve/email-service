## Create Dataset Assets S3 Bucket
resource "aws_s3_bucket" "email_templates_s3_bucket" {
  bucket = local.email_templates_bucket_name

  lifecycle {
    prevent_destroy = true
  }

  tags = merge(
    local.common_tags,
    {
      "Name"         = local.email_templates_bucket_name
      "name"         = local.email_templates_bucket_name
      "service_name" = var.service_name
      "tier"         = "s3"
    },
  )
}

resource "aws_s3_bucket_policy" "email_templates_s3_bucket_policy" {
  bucket = aws_s3_bucket.email_templates_s3_bucket.bucket
  policy = data.aws_iam_policy_document.email_templates_s3_bucket_policy_document.json
}

resource "aws_s3_bucket_server_side_encryption_configuration" "email_templates_s3_bucket_encryption" {
  bucket = aws_s3_bucket.email_templates_s3_bucket.bucket

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = local.encryption_algorithm
    }
  }
}

resource "aws_s3_bucket_logging" "email_templates_s3_bucket_logging" {
  bucket = aws_s3_bucket.email_templates_s3_bucket.id

  target_bucket = data.terraform_remote_state.region.outputs.logs_s3_bucket_id
  target_prefix = local.email_templates_logs_target_prefix
}

resource "aws_s3_bucket_cors_configuration" "email_templates_s3_bucket_cors" {
  bucket = aws_s3_bucket.email_templates_s3_bucket.bucket

  cors_rule {
    allowed_headers = ["*"]
    allowed_methods = ["GET", "HEAD"]
    allowed_origins = ["*"]
    max_age_seconds = 3000
  }
}