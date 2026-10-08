#!/bin/bash
set -e

echo "=== Deleting Ingress (triggers ALB cleanup) ==="
kubectl delete ingress backend-ingress 2>/dev/null || true
echo "Waiting 30s for ALB deletion..."
sleep 30

echo "=== Verifying ALB removed ==="
ALB_CHECK=$(aws elbv2 describe-load-balancers --query "LoadBalancers[?contains(DNSName, 'k8s-')].DNSName" --output text)
if [ -n "$ALB_CHECK" ]; then
  echo "WARNING: ALB still exists — cleaning up manually"
  ALB_ARN=$(aws elbv2 describe-load-balancers --query "LoadBalancers[?contains(DNSName, 'k8s-')].LoadBalancerArn" --output text)
  aws elbv2 delete-load-balancer --load-balancer-arn "$ALB_ARN"
  sleep 10
fi

echo "=== Destroying EKS, NAT, RDS, EIP ==="
cd ~/project-2/infra
terraform destroy \
  -target=aws_eks_node_group.main \
  -target=aws_eks_cluster.main \
  -target=aws_nat_gateway.main \
  -target=aws_db_instance.main \
  -target=aws_eip.nat

echo "=== Cleaning up orphaned target groups ==="
for TG_ARN in $(aws elbv2 describe-target-groups --query "TargetGroups[?starts_with(TargetGroupName, 'k8s-')].TargetGroupArn" --output text); do
  echo "Deleting target group: $TG_ARN"
  aws elbv2 delete-target-group --target-group-arn "$TG_ARN"
done

echo "=== Verifying teardown ==="
echo "EKS clusters:"
aws eks list-clusters --output text
echo "NAT gateways:"
aws ec2 describe-nat-gateways --filter "Name=state,Values=available" --query "NatGateways[].NatGatewayId" --output text
echo "RDS instances:"
aws rds describe-db-instances --query "DBInstances[].DBInstanceIdentifier" --output text
echo "Orphaned EIPs:"
aws ec2 describe-addresses --query "Addresses[?AssociationId==null].PublicIp" --output text
echo "Load balancers:"
aws elbv2 describe-load-balancers --query "LoadBalancers[].DNSName" --output text
echo "Target groups:"
aws elbv2 describe-target-groups --query "TargetGroups[?starts_with(TargetGroupName, 'k8s-')].TargetGroupArn" --output text

echo ""
echo "=== Teardown complete ==="
echo "NOTE: S3 (frontend) and CloudFront are still running (pennies/month)."
echo "To destroy those too: cd ~/project-2/infra && terraform destroy -target=aws_cloudfront_distribution.frontend -target=aws_s3_bucket.frontend"
