variable "project_name" {
  description = "Name prefix for all resources"
  type        = string
  default     = "api-uptime-monitor"
}

variable "aws_region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "us-east-1"
}

variable "db_name" {
  description = "Name of the PostgreSQL database"
  type        = string
  default     = "uptime_monitor"
}

variable "db_username" {
  description = "RDS master username"
  type        = string
  default     = "postgres"
}
