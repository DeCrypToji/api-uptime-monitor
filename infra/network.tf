# ---------------------------------------------------------------------------
# VPC — the private network that contains everything
# ---------------------------------------------------------------------------
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${var.project_name}-vpc"
  }
}

# ---------------------------------------------------------------------------
# Internet Gateway — the door to the public internet (for public subnets)
# ---------------------------------------------------------------------------
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.project_name}-igw"
  }
}

# ---------------------------------------------------------------------------
# Availability Zones — fetch the AZs available in this region
# ---------------------------------------------------------------------------
data "aws_availability_zones" "available" {
  state = "available"
}

# ---------------------------------------------------------------------------
# Public subnets (2 AZs) — route to internet; hold ALB + EKS nodes
# ---------------------------------------------------------------------------
resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.${count.index + 1}.0/24"
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.project_name}-public-${count.index + 1}"
    # EKS needs this tag to know it can place internet-facing load balancers here
    "kubernetes.io/role/elb" = "1"
  }
}

# ---------------------------------------------------------------------------
# Private subnets (2 AZs) — NO internet route; hold RDS database
# ---------------------------------------------------------------------------
resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "${var.project_name}-private-${count.index + 1}"
    # EKS needs this tag to know it can place internal load balancers here
    "kubernetes.io/role/internal-elb" = "1"
  }
}

# ---------------------------------------------------------------------------
# Public route table — sends 0.0.0.0/0 (all internet traffic) to the IGW
# This route is what MAKES the public subnets public.
# ---------------------------------------------------------------------------
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "${var.project_name}-public-rt"
  }
}

# Associate both public subnets with the public route table
resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Note: private subnets get NO route table with an internet route.
# Their absence of an internet route is exactly what keeps them private.

# ---------------------------------------------------------------------------
# Security group: BACKEND — allows inbound HTTP, outbound anywhere
# ---------------------------------------------------------------------------
resource "aws_security_group" "backend" {
  name        = "${var.project_name}-backend-sg"
  description = "Backend pods: allow HTTP in, all out"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP from load balancer"
    from_port   = 8000
    to_port     = 8000
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-backend-sg"
  }
}

# ---------------------------------------------------------------------------
# Security group: DATABASE — allows inbound ONLY from the backend SG, port 5432
# This is the rule that enforces "only the backend talks to the database."
# ---------------------------------------------------------------------------
resource "aws_security_group" "database" {
  name        = "${var.project_name}-database-sg"
  description = "RDS: allow Postgres ONLY from backend SG"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Postgres from backend only"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.backend.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-database-sg"
  }
}
