# ---------------------------------------------------------------------------
# ECR — private container registry for the backend image.
# Lives outside the VPC; EKS pulls from it over AWS's network.
# ---------------------------------------------------------------------------
resource "aws_ecr_repository" "backend" {
  name = "${var.project_name}-backend"

  # IMMUTABLE: a pushed tag can never be overwritten.
  # Guarantees v1 today == v1 next month. No "latest" drift.
  image_tag_mutability = "IMMUTABLE"

  # Scan every pushed image for known CVEs (this is SCA, for free)
  image_scanning_configuration {
    scan_on_push = true
  }

  # Learning environment: allow destroy even with images present.
  force_delete = true

  tags = {
    Name = "${var.project_name}-backend"
  }
}

output "ecr_repository_url" {
  description = "URL of the ECR repository (used for docker push)"
  value       = aws_ecr_repository.backend.repository_url
}
