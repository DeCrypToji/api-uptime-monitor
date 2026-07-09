# API Uptime Monitor — Build Log & Error Postmortem

**Project:** API Uptime Monitor (Solo Developer Edition)
**Owner:** Janali Miller-Reid (DeCrypToji)
**Stack:** Go (Gin) · React + TypeScript (Vite) · PostgreSQL · Docker · JWT · GitHub Actions
**Scope of this document:** Everything built through Week 1 and Phase 2 Day 1, plus a
complete, honest breakdown of every error encountered and how each was resolved.

This document exists so the build can be **explained and defended**, not just demoed.
Where an error was caused by a bad process decision (not just a typo), that is stated
plainly — because the lesson is the valuable part.

---

## Part 1 — What Has Been Built

### 1.1 Week 1 (Foundation) — Complete

| Area | What exists | Files |
|---|---|---|
| Database | 8-table schema with constraints, indexes, views, triggers | `schema.sql` |
| Auth | Signup, login, logout, JWT issue + middleware, bcrypt hashing | `auth.go`, `handlers.go` |
| Endpoints | Full CRUD (create, list, get one, update, soft-delete) | `handlers.go` |
| Server | Gin router, route table, DB init + connection pooling | `main.go` |
| Frontend | Login, Signup, Dashboard, ForgotPassword (placeholder) | `frontend/src/...` |
| CI/CD | GitHub Actions: backend test/build, frontend build, Docker build | `.github/workflows/ci.yml` |
| Docs | Product spec, decisions log, architecture guide | `DESIGN.md`, `DECISIONS.md`, `ARCHITECTURE_GUIDE.md` |

**End-to-end verified in Week 1:** a user can sign up, log in, add endpoints, and list
them. CI passes green. No secrets committed (`.env` gitignored, `.env.example` holds
placeholders only).

### 1.2 Phase 2 Day 1 (Health Checks) — Complete

| Capability | Detail |
|---|---|
| Perform a check | Real HTTP request to the endpoint URL, 10-second timeout |
| Measure | Response time in ms, returned status code |
| Decide | `is_healthy = (status_code == expected_status_code)` |
| Record history | INSERT into `health_checks` (append-only time-series) |
| Update snapshot | UPDATE `endpoints.last_*` columns (fast dashboard read) |
| Trigger | `POST /api/v1/endpoints/:id/check` (manual, auth-protected) |

**New file:** `health_check.go` (`PerformHealthCheck`, `SaveHealthCheck`,
`UpdateEndpointStatus`, `CheckEndpointHealth`).
**New route:** `POST /endpoints/:id/check → checkEndpointHealthHandler`.

**End-to-end verified in Phase 2:**
```
POST /endpoints/<id>/check  → {"message":"health check completed"}
GET  /endpoints/<id>        → last_is_healthy:true, last_response_time_ms:101, last_status_code:200
GET  /endpoints/<id>/health → 2 historical records with timestamps
```
Committed and pushed to the private repo.

---

## Part 2 — Error Postmortem (Phase 2 Day 1)

Each entry lists the **symptom** (what was seen), the **root cause** (why), the **fix**
(what resolved it), and the **lesson**. Errors are grouped by category so patterns are
visible.

Two categories matter for the lesson:
- **Genuine** — a real discovery about the language, DB, or runtime worth knowing.
- **Self-inflicted** — caused by editing process (rebuilding working files, changing a
  signature without updating callers). Avoidable. These are the most important to learn from.

---

### Category A — Dependency & Import errors (Genuine, low-severity)

#### A1. Missing module
- **Symptom:** `no required module provides package github.com/google/uuid`
- **Root cause:** `health_check.go` imported `uuid` but the dependency wasn't in `go.mod`.
- **Fix:** `go get github.com/google/uuid`
- **Lesson:** A new import isn't usable until the module is added. `go get` updates
  `go.mod`/`go.sum`; `go mod tidy` keeps them clean.

#### A2. Unused import
- **Symptom:** `"database/sql" imported and not used`
- **Root cause:** An import left in from an early draft that the final code didn't use.
  Go treats unused imports as a **compile error**, not a warning.
- **Fix:** Removed the import.
- **Lesson:** Go is strict by design — unused imports and variables fail the build. This
  is a feature; it keeps code clean. `gofmt`/`goimports` can auto-remove them.

---

### Category B — Editing-process errors (Self-inflicted, high-severity)

> This category is the heart of the postmortem. None of these were necessary. They came
> from rebuilding whole files from scratch instead of making targeted edits, and from
> changing a function in one place without updating everywhere it was used. The fix in
> almost every case was "restore the known-good file from git, then make the *one* change
> that was actually needed."

#### B1. Rebuilt `main.go` dropped the database layer
- **Symptom:** `undefined: db`, `undefined: initDB`, then a cascade of `undefined: db`
  across `handlers.go`.
- **Root cause:** `main.go` was regenerated from scratch and the regeneration omitted the
  `var db *sql.DB` declaration and the `initDB()` function that the original working file
  had. Every handler depends on the package-level `db`, so the whole package stopped
  compiling.
- **Fix:** Re-added `var db *sql.DB` and `initDB()` (DSN build, `Ping`, `SetMaxOpenConns`,
  `SetMaxIdleConns`).
- **Lesson:** Don't rewrite a file that already works to add one route. The working file
  carried load-bearing code that wasn't obvious from the part being changed.

#### B2. Cascade from rebuilding `handlers.go` + `auth.go`
- **Symptoms (chained):**
  - `endpoint.UserID undefined (type Endpoint has no field or method UserID)`
  - `undefined: jwtSecret`
  - `undefined: NewJWT`
  - `cannot take address of statusPage["id"] (map index expression of type any)`
  - `Endpoint redeclared in this block`
- **Root cause:** Rebuilding both files introduced mismatches between them — a struct
  referenced a field it didn't have, helper symbols (`jwtSecret`, `NewJWT`) that lived in
  the original `auth.go` went missing, code tried to `Scan` into `gin.H` map values (which
  isn't addressable), and `Endpoint` ended up declared in **two** files at once.
- **Fix:** `git checkout auth.go handlers.go main.go` to restore the green Week 1 versions,
  **then** add only `health_check.go` plus one handler and one route.
- **Lesson:** `git checkout <file>` is the fastest "undo" in the toolbox. Restoring to a
  known-good commit and re-applying one surgical change beats debugging a self-made mess.
  A single type (`Endpoint`) must be defined in exactly one place.

#### B3. Signature change without updating the caller
- **Symptom:** `not enough arguments in call to CheckEndpointHealth: have (Endpoint) want (string, Endpoint)`
- **Root cause:** `CheckEndpointHealth` was changed to take `(userID string, endpoint
  Endpoint)` so it could thread the user id down to the INSERT — but the handler that
  calls it still passed only `(endpoint)`.
- **Fix:** Updated the call site to `CheckEndpointHealth(userID, endpoint)`.
- **Lesson:** When a function signature changes, every caller must change in the **same
  pass**. The compiler finds them for you — read the error, it names the exact mismatch.

---

### Category C — Database / SQL ↔ Go type errors (Genuine, high-value)

> These are the errors actually worth having hit. They're real, common in Go+SQL work,
> and being able to explain them is interview-grade knowledge.

#### C1. NOT NULL constraint on `health_checks.user_id`
- **Symptom:** `pq: null value in column "user_id" of relation "health_checks" violates not-null constraint`
- **Root cause:** The `health_checks` table requires `user_id`, but the original
  `SaveHealthCheck` INSERT didn't include that column, so Postgres rejected the row.
- **Fix:** Added `user_id` to the INSERT column list and threaded `userID` through
  `CheckEndpointHealth → SaveHealthCheck`.
- **Lesson:** The database schema is the contract. An INSERT must satisfy every NOT NULL
  column. The DB caught the mistake — that's the constraint doing its job.

#### C2. COALESCE coerced a timestamp into text
- **Symptom:** `sql: Scan error on column index 7, name "coalesce": unsupported Scan,
  storing driver.Value type string into type *time.Time`
- **Root cause:** The list query wrapped `last_checked_at` in a `COALESCE(...)` that
  returned a **string**, but Go was scanning that column into a `*time.Time`. Type mismatch.
- **Fix:** Removed the COALESCE on the timestamp column and selected it directly so the
  driver returns a real timestamp (or NULL).
- **Lesson:** `COALESCE` changes the returned type to match its fallback. Don't COALESCE a
  timestamp to a string and then expect to scan a `time.Time`. Keep DB types and Go scan
  targets aligned.

#### C3. NULL scanned into a non-nullable Go type
- **Symptom:** `sql: Scan error on column index 5, name "last_is_healthy":
  sql/driver: couldn't convert <nil> into type bool`
- **Root cause:** A freshly-created endpoint has never been checked, so `last_is_healthy`
  (and the other `last_*` columns) are SQL `NULL`. Scanning `NULL` into a plain `bool`
  is impossible.
- **Fix:** Changed the scan targets for nullable columns to pointers: `*bool`, `*int`,
  `*time.Time`. A `nil` pointer represents "no value yet" and serializes to JSON `null`.
- **Lesson:** **Nullable DB column → pointer (or `sql.NullX`) in Go.** This is one of the
  most common Go+SQL gotchas. The dashboard correctly shows `null` for never-checked
  endpoints because of this.

---

### Category D — Runtime / environment errors (Genuine, operational)

#### D1. Route returns 404 even though it's in the code
- **Symptom:** `POST /api/v1/endpoints/:id/check` returned `404 Not Found`.
- **Root cause:** The running server process was started **before** the route existed in
  `main.go`. A running Go binary doesn't pick up source changes until it's restarted; the
  startup route table confirmed the route wasn't registered in the live process.
- **Fix:** Stopped (`Ctrl+C`) and restarted the backend. Confirmed the route now appears
  in the `[GIN-debug]` route list on boot.
- **Lesson:** `go run` doesn't hot-reload. After editing routes/handlers, restart and read
  the printed route table — it's the source of truth for what's actually registered.
  (Tools like `air` add hot-reload if desired.)

#### D2. `invalid or expired token`
- **Symptom:** Auth-protected calls returned `{"error":"invalid or expired token"}`.
- **Root causes (two):** (1) JWTs are issued with a **15-minute** expiry, so an old token
  simply ages out. (2) A token pasted into a shell variable was **truncated**, so its
  signature no longer verified.
- **Fix:** Re-fetched a fresh token via the login endpoint and assigned it cleanly to
  `$TOKEN`.
- **Lesson:** Short token lifetimes are a security feature, not a bug. For repeated manual
  testing, fetch the token programmatically (`curl ... | grep | cut`) rather than copy-
  pasting, which avoids truncation and expiry surprises.

#### D3. Docker socket permission denied
- **Symptom:** `permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`
- **Root cause:** The shell session's user wasn't in the `docker` group for that session.
- **Fix:** `sudo usermod -aG docker $USER` then `newgrp docker` (or a new shell) to pick
  up the group.
- **Lesson:** Group membership changes apply to **new** sessions. `newgrp` activates the
  group without a full re-login.

---

## Part 3 — Patterns That Emerged

1. **Restore, then change one thing.** The fastest path out of a broken multi-file state
   was `git checkout` to the last green commit, followed by the single surgical edit that
   was actually required. Most of Category B would not have happened with this as the
   default.

2. **One source of truth per type.** `Endpoint` must live in exactly one file. Duplicated
   type definitions compile-error immediately and waste time.

3. **Nullable everywhere a value can be absent.** Any DB column that can be `NULL` needs a
   pointer or `sql.NullX` on the Go side. This single rule would have pre-empted C2 and C3.

4. **The compiler and the DB are allies.** Go's strict compile errors and Postgres's
   constraints both *caught mistakes early*. Every error in Categories A, C, and D was the
   tooling refusing to let a bug through silently.

5. **Read the boot output.** The `[GIN-debug]` route table on startup is the definitive
   list of what the *running* server serves. When a route 404s, check there first.

---

## Part 4 — Verification Checklist (repeatable)

```bash
# Backend compiles & runs
go run main.go auth.go handlers.go health_check.go
# → "Database connected successfully" and the GIN route table, including:
#   POST /api/v1/endpoints/:id/check --> main.checkEndpointHealthHandler

# Fresh token
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecurePass123!"}' \
  | grep -o '"jwt_token":"[^"]*"' | cut -d'"' -f4)

# List endpoints (nullable last_* fields serialize as null before first check)
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/v1/endpoints

# Trigger a check
curl -X POST http://localhost:8000/api/v1/endpoints/<ID>/check \
  -H "Authorization: Bearer $TOKEN"        # → health check completed

# Snapshot updated
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/v1/endpoints/<ID>
# → last_is_healthy:true, last_response_time_ms:<n>, last_status_code:200

# History recorded
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8000/api/v1/endpoints/<ID>/health
# → array of checks with checked_at timestamps
```

---

## Part 5 — Honest Status

- **Foundation + first real feature: working and committed.**
- **Maturity gaps (intentional, scheduled):** automatic scheduling, alerting (Slack),
  input validation hardening, pagination, rate limiting, API docs, backend observability.
- **The health check entry point was built to be reused** by the upcoming scheduler with
  no change to the monitoring logic — only the trigger changes.

The most useful takeaway from Day 1 isn't the feature — it's the editing discipline:
**restore to green and make one change**, define each type once, and treat every nullable
column as nullable on both sides of the boundary.

---

*Generated at the close of Phase 2 Day 1.*

---

# Phase 2 Days 2–4 — Addendum

Days 2–4 went far more smoothly than Day 1 — a direct result of the discipline the Day 1
postmortem established. Notably, **almost no self-inflicted cascade errors**: files were
edited surgically, types stayed defined once, and schema was verified before writing INSERTs.
The errors that did occur were genuine (environment, browser caching) rather than
process-induced. This addendum records what was built, the few errors hit, and — importantly
— the decisions and technical debt logged along the way.

## Part 6 — What Was Built (Days 2–4)

| Day | Feature | Files |
|---|---|---|
| 2 | Automatic scheduler — checks all active endpoints on an interval | `scheduler.go` (new), `main.go` (1 line) |
| 3 | Edge-triggered Slack alerting on state change | `alerts.go` (new), `health_check.go` (modified) |
| 4 | Frontend live status, Check Now button, confirmation flash | `frontend/src/pages/Dashboard.tsx` |

All of Days 2–3 committed green. Day 4 completed and verified locally.

## Part 7 — Error Log (Days 2–4)

### D2-1. Database not running on restart (Genuine, environment)
- **Symptom:** `Failed to connect to database: dial tcp 127.0.0.1:5432: connect: connection refused` on backend boot.
- **Root cause:** The `postgres-uptime` container was stopped (machine restart between sessions). The backend has a hard dependency on the DB and `log.Fatalf`s if it can't connect.
- **Fix:** `docker start postgres-uptime`, then relaunch backend.
- **Lesson:** Dependencies start bottom-up: database → backend → frontend. `connection refused` specifically means "nothing is listening on that port" — i.e. the service isn't running — as opposed to `timed out` (something's there, not answering) or `no such host` (address won't resolve). Knowing which "can't connect" maps to which cause is 2am debugging fluency.

### D3-1. Placeholder text saved as webhook URL (Self-inflicted, trivial)
- **Symptom:** `Alert: failed to send Slack ... Post "PASTE_WEBHOOK_SITE_URL_HERE": unsupported protocol scheme ""`
- **Root cause:** The `UPDATE users SET slack_webhook_url` command was run with the literal placeholder still in it, not a real URL. Go's HTTP client rejected a "URL" with no `https://` scheme.
- **Fix:** Re-ran the UPDATE with a real webhook.site URL; verified with a follow-up SELECT before testing again.
- **Lesson:** Confirmed the send path was actually working — it reached the POST attempt, proving everything up to the URL was correct. Verify inserted values (`SELECT` after `UPDATE`) before assuming a feature is broken. A copy-paste placeholder is not a logic bug.

### D3-2. `is_sent` staying false (Diagnosed, not a bug)
- **Symptom:** All `alert_events` rows showed `is_sent = f`.
- **Root cause:** Same as D3-1 — the send never succeeded (bad webhook URL), so the row was never flipped to `is_sent = true`. The *recording* half of the pipeline worked; the *sending* half was failing on the URL.
- **Fix:** Resolved by fixing the webhook URL (D3-1). After that, the newest row correctly showed `is_sent = t`.
- **Lesson:** The record-first / send-second design did exactly its job — it preserved an honest audit trail of events that were detected but not delivered. `is_sent = false` rows are a feature (retry candidates), not corruption.

### D4-1. Frontend button not reacting (Genuine, browser caching)
- **Symptom:** After saving the new `Dashboard.tsx`, clicking Check Now did nothing visible — though backend logs showed the `POST /check` and re-fetch both returning 200.
- **Root cause:** The browser was running a **stale cached copy** of the old Dashboard code; Vite's hot-reload hadn't fully applied. The backend was working perfectly — the disconnect was entirely browser-side.
- **Diagnosis method:** `grep -c "handleCheckNow|Check Now|checkingId" Dashboard.tsx` returned `5`, proving the new code was on disk → eliminated "paste didn't land." No red TypeScript errors → eliminated compile failure. By elimination, the fault was stale browser code.
- **Fix:** Hard refresh (`Ctrl+Shift+R`) to bypass cache and pull the current build.
- **Lesson:** When backend logs prove the request succeeded but the UI doesn't change, the problem is frontend/caching, not logic. Debug by *elimination* — run the single check that rules out half the possibilities (here, `grep` for the new code) rather than guessing. Backend logs are the source of truth for whether a request actually happened.

### Noise correctly ignored (a skill, not an error)
Two log lines appeared during Days 2–4 that were *unrelated* to the problems being debugged and were correctly set aside:
- `Alert: ... but no webhook configured` — expected behavior after the test webhook was nulled; nothing to do with the frontend button.
- `DEP0060 DeprecationWarning: util._extend` — Vite's own internal Node deprecation nag; prints on nearly every project, not an error, not user code.
- **Lesson:** Not every log line relates to the symptom under investigation. Distinguishing signal from noise — parking the irrelevant message instead of chasing it — is itself a senior debugging skill.

## Part 8 — Decisions & Accepted Technical Debt (Days 2–4)

### Decisions
1. **Immediate boot pass before the ticker loop** (Day 2) — because `time.Ticker`'s first tick is delayed a full interval; without a manual first pass, the app appears dead for one interval on startup.
2. **Read-all-then-check in the scheduler** (Day 2) — drain the endpoint SELECT into a slice before running checks, to avoid holding a read cursor open during writes.
3. **Edge-triggered alerting** (Day 3) — alert only on state *transitions*, never on steady state, to prevent alert fatigue.
4. **Record-before-send for alerts** (Day 3) — write `alert_events` with `is_sent=false` first, flip to true after a successful send, so events survive send failures.
5. **Alerting is best-effort** (Day 3) — `MaybeSendAlert` swallows all errors; a failed notification must never fail the health check.
6. **Separate `flashingId` from `checkingId`** (Day 4) — one state variable per concept ("in flight" vs "just completed"), rather than overloading one.
7. **webhook.site before real Slack** (Day 3) — validate the alert *logic* against a visible test endpoint before introducing real Slack config, decoupling "is our logic right" from "is Slack set up right."

### Accepted Technical Debt (logged, deliberately not fixed)
1. **Fast-endpoint check-button spam** (Day 4) — the `disabled` guard only blocks re-clicks *while a request is in flight*. For near-instant endpoints (e.g. `localhost/health` at microseconds) the button re-enables too fast to prevent spam. Proper fix is **backend rate limiting**, since frontend guards are UX not security (an attacker bypasses the button and calls the API directly). Connected to the Security+ concept of *risk acceptance*: the risk is understood, low-impact for now, and consciously deferred rather than ignored.
2. **Still-deferred from Day 1:** input-validation hardening, pagination, API docs, backend observability — unchanged, still scheduled.

## Part 9 — Pattern Reinforcement

Days 2–4 validated the Day 1 lessons under real conditions:
- **Schema-first worked again.** Reading `\d alert_events` before writing the INSERT meant the `event_type` values matched the `CHECK` constraint on the first try — no round-trip.
- **Surgical edits held.** No file was needlessly rebuilt; no type was redeclared; no signature-change cascade. The Day 1 mess did not recur.
- **Nullable-as-pointer, everywhere.** The `*bool` / `*string` discipline handled never-checked endpoints (Day 4 three-state) and unconfigured webhooks (Day 3 skip) cleanly.
- **Debug by elimination.** The frontend caching bug (D4-1) was cornered with a single `grep`, not guesswork.

The throughline: the process discipline established by the Day 1 postmortem is what made Days
2–4 fast and mostly error-free. The document worked.

---

*Generated at the close of Phase 2 Day 4.*

---

# Phase 3 Day 1 — Cloud Deployment: Infrastructure Foundation

First deployment session. Goal: take the app off the laptop and onto AWS, defined entirely as
Terraform. Completed Steps 1–2 of an 8-step sequence (Terraform baseline + networking
foundation), both deployed to `us-east-1` and verified. Cost so far: ~$0 (networking is free
to run).

## Part 10 — What Was Built (Phase 3 Day 1)

| Step | Built | Verified |
|---|---|---|
| 1 | Terraform baseline: `provider.tf`, `versions.tf`, `variables.tf`; `terraform init` + `validate` | init + validate clean |
| 2 | Networking: VPC, 2 public + 2 private subnets (2 AZs), IGW, public route table, backend SG, database SG (`network.tf`) | 11 resources, 3 independent ways |

Files live in `~/project-2/infra/`, separate from application code.

## Part 11 — Error Log (Phase 3 Day 1)

### P3-1. Terraform "No valid credential sources found" (Genuine, WSL environment)
- **Symptom:** `terraform plan` failed with `No valid credential sources found ... no EC2 IMDS role found ... context deadline exceeded` — despite `aws sts get-caller-identity` working moments earlier.
- **Root cause:** WSL environment split. The **AWS CLI** in use was the *Windows* binary (PATH showed `/mnt/c/Program Files/Amazon/AWSCLIV2/`), reading Windows' credentials on the C: drive. **Terraform** is a *Linux-native* binary inside WSL, looking for credentials in WSL's `~/.aws/` or Linux env vars — neither existed. Same machine, two environments, two different credential homes.
- **Diagnosis:** `cat ~/.aws/credentials` (missing), `env | grep AWS` (empty), and reading the PATH revealed the CLI was the Windows one. The "it works in the CLI" fact was misleading because the CLI wasn't the Linux tool Terraform is.
- **Fix:** `aws configure` inside WSL to write a native `~/.aws/credentials` Linux Terraform can read. (User also rotated keys: deleted the old access key, generated and configured a fresh one — good credential hygiene.)
- **Lesson:** "Installed and working" is not one fact — it depends on *which* binary in *which* environment. The `/mnt/c/` in the PATH was the tell. This is the same class of confusion as the region split below: two tools, two assumptions, confusing result.

### P3-2. Empty verification output — CLI/Terraform region split (Genuine, config mismatch)
- **Symptom:** `aws ec2 describe-vpcs`/`describe-subnets` returned nothing, even though Terraform reported 11 resources created.
- **Root cause:** Region mismatch. Terraform's provider is set to `us-east-1` (in `provider.tf`), but the **AWS CLI's default region was `eu-central-1`** (Frankfurt). The verification commands queried Frankfurt and correctly found nothing — the infrastructure is in Virginia.
- **Diagnosis:** `aws configure get region` returned `eu-central-1`. Forcing `--region us-east-1` immediately showed the `10.0.0.0/16` VPC. `terraform state list` (region-agnostic) confirmed all 11 resources existed — proving the deploy succeeded and the problem was purely the CLI looking in the wrong region.
- **Fix:** `aws configure set region us-east-1` to align CLI with Terraform.
- **Lesson:** Inconsistent configuration across tools is a real bug source — a *successful* deploy looked like a *failure* purely because two tools disagreed on region. Keeping the environment consistent eliminates a whole class of "it's there but I can't see it" confusion. Also: region is a real decision (latency, cost, GDPR data-residency) — us-east-1 chosen for learning/service-availability; eu-west-2 (London) would suit a UK production deployment better.

### Method note: diagnosis by elimination (recurring skill)
Both P3-1 and P3-2 "looked like" the infrastructure was broken; both were environment/config
issues on the *tooling* side, not the deploy. In each case the fix came from a single
disambiguating check (`terraform state list` to prove resources exist regardless of CLI
config; `aws configure get region` to expose the mismatch) rather than assuming the worst.
Same debug-by-elimination discipline as the Day 4 frontend caching bug.

## Part 12 — Decisions (Phase 3 Day 1)

1. **EKS/Kubernetes chosen deliberately** — for skill development in a targeted technology,
   funded by expiring credits. Justified as cost-benefit (benefit: real K8s reps; cost:
   credits that expire anyway), not "the spec said so."
2. **NAT Gateway omitted** — the DB only talks to the backend inside the VPC; it has no need
   to reach the internet, so ~$32/mo of NAT would be waste. (Revisit at EKS step if nodes
   need it.)
3. **Hand-write Terraform, not a community module** — chosen for the learning value of
   confronting every resource, over the speed of a prebuilt module. (Distinct axis from the
   NAT decision: *how you author* vs *what you build*.)
4. **`us-east-1` region** — for service availability and tutorial-alignment during learning;
   noted that eu-west-2 would be the real choice for a UK deployment.
5. **Teardown discipline agreed** — with IaC, running infrastructure is disposable; value is
   in the `.tf` files. Rule: don't start a billable, time-consuming step without time to
   finish or tear down. (Networking is free, so left running between sessions.)

## Part 13 — Accepted Technical Debt / Deferred (unchanged + new)
- **New:** none introduced this session — the networking was built correctly and completely.
- **Carried:** SAST/DAST/SCA security scanning to be added to CI/CD as a **dedicated DevSecOps
  hardening phase** after deployment (DAST needs a running target; several scans are more
  meaningful against live infra). Treated as first-class project work, not optional polish.
- **Carried from Phase 2:** fast-endpoint check-button spam (needs backend rate limiting),
  input-validation hardening, pagination, API docs, backend observability.

## Part 14 — Restart Point
- **Completed:** Steps 1–2 (Terraform baseline + networking), deployed to us-east-1, verified.
- **Next:** Step 3 — RDS PostgreSQL into the private subnets, then apply `schema.sql`. **First
  billable step** — start of a session with time to build, verify, and tear down.
- **On resume, verify environment first:** `cd ~/project-2/infra && terraform state list`
  (network still present) and `aws sts get-caller-identity` (creds live). Region now aligned
  to us-east-1.

---

*Generated at the close of Phase 3 Day 1.*
