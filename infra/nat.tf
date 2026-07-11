# ---------------------------------------------------------------------------
# Elastic IP for the NAT Gateway.
# A NAT needs a static public IP to masquerade outbound traffic behind.
# ---------------------------------------------------------------------------
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.project_name}-nat-eip"
  }
}

# ---------------------------------------------------------------------------
# NAT Gateway — the one-way valve.
# It lives in a PUBLIC subnet (it needs internet access itself),
# but serves the PRIVATE subnets: outbound-initiated connections only.
# Return traffic is permitted; nothing external can initiate inward.
# ~$32/month while running.
# ---------------------------------------------------------------------------
resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id

  depends_on = [aws_internet_gateway.main]

  tags = {
    Name = "${var.project_name}-nat"
  }
}

# ---------------------------------------------------------------------------
# Private route table — 0.0.0.0/0 → NAT (not the IGW directly).
# THIS is what gives private subnets outbound-only internet access.
# Before this existed, the private subnets had no route table at all.
# ---------------------------------------------------------------------------
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main.id
  }

  tags = {
    Name = "${var.project_name}-private-rt"
  }
}

resource "aws_route_table_association" "private" {
  count          = 2
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}
