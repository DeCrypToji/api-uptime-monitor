# API Uptime Monitor — Go Backend

## Setup

### Prerequisites
- Go 1.21+
- PostgreSQL 14+
- AWS CLI (for RDS in production)
- Git

### Local Development Setup

1. **Copy environment file:**
```bash
cp .env.example .env
```

2. **Edit `.env` with local database:**
```bash
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_local_password
DB_NAME=uptime_monitor
JWT_SECRET=your_local_secret_key_here
```

3. **Create local PostgreSQL database:**
```bash
psql -U postgres -c "CREATE DATABASE uptime_monitor;"
```

4. **Load schema:**
```bash
psql -U postgres -d uptime_monitor -f ../schema.sql
```

5. **Install Go dependencies:**
```bash
go mod download
go mod tidy
```

6. **Run locally:**
```bash
go run main.go auth.go
```

Server starts on `http://localhost:8000`

## Endpoints

### Health Check
```
GET /health
```

### Auth (Public)
```
POST /api/v1/auth/signup
POST /api/v1/auth/login
```

### Endpoints (Protected — requires JWT)
```
GET /api/v1/endpoints
POST /api/v1/endpoints
GET /api/v1/endpoints/:id
PATCH /api/v1/endpoints/:id
DELETE /api/v1/endpoints/:id
```

### Health Checks (Protected)
```
GET /api/v1/endpoints/:id/health
```

### Status Pages (Public)
```
GET /api/v1/status/:slug
```

## Project Structure

```
backend/
├── main.go           # Entry point, Gin setup, routing
├── auth.go           # JWT middleware, authentication
├── go.mod            # Module definition
├── .env.example      # Environment template
├── .gitignore        # Git ignore rules
└── README.md         # This file
```

## Development

### Testing
```bash
go test ./...
```

### Run with hot reload (optional)
```bash
go install github.com/cosmtrek/air@latest
air
```

### Format code
```bash
go fmt ./...
```

### Lint
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run
```

## Deployment

### To EKS (Week 1)

1. **Build Docker image:**
```bash
docker build -t 119750096239.dkr.ecr.eu-central-1.amazonaws.com/api-uptime-monitor:latest .
```

2. **Push to ECR:**
```bash
aws ecr get-login-password --region eu-central-1 | docker login --username AWS --password-stdin 119750096239.dkr.ecr.eu-central-1.amazonaws.com
docker push 119750096239.dkr.ecr.eu-central-1.amazonaws.com/api-uptime-monitor:latest
```

3. **Deploy with Terraform** (see ../terraform/)

### Database Migrations

Use the cleanup function defined in schema.sql:

```sql
SELECT cleanup_old_health_checks();
```

Schedule as cron job in Kubernetes:
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: health-check-cleanup
spec:
  schedule: "0 2 * * *" # Daily at 2am UTC
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: cleanup
            image: postgres:14
            command: ["psql", "-h", "$DB_HOST", "-U", "$DB_USER", "-d", "$DB_NAME", "-c", "SELECT cleanup_old_health_checks();"]
            env:
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: password
```

## Notes

### JWT Token Format
```
Authorization: Bearer <jwt_token>
```

### Error Responses
All errors return JSON:
```json
{
  "error": "error message"
}
```

### Rate Limiting
- Free tier: 100 req/min per user
- Starter: same as free
- Pro: 1000 req/min per user

Implement in v1.1 using Redis.

## TODO (Week 2+)

- [ ] Implement signup/login handlers with bcrypt
- [ ] Implement endpoint CRUD with validation
- [ ] Implement health check persistence
- [ ] Implement alert event creation
- [ ] Add request logging middleware
- [ ] Add error handling middleware
- [ ] Add CORS middleware
- [ ] Add rate limiting middleware
- [ ] Add metrics (Prometheus)
- [ ] Add database migrations
- [ ] Add OpenAPI documentation (Swagger)

## Troubleshooting

### "database is locked"
- Make sure only one Go process is running
- Check: `lsof -i :8000`

### "connection refused"
- Database not running: `psql -U postgres`
- Check DB_HOST, DB_PORT in .env

### JWT errors
- Make sure JWT_SECRET is set in .env
- Check token format in Authorization header

## References

- [Gin Documentation](https://gin-gonic.com/)
- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [JWT Go Library](https://github.com/golang-jwt/jwt)
