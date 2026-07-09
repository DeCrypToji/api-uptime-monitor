# ---------------------------------------------------------------------------
# Generate a strong random password for the RDS master user.
# NOTE: this value passes through terraform.tfstate in plaintext.
# terraform.tfstate is gitignored for exactly this reason.
# ---------------------------------------------------------------------------
resource "random_password" "db_master" {
  length  = 32
  special = true
  # RDS disallows these characters in the master password
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

# ---------------------------------------------------------------------------
# Store the password in AWS Secrets Manager (encrypted at rest).
# Backend pods will read it from here via IAM — the password never lives
# in code, images, or Kubernetes manifests.
# ---------------------------------------------------------------------------
resource "aws_secretsmanager_secret" "db_password" {
  name        = "${var.project_name}-db-password"
  description = "RDS master password for ${var.project_name}"

  # Learning environment: allow immediate deletion on destroy.
  # Production default is a 7-30 day recovery window.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id = aws_secretsmanager_secret.db_password.id
  secret_string = jsonencode({
    username = var.db_username
    password = random_password.db_master.result
    dbname   = var.db_name
  })
}

# ---------------------------------------------------------------------------
# DB subnet group — tells RDS it may ONLY live in the private subnets.
# This is where network isolation stops being theoretical.
# ---------------------------------------------------------------------------
resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db-subnet-group"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "${var.project_name}-db-subnet-group"
  }
}

# ---------------------------------------------------------------------------
# The PostgreSQL instance itself
# ---------------------------------------------------------------------------
resource "aws_db_instance" "main" {
  identifier     = "${var.project_name}-db"
  engine         = "postgres"
  engine_version = "15.7"
  instance_class = "db.t3.micro" # free-tier eligible

  allocated_storage     = 20
  max_allocated_storage = 20
  storage_type          = "gp3"
  storage_encrypted     = true

  db_name  = var.db_name
  username = var.db_username
  password = random_password.db_master.result

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.database.id]

  # CRITICAL: no public endpoint. The database is reachable only from
  # inside the VPC, by resources in the backend security group.
  publicly_accessible = false

  # Learning-environment choices (NOT production defaults):
  skip_final_snapshot     = true # prod: false, so you keep a final backup
  backup_retention_period = 0    # prod: 7+ days
  deletion_protection     = false # prod: true

  tags = {
    Name = "${var.project_name}-db"
  }
}
