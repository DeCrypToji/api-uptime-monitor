# ---------------------------------------------------------------------------
# IAM ROLE 1: the CLUSTER role.
# What the EKS control plane may do on your behalf.
# ---------------------------------------------------------------------------
resource "aws_iam_role" "cluster" {
  name = "${var.project_name}-eks-cluster-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "cluster_policy" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# ---------------------------------------------------------------------------
# IAM ROLE 2: the NODE role. A DIFFERENT identity with DIFFERENT permissions.
# What the worker EC2 instances may do.
# ---------------------------------------------------------------------------
resource "aws_iam_role" "node" {
  name = "${var.project_name}-eks-node-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

# Join the cluster
resource "aws_iam_role_policy_attachment" "node_worker" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy"
}

# Pod networking (VPC CNI)
resource "aws_iam_role_policy_attachment" "node_cni" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy"
}

# AUTHORIZATION to pull images. (The NAT provides REACHABILITY. Both required.)
resource "aws_iam_role_policy_attachment" "node_ecr" {
  role       = aws_iam_role.node.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}

# ---------------------------------------------------------------------------
# The EKS cluster (managed control plane).
# ~$0.10/hour. Takes 15-20 minutes to provision.
# ---------------------------------------------------------------------------
resource "aws_eks_cluster" "main" {
  name     = "${var.project_name}-cluster"
  role_arn = aws_iam_role.cluster.arn
  version  = "1.31"

  vpc_config {
    # Control plane ENIs span both private subnets
    subnet_ids              = aws_subnet.private[*].id
    endpoint_private_access = true
    endpoint_public_access  = true # so YOUR kubectl can reach it
  }

  depends_on = [aws_iam_role_policy_attachment.cluster_policy]

  tags = {
    Name = "${var.project_name}-cluster"
  }
}

# ---------------------------------------------------------------------------
# Managed node group — the EC2 workers that run your pods.
# PRIVATE subnets: no public IPs, outbound via NAT only.
# ---------------------------------------------------------------------------
resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "${var.project_name}-nodes"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = aws_subnet.private[*].id

  launch_template {
    id      = aws_launch_template.node.id
    version = aws_launch_template.node.latest_version
  }


  scaling_config {
    desired_size = 2
    min_size     = 1
    max_size     = 3
  }

  depends_on = [
    aws_iam_role_policy_attachment.node_worker,
    aws_iam_role_policy_attachment.node_cni,
    aws_iam_role_policy_attachment.node_ecr,
  ]

  tags = {
    Name = "${var.project_name}-nodes"
  }
}

output "cluster_name" {
  value = aws_eks_cluster.main.name
}

output "cluster_endpoint" {
  value = aws_eks_cluster.main.endpoint
}

# ---------------------------------------------------------------------------
# Launch template — the blueprint every node is built from.
# We route the node group through this so we can attach backend-sg,
# letting nodes (and the pods on them) reach RDS.
#
# CRITICAL: launch template security groups REPLACE the defaults EKS would
# attach — they don't add to them. So we MUST explicitly include the cluster
# security group, or nodes lose control-plane connectivity and go NotReady.
# Three paths wired here: backend-sg (RDS) + cluster SG (control plane).
# Outbound-to-internet is handled separately by the NAT route.
# ---------------------------------------------------------------------------
resource "aws_launch_template" "node" {
  instance_type = "t3.small"
  name_prefix = "${var.project_name}-node-"

  vpc_security_group_ids = [
    aws_security_group.backend.id,
    aws_eks_cluster.main.vpc_config[0].cluster_security_group_id,
  ]

  tag_specifications {
    resource_type = "instance"
    tags = {
      Name = "${var.project_name}-node"
    }
  }
}

# ---------------------------------------------------------------------------
# PIECE 1: Pod Identity Agent add-on.
# Runs as a DaemonSet (one per node). It's the broker: when a pod needs AWS
# credentials, this agent checks the association and hands back temporary,
# scoped credentials. Nothing else in Pod Identity works without it.
# ---------------------------------------------------------------------------
resource "aws_eks_addon" "pod_identity" {
  cluster_name  = aws_eks_cluster.main.name
  addon_name    = "eks-pod-identity-agent"
  addon_version = "v1.3.0-eksbuild.1"
}
