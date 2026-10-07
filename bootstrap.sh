#!/bin/bash
set -e

echo "=== Updating kubeconfig ==="
aws eks update-kubeconfig --region us-east-1 --name api-uptime-monitor-cluster

echo "=== Installing monitoring stack ==="
helm install monitoring prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set grafana.adminPassword=admin \
  --set alertmanager.enabled=true \
  --set prometheus.prometheusSpec.retention=7d 2>/dev/null || echo "Monitoring stack already installed"

echo "=== Installing AWS Load Balancer Controller ==="
VPC_ID=$(aws ec2 describe-vpcs --filters "Name=tag:Name,Values=api-uptime-monitor-vpc" --query "Vpcs[0].VpcId" --output text)
ALB_ROLE_ARN=$(cd ~/project-2/infra && terraform output -raw alb_controller_role_arn)

helm repo add eks https://aws.github.io/eks-charts 2>/dev/null || true
helm repo update
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  --namespace kube-system \
  --set clusterName=api-uptime-monitor-cluster \
  --set serviceAccount.create=true \
  --set serviceAccount.name=aws-load-balancer-controller \
  --set "serviceAccount.annotations.eks\.amazonaws\.com/role-arn=$ALB_ROLE_ARN" \
  --set region=us-east-1 \
  --set vpcId=$VPC_ID 2>/dev/null || echo "ALB controller already installed"

echo "=== Loading schema prerequisites ==="
DB_PASSWORD=$(aws secretsmanager get-secret-value \
  --secret-id api-uptime-monitor-db-password \
  --query SecretString --output text | python3 -c "import sys,json; print(json.load(sys.stdin)['password'])")
DB_HOST=$(aws rds describe-db-instances --db-instance-identifier api-uptime-monitor-db \
  --query "DBInstances[0].Endpoint.Address" --output text)

kubectl create configmap schema-sql --from-file=schema.sql 2>/dev/null || true
kubectl delete secret db-credentials 2>/dev/null || true
kubectl create secret generic db-credentials \
  --from-literal=host="$DB_HOST" \
  --from-literal=username=postgres \
  --from-literal=dbname=uptime_monitor \
  --from-literal=password="$DB_PASSWORD"

echo "=== Loading schema ==="
kubectl delete job schema-load 2>/dev/null || true
kubectl apply -f schema-job.yaml

echo "=== Deploying backend ==="
kubectl apply -f backend-deploy.yaml
kubectl apply -f service-monitor.yaml

echo "=== Applying alert rules ==="
kubectl apply -f alert-rules.yaml

echo "=== Applying network policies ==="
kubectl apply -f network-policy.yaml

echo "=== Waiting for pods ==="
kubectl wait --for=condition=ready pod -l app=backend --timeout=120s

echo "=== Waiting for ALB controller ==="
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=aws-load-balancer-controller -n kube-system --timeout=60s

echo "=== Applying Ingress ==="
kubectl apply -f ingress.yaml

echo "=== Done ==="
echo ""
echo "Port-forward commands:"
echo "  Grafana:  kubectl port-forward svc/monitoring-grafana -n monitoring 3000:80 &"
echo "  Backend:  kubectl port-forward svc/backend 8000:80 &"
echo ""
echo "Public URL: https://api.decryptoji.com/health"
echo ""
echo "Check ALB address:"
echo "  kubectl get ingress backend-ingress"
