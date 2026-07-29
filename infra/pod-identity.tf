# ---------------------------------------------------------------------------
# PIECE 2+3: IAM role the backend pod assumes.
#
# TRUST POLICY (piece 3) — WHO may assume this role.
# Trusts the EKS Pod Identity service, and nothing else. This is the "can I
# become this identity?" gate. Without it, assumption is denied at the door
# and the permission policy below never even comes into play.
# ---------------------------------------------------------------------------
resource "aws_iam_role" "backend_pod" {
  name = "${var.project_name}-backend-pod-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "pods.eks.amazonaws.com" }
      Action = [
        "sts:AssumeRole",
        "sts:TagSession"
      ]
    }]
  })
}

# ---------------------------------------------------------------------------
# PERMISSION POLICY (piece 2) — WHAT the role can do once assumed.
# Scoped to reading exactly ONE secret: the DB password. Nothing else.
# This is the "what can I do now that I'm this identity?" gate.
# Least privilege: the backend can read its own DB password and nothing more.
# ---------------------------------------------------------------------------
resource "aws_iam_role_policy" "backend_secrets" {
  name = "${var.project_name}-backend-secrets-read"
  role = aws_iam_role.backend_pod.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = aws_secretsmanager_secret.db_password.arn
    }]
  })
}

# ---------------------------------------------------------------------------
# PIECE 4: the association — wires ServiceAccount -> IAM role.
# "Pods using ServiceAccount 'backend-sa' in namespace 'default' get this role."
# Before this, the role and the ServiceAccount exist independently.
# ---------------------------------------------------------------------------
resource "aws_eks_pod_identity_association" "backend" {
  cluster_name    = aws_eks_cluster.main.name
  namespace       = "default"
  service_account = "backend-sa"
  role_arn        = aws_iam_role.backend_pod.arn
}
