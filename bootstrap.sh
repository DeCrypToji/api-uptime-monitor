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
  --set prometheus.prometheusSpec.retention=7d

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

echo "=== Waiting for pods ==="
kubectl wait --for=condition=ready pod -l app=backend --timeout=120s

echo "=== Done. Port-forward commands: ==="
echo "Grafana:  kubectl port-forward svc/monitoring-grafana -n monitoring 3000:80 &"
echo "Backend:  kubectl port-forward svc/backend 8000:80 &"
