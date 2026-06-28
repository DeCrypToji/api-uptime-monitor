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
