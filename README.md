# API Uptime Monitor

A cloud-native API uptime monitoring service for solo developers. Register HTTP endpoints, get automated health checks on a schedule, and receive edge-triggered Slack alerts when an endpoint changes state (up→down or down→up). Includes a live dashboard for status and manual checks.

Built as a production-grade portfolio project demonstrating end-to-end cloud deployment, infrastructure as code, identity-based security, and DevSecOps practices.

## Tech Stack

**Application:** Go (Gin), React/TypeScript (Vite), PostgreSQL
**Infrastructure:** AWS (EKS, RDS, ECR, Secrets Manager, VPC), Terraform
**Security:** EKS Pod Identity, IAM least-privilege, TLS-enforced DB, identity-based security groups
**CI/CD:** GitHub Actions — govulncheck (reachability-gated), Trivy (informational SCA)
**Containers:** Docker (multi-stage, Alpine-based), Kubernetes (Deployments, Services, Jobs, ServiceAccounts)

## Architecture

```
                    ┌──────────────────────────────────────────┐
                    │              AWS VPC (10.0.0.0/16)       │
                    │                                          │
                    │   ┌─── Private Subnets ───────────────┐  │
                    │   │                                   │  │
                    │   │   EKS Cluster (K8s 1.31)          │  │
                    │   │   ┌─────────────┐ ┌────────────┐  │  │
                    │   │   │ API Pods (N) │ │ Scheduler  │  │  │
                    │   │   │ RUN_SCHED=  │ │ Pod (1)    │  │  │
                    │   │   │  false      │ │ RUN_SCHED= │  │  │
                    │   │   │             │ │  true      │  │  │
                    │   │   └──────┬──────┘ └─────┬──────┘  │  │
                    │   │          │    Pod Identity│         │  │
                    │   │          │   (backend-sa) │         │  │
                    │   │          └───────┬────────┘         │  │
                    │   │                  │ TLS (require)    │  │
                    │   │          ┌───────▼────────┐         │  │
                    │   │          │  RDS Postgres  │         │  │
                    │   │          │  (encrypted)   │         │  │
                    │   │          └────────────────┘         │  │
                    │   └───────────────────────────────────┘  │
                    │                                          │
                    │   Secrets Manager ◄── Pod Identity ──►   │
                    │   (DB password,        (scoped IAM,      │
                    │    single source)       least privilege)  │
                    └──────────────────────────────────────────┘
```

**Key design decisions:**

- **API/Scheduler split** — the same image runs as two Deployments: the API scales to N replicas for availability (scheduler disabled), while a single scheduler pod runs health checks (preventing duplicate checks and alerts). Differentiated by the `RUN_SCHEDULER` env var.
- **Pod Identity over K8s Secrets** — the backend fetches the DB password directly from Secrets Manager at startup via a scoped IAM role, so the password exists in one place (Secrets Manager), never duplicated into cluster storage.
- **Identity-based security** — security groups reference other security groups (not IP ranges); pod credentials are scoped and temporary (Pod Identity); the CI pipeline will use OIDC federation (no stored keys). No long-lived static secrets anywhere.
- **Three-tier network isolation** — public subnets (ALB/NAT), private subnets (EKS nodes, RDS). The database has no public endpoint and no internet route; reachable only from the backend security group.

## CI Pipeline

The pipeline runs on every push to `main` and every PR. Security scanning uses a **two-tool strategy** gating on reachability, not mere presence:

| Job | Tool | Purpose | Blocking? |
|---|---|---|---|
| `backend` | Go 1.25 | Build, test, format check | Yes |
| `frontend` | Node 18 | npm install, build | Yes |
| `vuln-reachable` | govulncheck | Fails if code **calls** a vulnerable function | **Yes — the real gate** |
| `scan-image` | Trivy | Reports CVEs present in the container image | No — informational |

**Why this design:** gating on presence (Trivy alone) breaks the pipeline every time the vulnerability database updates with CVEs in unreachable subpackages your code never calls. Gating on reachability (govulncheck) means a red pipeline signals a genuine, exploitable risk — keeping the blocking signal meaningful rather than training developers to ignore it.

## Project Structure

```
├── main.go                  # Entry point, DB init, Secrets Manager fetch, routes
├── auth.go                  # JWT authentication (bcrypt, signed tokens)
├── handlers.go              # Endpoint CRUD handlers
├── health_check.go          # HTTP health-check engine
├── scheduler.go             # Background check loop (gated by RUN_SCHEDULER)
├── alerts.go                # Edge-triggered Slack alerting
├── schema.sql               # PostgreSQL schema (8 tables)
├── Dockerfile               # Multi-stage build (golang:1.25-alpine → alpine)
├── backend-deploy.yaml      # K8s: ServiceAccount, API + Scheduler Deployments, Service
├── schema-job.yaml          # K8s Job: loads schema into RDS
├── infra/                   # Terraform (VPC, RDS, ECR, EKS, Pod Identity, NAT)
│   ├── network.tf
│   ├── database.tf
│   ├── ecr.tf
│   ├── eks.tf
│   ├── nat.tf
│   ├── pod-identity.tf
│   ├── provider.tf
│   ├── variables.tf
│   └── versions.tf
├── frontend/                # React/TypeScript dashboard (Vite)
├── .github/workflows/
│   └── ci.yaml              # CI pipeline (govulncheck gate + Trivy informational)
└── docs/
    ├── ARCHITECTURE_GUIDE.md
    ├── BUILD_LOG.md
    ├── BACKEND_README.md
    ├── DECISIONS.md
    └── DESIGN.md
```

## Local Development

```bash
# Prerequisites: Go 1.25+, Docker, PostgreSQL (local or containerised)

# Clone and set up
git clone https://github.com/DeCrypToji/api-uptime-monitor.git
cd api-uptime-monitor

# Create a .env file for local config
cat > .env << 'EOF'
DB_USER=postgres
DB_PASSWORD=your_local_password
DB_NAME=uptime_monitor
DB_HOST=localhost
DB_PORT=5432
DB_SSLMODE=disable
PORT=8000
EOF

# Load the schema
psql -U postgres -d uptime_monitor -f schema.sql

# Run the backend
go build -o api-uptime-monitor .
./api-uptime-monitor

# Frontend (separate terminal)
cd frontend && npm install && npm run dev
```

The app defaults to secure settings (`DB_SSLMODE=require`, scheduler enabled). Local development overrides these via `.env` — the same binary, configured by environment.

## Cloud Deployment

Infrastructure is managed entirely via Terraform and rebuilds from code in ~25 minutes:

```bash
cd infra
terraform apply          # provisions VPC, RDS, ECR, EKS, Pod Identity, NAT

# Build and push the backend image
aws ecr get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin <account>.dkr.ecr.us-east-1.amazonaws.com
docker build -t api-uptime-monitor-backend:v2 .
docker tag api-uptime-monitor-backend:v2 <account>.dkr.ecr.us-east-1.amazonaws.com/api-uptime-monitor-backend:v2
docker push <account>.dkr.ecr.us-east-1.amazonaws.com/api-uptime-monitor-backend:v2

# Deploy to EKS
aws eks update-kubeconfig --region us-east-1 --name api-uptime-monitor-cluster
kubectl apply -f schema-job.yaml      # load schema into fresh RDS
kubectl apply -f backend-deploy.yaml  # deploy API + scheduler + service
```

## Documentation

- **[Architecture Guide](docs/ARCHITECTURE_GUIDE.md)** — detailed system design, data flows, component breakdown, security model
- **[Build Log](docs/BUILD_LOG.md)** — error postmortems, debugging decisions, root-cause analyses
- **[Backend Reference](docs/BACKEND_README.md)** — API endpoints, auth flow, handler details
- **[Design Decisions](docs/DECISIONS.md)** — architectural choices and rationale
- **[Design Document](docs/DESIGN.md)** — original design specification

## Current Status

**Working and deployed:**
- Backend live on EKS (Pod Identity proven end-to-end, TLS-enforced DB connection)
- API/Scheduler split architecture (scalable API, singular scheduler)
- CI pipeline with govulncheck reachability gate and Trivy informational scanning
- 0 reachable vulnerabilities (govulncheck clean)

**In progress:**
- CD pipeline (GitHub OIDC federation — no stored keys)
- Public exposure (Ingress/ALB, Route 53, ACM)
- Observability (Prometheus, Grafana, Alertmanager)
- Frontend cloud deployment (S3 + CloudFront)
