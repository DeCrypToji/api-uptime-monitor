# API Uptime Monitor

A cloud-native API uptime monitoring service for solo developers. Register HTTP endpoints, get automated health checks on a schedule, and receive edge-triggered Slack alerts when an endpoint changes state (up→down or down→up). Includes a live dashboard for status and manual checks.

Built as a production-grade portfolio project demonstrating end-to-end cloud deployment, infrastructure as code, identity-based security, and DevSecOps practices.

## Tech Stack

**Application:** Go (Gin), React/TypeScript (Vite), PostgreSQL
**Infrastructure:** AWS (EKS, RDS, ECR, Secrets Manager, VPC), Terraform
**Security:** Pod Identity, non-root containers, Network Policies, security headers, rate limiting, SAST + SCA + DAST
**CI/CD:** GitHub Actions (9-job security pipeline with DAST, OIDC federation to AWS), ArgoCD (GitOps, two-repo, self-healing)
**Observability:** Prometheus, Grafana, Alertmanager (custom application metrics + alert rules)
**Containers:** Docker (multi-stage, Alpine-based, non-root), Kubernetes (Deployments, Services, Jobs, ServiceAccounts, NetworkPolicies)

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

## CI/CD Pipeline

The pipeline runs on every push to `main` and every PR. Security scanning uses a **two-tool strategy** gating on reachability, not mere presence. Deployment uses OIDC federation — no stored AWS credentials.

| Job | Tool | Purpose | Blocking? |
|---|---|---|---|
| `backend` | Go 1.25 | Build, test, format check | Yes |
| `frontend` | Node 18 | npm install, build | Yes |
| `vuln-reachable` | govulncheck | Fails if code **calls** a vulnerable function | **Yes — the real gate** |
| `scan-image` | Trivy | Reports CVEs present in the container image | No — informational |
| `lint-docker` | hadolint | Dockerfile best practices | Yes |
| `secrets-scan` | Gitleaks | Detects committed secrets in git history | Yes |
| `sast-go` | gosec | Static analysis for Go security bugs | Yes |
| `scan-iac` | Trivy config | Terraform misconfiguration detection | Yes |
| `dast` | OWASP ZAP | Dynamic security testing against running app | No — informational |
| `deploy` | OIDC + ECR | Builds image, pushes to ECR, updates config repo | Only on main |

**Why this design:** gating on presence (Trivy alone) breaks the pipeline every time the vulnerability database updates with CVEs in unreachable subpackages your code never calls. Gating on reachability (govulncheck) means a red pipeline signals a genuine, exploitable risk — keeping the blocking signal meaningful rather than training developers to ignore it.

**CD flow:** on push to main, the deploy job authenticates to AWS via OIDC (temporary credentials, no stored keys), pushes the image to ECR with a commit-SHA tag, and updates the config repo. ArgoCD watches the config repo and auto-syncs the cluster — automated sync, self-healing, drift correction.

## Project Structure

```
├── main.go                  # Entry point, DB init, Secrets Manager fetch, routes, Prometheus metrics
├── auth.go                  # JWT authentication (bcrypt, signed tokens)
├── handlers.go              # Endpoint CRUD handlers
├── health_check.go          # HTTP health-check engine + health check metrics
├── scheduler.go             # Background check loop (gated by RUN_SCHEDULER) + scheduler metrics
├── alerts.go                # Edge-triggered Slack alerting
├── schema.sql               # PostgreSQL schema (8 tables)
├── Dockerfile               # Multi-stage build (golang:1.25-alpine → alpine, non-root user)
├── .dockerignore            # Excludes .git, frontend, infra, node_modules from build context
├── backend-deploy.yaml      # K8s: ServiceAccount, API + Scheduler Deployments, Services
├── schema-job.yaml          # K8s Job: loads schema into RDS (PGPASSWORD, not URI)
├── service-monitor.yaml     # Prometheus ServiceMonitor for API + scheduler scraping
├── alert-rules.yaml         # Prometheus alert rules (CrashLoopBackOff, error rate, scheduler)
├── network-policy.yaml      # K8s NetworkPolicies (egress/ingress restrictions)
├── docker-compose.ci.yaml   # Docker Compose for DAST scanning in CI
├── bootstrap.sh             # Post-rebuild automation (monitoring, schema, deploy)
├── infra/                   # Terraform (VPC, RDS, ECR, EKS, Pod Identity, NAT, OIDC)
│   ├── network.tf
│   ├── database.tf
│   ├── ecr.tf
│   ├── eks.tf
│   ├── nat.tf
│   ├── pod-identity.tf
│   ├── oidc.tf
│   ├── provider.tf
│   ├── variables.tf
│   └── versions.tf
├── monitoring/
│   └── dashboard.json       # Exported Grafana dashboard (4 custom panels)
├── frontend/                # React/TypeScript dashboard (Vite)
├── .github/workflows/
│   └── ci.yaml              # CI/CD pipeline (9 jobs + OIDC deploy)
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

Infrastructure is managed entirely via Terraform. Post-provisioning setup is automated via `bootstrap.sh`:

```bash
cd infra
terraform apply              # provisions VPC, RDS, ECR, EKS, Pod Identity, NAT, OIDC (~25 min)

cd ..
./bootstrap.sh               # installs monitoring stack, loads schema, deploys backend
```

In production, deployment is fully automated: push code → CI pipeline runs 9 security jobs → OIDC-authenticated ECR push → config repo updated → ArgoCD auto-deploys. No manual `kubectl apply` or image tagging required.

## Documentation

- **[Architecture Guide](docs/ARCHITECTURE_GUIDE.md)** — detailed system design, data flows, component breakdown, security model
- **[Build Log](docs/BUILD_LOG.md)** — error postmortems, debugging decisions, root-cause analyses
- **[Backend Reference](docs/BACKEND_README.md)** — API endpoints, auth flow, handler details
- **[Design Decisions](docs/DECISIONS.md)** — architectural choices and rationale
- **[Design Document](docs/DESIGN.md)** — original design specification

## Current Status

**Working and deployed:**
- Backend live on EKS (Pod Identity proven end-to-end, TLS-enforced DB connection)
- API/Scheduler split architecture (scalable API tier, singular scheduler from one image)
- CI pipeline: 9 jobs — govulncheck reachability gate, Trivy informational SCA, hadolint, gosec, Gitleaks, Trivy IaC, DAST (OWASP ZAP)
- CD pipeline: OIDC-authenticated ECR push (no stored AWS keys) + config repo update
- GitOps: ArgoCD with two-repo architecture, automated sync, self-healing, drift correction
- 0 reachable vulnerabilities (govulncheck clean)
- Observability: Prometheus + Grafana + Alertmanager with custom application metrics (request rate, latency, scheduler runs, health check results), custom dashboard, and alert rules (CrashLoopBackOff, high error rate, scheduler-not-running, health check failure rate)
- Security hardening: non-root container (readOnlyRootFilesystem, allowPrivilegeEscalation: false), Network Policies (egress/ingress restricted), security response headers (HSTS, CSP, X-Frame-Options), rate limiting on auth endpoints, .dockerignore (828MB → ~1MB build context)

**In progress:**
- Public exposure (Ingress/ALB, Route 53, ACM)
- Frontend cloud deployment (S3 + CloudFront)
- lib/pq → pgx migration (unmaintained driver with unfixable CVEs)
