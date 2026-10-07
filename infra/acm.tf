# ---------------------------------------------------------------------------
# ACM certificate for api.decryptoji.com
# Validated via DNS — Route 53 proves domain ownership automatically.
# ---------------------------------------------------------------------------
resource "aws_acm_certificate" "api" {
  domain_name       = "api.decryptoji.com"
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

# Create the DNS validation record in Route 53
resource "aws_route53_record" "cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.api.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  zone_id = var.route53_zone_id
  name    = each.value.name
  type    = each.value.type
  records = [each.value.record]
  ttl     = 60

  allow_overwrite = true
}

# Wait for the certificate to be validated
resource "aws_acm_certificate_validation" "api" {
  certificate_arn         = aws_acm_certificate.api.arn
  validation_record_fqdns = [for record in aws_route53_record.cert_validation : record.fqdn]
}
