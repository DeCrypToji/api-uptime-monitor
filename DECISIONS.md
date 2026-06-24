# Key Decisions (June 24, 2026)

## Market: Solo Developers First
- Why: 50k+ developers, $0 CAC, high NRR
- Not: Teams (5k companies, needs sales)

## MVP: No Slack OAuth (Use Manual Webhook)
- Why: Saves 2-3 weeks, manual webhook works for MVP
- Add OAuth in v1.1 when you have 10+ customers

## Single Region (us-east-1 Only)
- Why: Simplifies Lambda scheduling, reduces cost
- Multi-region when customers ask

## Free Tier: 2 Endpoints + 5-Min Checks
- Why: Drives upgrade to $9 starter tier
- Not: 50 free endpoints (UptimeRobot does this, but established)

## Timeline: 6 Weeks (Not 4)
- Conservative: Code, test, deploy, fix
- First time building Go in production

## Infrastructure: EKS (Overkill But Worth It)
- Why: Same patterns as Project 1
- Value: Learn Kubernetes → contract work
