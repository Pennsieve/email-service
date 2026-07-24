############################################################
# Notifications Postgres database
#
# Backs the notification schema (users/topics/subscriptions/notifications/
# messages/user_notifications/notification_preferences/notification_audits —
# see internal/notifications). Schema is applied out-of-band via the
# migrations in internal/notifications/migrations (see README "Notifications
# schema" section) — Terraform only provisions the instance and credentials.
############################################################

# Master credentials. Generated once and stored in Secrets Manager; the
# password is never in state as plaintext beyond the resource itself (which
# Terraform state always risks for random_password — treat state as sensitive,
# same as every other secret-bearing resource in this stack).
resource "random_password" "notifications_db_master_password" {
  length  = 32
  special = false
}

resource "aws_secretsmanager_secret" "notifications_db_secret" {
  name = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  tags = merge(
    local.common_tags,
    {
      "Name"         = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "service_name" = var.service_name
    },
  )
}

# Holds the connection info a future consumer needs to build a DSN: host,
# port, dbname, username, password. Kept as one JSON secret rather than
# separate parameters so the lambda only needs one GetSecretValue call.
resource "aws_secretsmanager_secret_version" "notifications_db_secret_version" {
  secret_id = aws_secretsmanager_secret.notifications_db_secret.id
  secret_string = jsonencode({
    host     = aws_db_instance.notifications_db.address
    port     = aws_db_instance.notifications_db.port
    dbname   = aws_db_instance.notifications_db.db_name
    username = aws_db_instance.notifications_db.username
    password = random_password.notifications_db_master_password.result
  })
}

resource "aws_db_subnet_group" "notifications_db_subnet_group" {
  name       = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  subnet_ids = tolist(data.terraform_remote_state.vpc.outputs.private_subnet_ids)

  tags = merge(
    local.common_tags,
    {
      "Name"         = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "service_name" = var.service_name
    },
  )
}

# Ingress is scoped to the queue lambda's security group so only this service
# can reach the database directly.
resource "aws_security_group" "notifications_db_security_group" {
  name        = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  description = "Allow Postgres access from the ${var.service_name} queue lambda"
  vpc_id      = data.terraform_remote_state.vpc.outputs.vpc_id

  ingress {
    description     = "Postgres from the queue lambda"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [data.terraform_remote_state.platform_infrastructure.outputs.upload_v2_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    local.common_tags,
    {
      "Name"         = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "service_name" = var.service_name
    },
  )
}

resource "aws_db_instance" "notifications_db" {
  identifier     = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
  engine         = "postgres"
  engine_version = var.notifications_db_engine_version
  instance_class = var.notifications_db_instance_class

  allocated_storage     = var.notifications_db_allocated_storage
  max_allocated_storage = var.notifications_db_max_allocated_storage
  storage_type          = "gp3"
  storage_encrypted     = true

  db_name  = "notifications"
  username = "notifications_admin"
  password = random_password.notifications_db_master_password.result
  port     = 5432

  db_subnet_group_name   = aws_db_subnet_group.notifications_db_subnet_group.name
  vpc_security_group_ids = [aws_security_group.notifications_db_security_group.id]
  publicly_accessible    = false
  multi_az               = var.notifications_db_multi_az

  backup_retention_period = 7
  # Applied at the next maintenance window instead of immediately, so a
  # `terraform apply` never triggers an on-the-spot reboot/failover.
  apply_immediately   = false
  deletion_protection = var.notifications_db_deletion_protection
  # A final snapshot on destroy is the default-safe choice for a database that
  # holds an audit trail (NotificationAudit) and message history (Message).
  skip_final_snapshot       = false
  final_snapshot_identifier = "${var.environment_name}-${var.service_name}-notifications-db-final-${data.terraform_remote_state.region.outputs.aws_region_shortname}"

  tags = merge(
    local.common_tags,
    {
      "Name"         = "${var.environment_name}-${var.service_name}-notifications-db-${data.terraform_remote_state.region.outputs.aws_region_shortname}"
      "service_name" = var.service_name
    },
  )
}
