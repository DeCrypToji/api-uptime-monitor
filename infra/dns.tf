# ---------------------------------------------------------------------------
# Route 53 alias record — points api.decryptoji.com at the ALB.
# The ALB is created by the AWS Load Balancer Controller when the Ingress
# resource is applied, so this record is added after the ALB exists.
# ---------------------------------------------------------------------------
# NOTE: The ALB DNS name will be populated after the Ingress is created.
# We'll use a data source to look it up, or add it manually.
# For now, the ACM cert and validation are the Terraform-managed pieces.
