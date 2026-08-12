# API Uptime Monitor — Design Document (Solo Developer Edition)

## Executive Summary

**API Uptime Monitor** is a freemium SaaS platform for solo developers and indie hackers who need simple, reliable HTTP endpoint monitoring with Slack alerts.

**Market:** Solo developers building APIs ($0 CAC, community-driven growth)  
**MVP timeline:** 4 weeks (simplified for solo devs, not teams)  
**Year 1 goal:** 100 free users, 20-25 paid users, ~$3,900 ARR  

---

## 1. Market & Opportunity

### Market Size (2024-2025 Data)

- **Total uptime monitoring market:** $10.75 billion (2024) → $21.28 billion (2032), 8.45% CAGR
- **Target segment:** Solo developers + indie hackers
  - Estimated 50,000+ developers building APIs
  - Higher retention than teams (mission-critical monitoring)
  - Self-serve adoption (no sales cycle)
  - Community-driven distribution (HackerNews, ProductHunt, Twitter)

### Why Solo Developers First

**Competitive advantage over team-focused tools:**

| Factor | Solo Devs | Teams |
|---|---|---|
| Market size | 50k+ | 5k companies |
| NRR (retention) | 95%+ | 60% |
| Customer acquisition cost | $0 (community) | $1000s (sales) |
| Switching cost | High (mission-critical) | Low (re-evaluate often) |
| Purchase friction | One click | 3-month evaluation |

**Proven by Better Stack:** Started with developer-first positioning, raised $39M, now serving 200k+ users and 4k+ customers. Your advantage: no VC pressure, so you can focus on product quality.

### Pain Points We're Solving

1. **Blind to outages** — developers don't know their API is down until users complain
2. **No proof of reliability** — can't show customers/investors "we're 99.5% up"
3. **Too much complexity** — existing tools (Pingdom, Datadog) are overkill for one person
4. **Too expensive** — Datadog scales cost quickly; UptimeRobot is weak on analytics
5. **Too noisy** — alert fatigue from false positives and one-off failures

---

## 2. Product Strategy (MVP = Solo Dev Focused)

### What Solo Developers Actually Need

**Essential features (MVP):**
- Add 5-10 endpoints to monitor
- 1-minute check interval (fast feedback loop)
- Slack alerts (instant notification, no dashboard login)
- Simple uptime dashboard (one page, readable at a glance)
- Public status page (to embed in README)
- HTTP method + expected status code validation
- 48-hour historical data (see patterns)

**NOT in MVP (complexity kills solo devs):**
- Team permissions/roles
- Advanced alerting rules (solo = one rule: if down, alert)
- Analytics dashboard (metrics they don't need)
- SLA reporting (no customers to report to)
- Multi-region monitoring (overkill for one person)

### Why This Cuts Scope by 60%

Solo dev tools are **5x simpler** than team tools:
- No RBAC system (one user = owner)
- No complex alerting engine (down = alert, up = alert off)
- No compliance reporting (no regulatory requirement)
- No multi-tenant isolation headaches (single user per account)

**Result:** Ship MVP in 4 weeks instead of 4 months.

---

## 3. User Flows

### Solo Developer Happy Path

**Signup → Add endpoint → Get alert → Upgrade**

1. Lands on marketing site → "Know the moment your API goes down. 2 minutes to set up."
2. Clicks "Start free" → GitHub OAuth (no password)
3. Logs in → Empty dashboard: "You have 0 endpoints. Add one to get started."
4. Fills form:
   - URL: `https://api.myapp.com/health`
   - HTTP method: GET
   - Expected status: 200
   - Check interval: 1 minute
5. Clicks "Monitor now"
6. System checks immediately → Dashboard shows: "✅ Healthy (42ms)"
7. Clicks "Connect Slack" → OAuth to their workspace
8. Selects which channel (#alerts or DM)
9. Goes back to coding
10. **At 2:15am** — API database connection fails
11. System detects failure after 1 minute → sends Slack: "🚨 api.myapp.com is DOWN (502 Bad Gateway)"
12. Developer fixes database → API recovers
13. System detects recovery after 1 minute → sends Slack: "✅ api.myapp.com is BACK UP (42ms)"
14. **Next morning** — opens dashboard:
    - Shows "99.2% uptime this week (2 outages)"
    - "Downtime: 12 minutes total (3am, 8:15am)"
    - Incident list with timestamps and duration
15. **After 2 weeks of free usage:**
    - Has 4 endpoints monitored
    - Wants 30-second checks (instead of 1 minute)
    - Wants to monitor 20+ endpoints
    - Clicks upgrade → chooses $9/month "Starter" plan
    - Instant access to 1-minute checks + 10 endpoints
16. **After 3 months** (now at $9/month):
    - Added 8 more endpoints
    - Upgraded to $29/month "Pro" for 50 endpoints
    - Uses webhook to auto-escalate critical failures to PagerDuty
    - Shows status page on website: "uptime.myapp.com"

---

## 4. Pricing (Community-First)

**Philosophy:** Low entry cost, high perceived value

### FREE TIER
- ✓ 2 endpoints
- ✓ 5-minute check interval
- ✓ Slack alerts
- ✓ Basic status page (uptime.decryptoji.com/myapp)
- ✓ Email support
- Cost: $0 forever
- Target: Proof of concept, side projects

### STARTER TIER ($9/month)
- ✓ 10 endpoints
- ✓ 1-minute check intervals
- ✓ Slack + Email alerts
- ✓ Custom status page (myapp.uptime.dev)
- ✓ 30-day historical data
- ✓ Basic analytics (uptime %)
- ✓ Priority email support
- Cost: $9/month
- Target: Solo developers with 3-5 APIs

### PRO TIER ($29/month)
- ✓ 50 endpoints
- ✓ 30-second check intervals
- ✓ Webhooks (custom integrations)
- ✓ PagerDuty integration
- ✓ Uptime badge for README
- ✓ 90-day historical data
- ✓ Advanced analytics (response times, error rates)
- ✓ Slack priority support
- Cost: $29/month
- Target: Indie SaaS founders, small teams

**Why this pricing works:**
- $9 is impulse buy (no approval needed)
- $29 is 30 minutes of freelance work per month (easy justify)
- Free tier has enough to validate before paying
- No surprises (fixed per tier, not per monitor)

---

## 5. Tech Stack (Production-Grade But Simple)

| Layer | Choice | Rationale |
|---|---|---|
| **Frontend** | React on S3 + CloudFront | Full control, works with Go backend, fast iteration |
| **Backend** | Go with Gin framework | Single binary, cloud-native, scales horizontally |
| **Database** | PostgreSQL on RDS | Proven, relational, no upfront schema design |
| **Monitoring jobs** | Go worker (SQS + Lambda) | Simple scheduling, scales with demand |
| **Status pages** | Static (pre-rendered HTML) | Fast, global distribution via CloudFront |
| **Deployment** | Kubernetes on EKS | Multi-AZ, auto-scaling, same patterns as Project 1 |
| **Infrastructure as Code** | Terraform | Destroy/rebuild for cost control, fully reproducible |
| **CI/CD** | GitHub Actions | Built-in, free, used in Project 1 already |

**Why Kubernetes for solo dev tool?**
- Sounds overkill, but isn't
- You learn real DevOps (valuable for portfolio)
- Can scale from 1 user to 1000 users without rearchitecting
- Cost control (destroy during sleep, rebuild during work)
- You already have the patterns from Project 1

---

## 6. Security Requirements (Shift-Left)

### Authentication
- Password: 12+ chars (uppercase + lowercase + number + special)
- Hashing: bcrypt, never plaintext
- JWT tokens: Short-lived (15 min access, 7 day refresh)
- GitHub OAuth: Optional social login (preferred for devs)
- Rate limiting: 5 failed attempts = 15-minute lockout
- Session timeout: 30 days

### Authorization
- RBAC: Solo dev = owner only (no permissions system in MVP)
- Scope: User can only see/edit their own endpoints
- API keys: Can generate one key per endpoint (scoped, revocable)

### Data Protection
- At rest: AWS KMS encryption (default RDS)
- In transit: TLS 1.3 (ACM cert)
- Passwords: Never logged or cached
- API keys: Never returned twice (show once on creation)

### API Security
- Rate limiting: 100 req/min per user, 1000 req/min per API key
- Input validation: URL must be valid HTTP(S), method must be GET/POST/HEAD/PUT/PATCH
- CORS: Only decryptoji.com domains
- CSRF: Token-based (React handles automatically)

### Secrets Management
- Database credentials: AWS Secrets Manager
- Slack bot token: AWS Secrets Manager
- GitHub OAuth secret: AWS Secrets Manager
- Never commit: Pre-commit hook blocks credentials in git

### Monitoring & Alerting
- Failed logins logged (timestamp, IP, email — no passwords)
- Unusual API activity: Alert if 1000+ checks in 1 minute (potential abuse)
- Error logs sanitized: No passwords, tokens, or sensitive data in logs
- Audit trail: Every endpoint add/edit/delete logged with user + timestamp

---

## 7. Database Schema (PostgreSQL)

See `schema.sql` file for full schema.

Key tables:
- **users** — authentication + Slack config
- **endpoints** — what to monitor (URL, method, expected status)
- **health_checks** — results of each check (status, response time, error)
- **alert_events** — alerts sent to user (for history)
- **status_pages** — public pages users can share
- **subscriptions** — billing tier + Stripe info

---

## 8. Go-to-Market (Community-First)

### Launch Phases

**Week 1: ProductHunt**
- Polish landing page
- Create ProductHunt listing
- Get early feedback from makers
- Expected: 200-300 upvotes, 50-100 free signups

**Week 2: HackerNews**
- Write "Show HN: I built an uptime monitor because I got tired of waking up to customer complaints"
- Share journey and code (GitHub repo)
- Expected: 500-1000 views, 20-30 signups

**Week 3: Twitter**
- Thread: "Why I built my own uptime monitor instead of using Pingdom"
- Share pain points, architecture choices, lessons learned
- Tag @IndieHackers, @ProductHunt, relevant DevTools communities
- Expected: 2000+ impressions, 10-15 signups

**Month 1: Indie Developer Communities**
- IndieHackers (Show & Tell)
- DevTools subreddit (/r/devtools)
- Dev.to blogging community
- Sponsor indie dev newsletters (Bytes, JavaScript Weekly)
- Expected: 200-500 signups

**Ongoing: Content**
- Blog post: "How we built an uptime monitor with Go and Kubernetes"
- Share learnings: DevSecOps, multi-region monitoring, PostgreSQL for time-series data
- GitHub repo gets attention (code samples, architecture diagrams)

**Cost:** ~$0 (apart from optional newsletter sponsorships @ $200-500)

---

## 9. Year 1 Metrics & Success Criteria

**You'll know you have PMF when:**

✅ **Month 1:** 50+ free tier signups (community resonance)  
✅ **Month 2:** First 5 paying customers (willingness to pay)  
✅ **Month 3:** 10-15 paying customers (repeatable acquisition)  
✅ **Month 6:** 20+ paying customers ($280/month MRR)  
✅ **NRR >90%:** Customers stay and upgrade (product sticks)  

**If you hit this, you have a business worth scaling.** If not, lessons learned are invaluable.

---

## 10. Implementation Timeline

**Phase 1: Core MVP (Weeks 1-2)**
- Backend: User auth (JWT), endpoint CRUD, basic health checks
- Frontend: Signup, add endpoint, simple dashboard
- Database: Users, endpoints, health_checks tables
- Deploy: Single EKS cluster, RDS Postgres

**Phase 2: Monitoring + Alerts (Weeks 3-4)**
- Lambda for distributed health checks (3 regions)
- Slack integration (OAuth, webhooks)
- Alert logic (status change detection)
- Health check persistence

**Phase 3: Polish + Launch (Week 4-5)**
- Status pages (static HTML generation)
- Landing page + docs
- Free tier limits enforcement
- Error handling + edge cases
- Load testing

**Phase 4: Go-to-Market (Week 5-6)**
- ProductHunt launch
- HackerNews post
- Twitter thread
- Monitor feedback, iterate

---

## Next Steps

1. ✅ Design complete (this document)
2. ⏭️ Create database schema (schema.sql)
3. ⏭️ Scaffold Go backend (main.go, routes)
4. ⏭️ Scaffold React frontend (pages, components)
5. ⏭️ Terraform for EKS + RDS
6. ⏭️ First user signup → should work end-to-end
7. ⏭️ Lambda for health checks
8. ⏭️ Slack integration
9. ⏭️ Polish and launch

---

*Document version: 1.0*  
*Last updated: June 24, 2026*  
*Author: DeCrypToji (Janali Miller-Reid)*

---

## 11. MVP Adjustments (Based on Design Review)

### Free Tier Check Interval
- Free tier: 5-minute checks (not unlimited)
- Starter: 1-minute checks
- Pro: 30-second checks
- **Why:** Drives upgrade motivation without blocking free users

### Slack Integration MVP
- First version: User provides webhook URL manually (copy/paste)
- OAuth comes in v1.1 when you have 10+ paying users
- **Why:** Saves 1-2 weeks of work, manual webhook works great for MVP
- **Timeline:** Day 1-3 (instead of OAuth which is 2-3 weeks)

### Health Check Reliability
- Checks use Step Functions + EventBridge (not bare Lambda)
- 3 retries with 30-second backoff before alerting failure
- If all retries fail: sends "Monitoring service is experiencing issues" alert to user
- **Why:** Lambda cold starts are unpredictable; Step Functions are more reliable
- **Guarantees:** <2 minute alert time (99% of cases)

### Single Region MVP
- Launch in us-east-1 only
- Multi-region is v1.1 feature (when customers request it)
- **Why:** Reduces infrastructure complexity, Lambda costs, scheduling headaches
- **Timeline:** Saves 1 week of infrastructure work

### Tier Limits (Prevent Abuse)

**Free Tier:**
- 2 endpoints
- 1,000 API requests/day
- 5-minute check interval
- 24-hour historical data

**Starter Tier ($9/month):**
- 10 endpoints
- 10,000 API requests/day
- 1-minute check interval
- 30-day historical data

**Pro Tier ($29/month):**
- 50 endpoints
- Unlimited API requests
- 30-second check interval
- 90-day historical data

**Why:** Prevents abuse, drives upgrade to starter, clear value tiers

### Data Retention Job
- Cron job: Runs daily at 2am UTC
- Deletes `health_checks` records older than retention period
  - Free tier: older than 24 hours
  - Starter: older than 30 days
  - Pro: older than 90 days
- **Why:** Prevents unbounded database growth, keeps queries fast
- **Cost impact:** Saves ~$50-100/month on RDS storage

---

## 12. Implementation Priority (Weeks 1-6)

### Week 1: Core Infrastructure
- [ ] EKS cluster (Terraform from Project 1 patterns)
- [ ] RDS PostgreSQL
- [ ] GitHub Actions CI/CD
- [ ] Go project scaffold (Gin routes)
- [ ] React project scaffold

### Week 2: Authentication + Basic API
- [ ] User signup/login (JWT)
- [ ] Endpoint CRUD endpoints
- [ ] Dashboard basic layout
- [ ] Manual webhook URL input (no OAuth yet)

### Week 3: Health Checks + Alerts
- [ ] Step Functions + EventBridge setup
- [ ] Health check Lambda
- [ ] Alert persistence (alert_events table)
- [ ] Slack webhook integration (manual URL)

### Week 4: Status Pages + Polish
- [ ] Status page generation (static HTML)
- [ ] Uptime calculation
- [ ] Frontend forms validation
- [ ] Error handling

### Week 5: Testing + Hardening
- [ ] Load testing (100s of concurrent checks)
- [ ] Failure scenarios (Lambda timeout, DB down, Slack offline)
- [ ] Security audit (secrets, auth, rate limiting)
- [ ] Data retention cleanup job

### Week 6: Launch Prep
- [ ] Landing page
- [ ] Documentation + setup guide
- [ ] ProductHunt listing draft
- [ ] Deploy to production
- [ ] Monitor for bugs

---

## 13. Success Definition (Checkpoint Milestones)

**End of Week 2:** You can signup, add endpoint, see dashboard  
**End of Week 3:** You get Slack alert when endpoint goes down  
**End of Week 4:** Status page works, uptime % shows correctly  
**End of Week 5:** All failure modes handled, no crashes  
**End of Week 6:** ProductHunt launch, first 50 signups  

**Month 2 Goal:** 5 paying customers at $9/month = $45/month  
**Month 3 Goal:** 10-15 paying customers = $90-135/month  
**Month 6 Goal:** 20+ paying customers = $180+ /month = $2,160 ARR  

If you don't hit these, pause and ask: "What's blocking conversions?" Then pivot based on real user feedback.
