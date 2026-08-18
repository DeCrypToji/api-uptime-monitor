# ---------------------------------------------------------------------------
# PIECE 1: Register GitHub as an OIDC identity provider in AWS.
# This tells AWS: "GitHub's token service is a trusted identity source."
# Without this, AWS has no idea GitHub exists as an identity provider.
# ---------------------------------------------------------------------------
resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"]
}

# ---------------------------------------------------------------------------
# PIECE 2 + 3: IAM role the pipeline assumes.
#
# TRUST POLICY (piece 3): WHO can assume this role.
# Only GitHub Actions, and ONLY for the specific app repo on the main branch.
# A different repo, a fork, or a different branch = denied.
# ---------------------------------------------------------------------------
resource "aws_iam_role" "github_actions" {
  name = "${var.project_name}-github-actions-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Federated = aws_iam_openid_connect_provider.github.arn
      }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = "repo:DeCrypToji/api-uptime-monitor:*"
        }
      }
    }]
  })
}

# ---------------------------------------------------------------------------
# PIECE 4: Permission policy — WHAT the role can do once assumed.
# Scoped to ECR operations on the specific repository only.
# The pipeline can authenticate to ECR and push/pull images. Nothing else.
# ---------------------------------------------------------------------------
resource "aws_iam_role_policy" "github_actions_ecr" {
  name = "${var.project_name}-github-actions-ecr"
  role = aws_iam_role.github_actions.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ecr:GetAuthorizationToken"
        ]
        Resource = "*"  # GetAuthorizationToken doesn't support resource-level scoping
      },
      {
        Effect = "Allow"
        Action = [
          "ecr:BatchCheckLayerAvailability",
          "ecr:GetDownloadUrlForLayer",
          "ecr:BatchGetImage",
          "ecr:PutImage",
          "ecr:InitiateLayerUpload",
          "ecr:UploadLayerPart",
          "ecr:CompleteLayerUpload"
        ]
        Resource = aws_ecr_repository.backend.arn
      }
    ]
  })
}
