# API Uptime Monitor — Complete Architecture Guide

## Table of Contents
1. System Overview
2. Data Flow (signup → dashboard)
3. Component Breakdown
4. Database Deep Dive
5. Authentication Flow
6. Common Failure Points
7. Debugging Guide
8. Health Check System (Phase 2)

---

## 1. SYSTEM OVERVIEW

```
┌─────────────────────────────────────────────────────────────────┐
│                         USER'S BROWSER                          │
│                    (http://localhost:3000)                      │
│                          React App                              │
│  ┌──────────┐  ┌──────────┐  ┌────────────┐  ┌──────────────┐  │
│  │  Login   │  │  Signup  │  │ Dashboard  │  │ Add Endpoint │  │
│  │  Page    │  │  Page    │  │   (list)   │  │    Form      │  │
│  └──────────┘  └──────────┘  └────────────┘  └──────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              │
                    HTTP Requests & Responses
                   (JSON over HTTPS/HTTP)
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                      GO BACKEND SERVER                          │
│                  (http://localhost:8000)                        │
│                                                                 │
│  Route: POST /api/v1/auth/signup                               │
│  │ → Parse email + password                                    │
│  │ → Hash password with bcrypt                                 │
│  │ → Insert user into database                                │
│  │ → Generate JWT token                                        │
│  │ → Return token to frontend                                  │
│                                                                 │
│  Route: POST /api/v1/auth/login                                │
│  │ → Lookup user by email                                      │
│  │ → Verify password with bcrypt                               │
│  │ → Generate JWT token                                        │
│  │ → Return token to frontend                                  │
│                                                                 │
│  Route: GET /api/v1/endpoints (Protected)                      │
│  │ → Verify JWT token from Authorization header               │
│  │ → Extract user_id from token                                │
│  │ → Query database for user's endpoints                       │
│  │ → Return endpoint list as JSON                              │
│                                                                 │
│  Route: POST /api/v1/endpoints (Protected)                     │
│  │ → Verify JWT token                                          │
│  │ → Parse endpoint details (URL, method, etc)                 │
│  │ → Insert into database                                      │
│  │ → Return endpoint ID                                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                      SQL Queries
                   (PostgreSQL Protocol)
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                    POSTGRESQL DATABASE                          │
│                  (localhost:5432 in Docker)                     │
│                                                                 │
│  Table: users                                                   │
│  ├─ id (UUID primary key)                                      │
│  ├─ email (unique)                                             │
│  ├─ password_hash (bcrypt hash)                                │
│  └─ created_at                                                 │
│                                                                 │
│  Table: endpoints                                               │
│  ├─ id (UUID primary key)                                      │
│  ├─ user_id (foreign key → users.id)                           │
│  ├─ url (what to monitor)                                      │
│  ├─ http_method (GET, POST, etc)                               │
│  ├─ expected_status_code (200, 201, etc)                       │
│  ├─ last_is_healthy (boolean)                                  │
│  ├─ last_response_time_ms (integer)                            │
│  └─ last_checked_at (timestamp)                                │
│                                                                 │
│  Table: health_checks                                           │
│  ├─ id (UUID primary key)                                      │
│  ├─ endpoint_id (foreign key → endpoints.id)                   │
│  ├─ status_code (HTTP status from check)                       │
│  ├─ response_time_ms (how long it took)                        │
│  ├─ is_healthy (boolean)                                       │
│  └─ checked_at (when the check ran)                            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 2. DATA FLOW: SIGNUP → DASHBOARD

### Flow 1: User Signs Up

**Step 1: Frontend (React)**
```
User types in signup form:
  email: "janali@example.com"
  password: "SecurePass123!"

User clicks "Sign Up"
  → handleSubmit() function runs
  → Converts expected_status_code to number
  → Sends HTTP POST to backend
```

**HTTP Request Sent:**
```
POST /api/v1/auth/signup HTTP/1.1
Host: localhost:8000
Content-Type: application/json

{
  "email": "janali@example.com",
  "password": "SecurePass123!"
}
```

**Step 2: Backend (Go) - signupHandler()**
```
1. Receive JSON from frontend
2. Parse JSON into SignupRequest struct
   email = "janali@example.com"
   password = "SecurePass123!"

3. Hash password with bcrypt
   SecurePass123! → $2a$10$abcd1234efgh5678ijkl... (60 chars)
   ↑ This is ONE-WAY encryption. Can't unhash.

4. Insert into database:
   INSERT INTO users (email, password_hash, created_at)
   VALUES ('janali@example.com', '$2a$10$abcd...', NOW())

5. Database generates UUID for user
   id = "4d8c6e5f-e5ad-4baa-8d1f-cf9c92340e6d"

6. Create JWT token
   Header: { "alg": "HS256", "typ": "JWT" }
   Payload: { 
     "user_id": "4d8c6e5f-e5ad-4baa-8d1f-cf9c92340e6d",
     "email": "janali@example.com",
     "exp": 1719355847
   }
   Signature: HMAC-SHA256(header.payload, JWT_SECRET)

7. Return response:
```

**HTTP Response:**
```
HTTP/1.1 201 Created
Content-Type: application/json

{
  "user_id": "4d8c6e5f-e5ad-4baa-8d1f-cf9c92340e6d",
  "jwt_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiNGQ4YzZlNWYtZTVhZC00YmFhLThhZDFmLWNmOWM5MjM0MGU2ZCIsImVtYWlsIjoiamFuYWxpQGV4YW1wbGUuY29tIiwiZXhwIjoxNzE5MzU1ODQ3fQ.abcd1234efgh..."
}
```

**Step 3: Frontend Stores Token**
```javascript
// In Login.tsx or Signup.tsx
localStorage.setItem('jwt_token', response.data.jwt_token)
localStorage.setItem('user_id', response.data.user_id)

// localStorage now contains the JWT
// This persists even after browser closes
```

**Step 4: Frontend Redirects to Dashboard**
```javascript
navigate('/dashboard')
```

---

### Flow 2: User Adds an Endpoint

**Step 1: Frontend Sends Request**
```javascript
const token = localStorage.getItem('jwt_token')
// token = "eyJhbGciOiJIUzI1NiIs..."

const payload = {
  name: "API Health Check",
  url: "https://api.example.com/health",
  http_method: "GET",
  expected_status_code: 200
}

axios.post('/api/v1/endpoints', payload, {
  headers: { Authorization: `Bearer ${token}` }
})
```

**HTTP Request Sent:**
```
POST /api/v1/endpoints HTTP/1.1
Host: localhost:8000
Content-Type: application/json
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...

{
  "name": "API Health Check",
  "url": "https://api.example.com/health",
  "http_method": "GET",
  "expected_status_code": 200
}
```

**Step 2: Backend - createEndpointHandler()**
```
1. Extract Authorization header
   Authorization: Bearer eyJhbGciOiJIUzI1NiIs...

2. authMiddleware() verifies token
   → Split "Bearer " prefix → get token
   → Use JWT_SECRET to verify signature
   → Decode payload
   → Extract user_id = "4d8c6e5f-e5ad-4baa-8d1f-cf9c92340e6d"
   → Check expiration time

3. If token invalid → return 401 Unauthorized
   If token valid → continue

4. Parse JSON body
   {
     name: "API Health Check",
     url: "https://api.example.com/health",
     http_method: "GET",
     expected_status_code: 200
   }

5. Insert into database
   INSERT INTO endpoints (
     user_id,
     name,
     url,
     http_method,
     expected_status_code,
     is_active,
     created_at,
     updated_at
   ) VALUES (
     '4d8c6e5f-e5ad-4baa-8d1f-cf9c92340e6d',
     'API Health Check',
     'https://api.example.com/health',
     'GET',
     200,
     true,
     NOW(),
     NOW()
   )

6. Database generates endpoint ID
   id = "12345678-abcd-4321-efgh-ijklmnopqrst"

7. Return response
```

**HTTP Response:**
```
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": "12345678-abcd-4321-efgh-ijklmnopqrst"
}
```

**Step 3: Frontend Fetches Updated List**
```javascript
// After endpoint created, fetch all endpoints
const response = await axios.get('/api/v1/endpoints', {
  headers: { Authorization: `Bearer ${token}` }
})

// Response contains array of all user's endpoints
// Frontend displays in table
```

---

## 3. COMPONENT BREAKDOWN

### Frontend (React)

**main.tsx** - Entry Point
```typescript
// This is where React mounts to the DOM
ReactDOM.createRoot(document.getElementById('root')!).render(
  <App />
)
```

**App.tsx** - Router & Auth Check
```typescript
function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  
  useEffect(() => {
    // On mount, check if JWT token exists in localStorage
    const token = localStorage.getItem('jwt_token')
    setIsAuthenticated(!!token)  // Convert to boolean
  }, [])
  
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/signup" element={<Signup />} />
      <Route path="/dashboard" element={<Dashboard />} />
    </Routes>
  )
}
```

**Login.tsx / Signup.tsx** - Auth Pages
```typescript
// These components:
// 1. Render a form
// 2. Collect email + password
// 3. Send to backend
// 4. Store JWT token in localStorage
// 5. Redirect to dashboard
```

**Dashboard.tsx** - Main App
```typescript
// This component:
// 1. Fetches user's endpoints from backend
// 2. Displays stats (total, healthy, down)
// 3. Shows list of endpoints
// 4. Renders "Add Endpoint" form
// 5. Handles logout
```

---

### Backend (Go)

**main.go** - Server Setup
```go
func main() {
  // 1. Connect to PostgreSQL
  db, err := initDB()
  
  // 2. Create Gin router
  router := gin.Default()
  
  // 3. Define routes
  v1 := router.Group("/api/v1")
  v1.POST("/auth/signup", signupHandler)
  v1.POST("/auth/login", loginHandler)
  v1.GET("/endpoints", authMiddleware(), getEndpointsHandler)
  v1.POST("/endpoints", authMiddleware(), createEndpointHandler)
  
  // 4. Start listening on port 8000
  router.Run(":8000")
}
```

**auth.go** - JWT Middleware
```go
func authMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
    // 1. Extract Authorization header
    //    "Bearer eyJhbGciOiJIUzI1NiIs..."
    
    // 2. Split and get token
    tokenString := parts[1]  // Remove "Bearer "
    
    // 3. Parse token with secret
    token, err := jwt.ParseWithClaims(
      tokenString, 
      &Claims{}, 
      func(token *jwt.Token) (interface{}, error) {
        return []byte(os.Getenv("JWT_SECRET")), nil
      }
    )
    
    // 4. Verify token is valid
    if !token.Valid {
      c.JSON(401, "invalid token")
      c.Abort()  // Stop processing this request
      return
    }
    
    // 5. Extract user_id and email from token
    claims := token.Claims.(*Claims)
    c.Set("user_id", claims.UserID)
    c.Set("email", claims.Email)
    
    // 6. Continue to next handler
    c.Next()
  }
}
```

**handlers.go** - Endpoint Logic
```go
func getEndpointsHandler(c *gin.Context) {
  // 1. Get user_id from context (set by authMiddleware)
  userID := GetUserID(c)
  
  // 2. Query database for user's endpoints
  rows, err := db.Query(
    "SELECT id, name, url, http_method... FROM endpoints WHERE user_id = $1",
    userID,
  )
  
  // 3. Scan results into endpoint structs
  endpoints := []Endpoint{}
  for rows.Next() {
    var ep Endpoint
    rows.Scan(&ep.ID, &ep.Name, &ep.URL, ...)
    endpoints = append(endpoints, ep)
  }
  
  // 4. Return as JSON
  c.JSON(200, endpoints)
}
```

---

## 4. DATABASE DEEP DIVE

### users Table

```sql
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,  -- bcrypt hash, 60 chars
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Why UUID?** 
- Can't guess user IDs from URL
- Distributed across microservices
- More secure than sequential IDs (1, 2, 3...)

**Why password_hash?**
- Never store plain passwords
- bcrypt is ONE-WAY: can't reverse hash to get password
- When user logs in: bcrypt.CompareHashAndPassword(hash, providedPassword)

---

### endpoints Table

```sql
CREATE TABLE endpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  url VARCHAR(2048) NOT NULL,
  http_method VARCHAR(10) DEFAULT 'GET',
  expected_status_code INT DEFAULT 200,
  name VARCHAR(255),
  is_active BOOLEAN DEFAULT true,
  
  -- Health check results (denormalized for speed)
  last_is_healthy BOOLEAN,
  last_response_time_ms INT,
  last_checked_at TIMESTAMP,
  
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  
  -- Relationships
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE(user_id, url)  -- Can't add same URL twice per user
);
```

**Denormalization Note:**
```
Normally: Query health_checks table for latest status
Reality:  We store last_is_healthy in endpoints for speed

Why? Dashboard needs to show 100 endpoints instantly.
Querying health_checks for each endpoint = slow.
Storing last result = fast lookup.

Trade-off: Uses more storage, but queries are 10x faster.
```

---

### health_checks Table

```sql
CREATE TABLE health_checks (
  id UUID PRIMARY KEY,
  endpoint_id UUID NOT NULL,
  status_code INT,           -- What HTTP status we got (200, 500, etc)
  response_time_ms INT,      -- How many ms it took
  is_healthy BOOLEAN,        -- status_code == expected_status_code?
  error_message TEXT,        -- Timeout? Connection refused?
  checked_at TIMESTAMP,
  
  FOREIGN KEY (endpoint_id) REFERENCES endpoints(id)
);

CREATE INDEX idx_health_checks_endpoint_time 
  ON health_checks(endpoint_id, checked_at DESC);
  -- Makes time-series queries fast
```

**Time-Series Data:**
```
Each check creates one row in health_checks.
If you check an endpoint every 30 seconds:
  - 2,880 checks per day
  - 86,400 checks per month
  - 1,051,200 checks per year (per endpoint!)

Index on (endpoint_id, checked_at) makes:
  "Give me last 100 checks for endpoint X" instant.
```

---

## 5. AUTHENTICATION FLOW - DETAILED

### Signup Flow

```
Frontend (React)
  ↓
User fills form: email="test@example.com", password="SecurePass123!"
  ↓
Click "Sign Up"
  ↓
handleSubmit() → axios.post('/api/v1/auth/signup', {email, password})
  ↓
Backend receives request
  ↓
signupHandler()
  ├─ Parse JSON
  ├─ Validate email format (binding:"email")
  ├─ Hash password: bcrypt.GenerateFromPassword("SecurePass123!", 10)
  │   → Returns: $2a$10$N9qo8uLO...
  │   → Cost 10 = ~100ms to hash (security vs speed tradeoff)
  ├─ INSERT INTO users VALUES (email, hashedPassword)
  ├─ Get back user ID
  ├─ Create JWT token:
  │   Header: {"alg": "HS256", "typ": "JWT"}
  │   Payload: {"user_id": "...", "email": "...", "exp": ...}
  │   Signature: HMAC-SHA256(header.payload, JWT_SECRET)
  └─ Return JWT to frontend
  ↓
Frontend receives JWT
  ↓
localStorage.setItem('jwt_token', jwt)
  ↓
Redirect to /dashboard
  ↓
Dashboard loads, calls GET /api/v1/endpoints with Authorization header
  ↓
authMiddleware() validates JWT
  ↓
getEndpointsHandler() returns user's endpoints
  ↓
Dashboard displays endpoints
```

### Login Flow

```
Frontend (React)
  ↓
User enters email + password
  ↓
loginHandler() → axios.post('/api/v1/auth/login', {email, password})
  ↓
Backend receives request
  ↓
loginHandler()
  ├─ Query database: SELECT id, password_hash FROM users WHERE email=?
  ├─ If not found → return 401 "invalid email or password"
  ├─ If found → verify password:
  │   bcrypt.CompareHashAndPassword(storedHash, providedPassword)
  │   ↓
  │   Hashes providedPassword with same salt as storedHash
  │   ↓
  │   Compares: storedHash == newHash ?
  │   ↓
  │   Returns true/false (can't reverse hash!)
  ├─ If password wrong → return 401
  ├─ If password correct → create JWT
  └─ Return JWT to frontend
  ↓
Same as signup from here...
```

### Token Usage

```
After login, frontend has JWT stored in localStorage.

Every protected request:
  Authorization: Bearer eyJhbGciOiJIUzI1NiI...

Backend authMiddleware():
  1. Extract header
  2. Get token after "Bearer "
  3. Verify signature with JWT_SECRET
  4. Check expiration (now < exp timestamp?)
  5. Extract user_id from payload
  6. Continue to handler

If token invalid or expired:
  → Return 401 Unauthorized
  → Frontend redirects to /login
  → User sees login form again
```

---

## 6. COMMON FAILURE POINTS

### Database Connection Issues

**Error:** `Failed to connect to database: dial tcp 127.0.0.1:5432: connect: connection refused`

**Causes:**
1. PostgreSQL container not running
2. Wrong host (localhost vs 0.0.0.0)
3. Wrong port (5432 vs 5433)
4. Wrong credentials in .env

**Debug:**
```bash
# Check if container is running
docker ps

# Check database is responding
docker exec postgres-uptime psql -U postgres -c "SELECT 1"

# Check .env has correct credentials
cat .env | grep DB_
```

---

### JWT Token Issues

**Error:** `invalid or expired token` (401)

**Causes:**
1. Token expired (exp timestamp passed)
2. Token signature invalid (JWT_SECRET changed)
3. Token corrupted/truncated
4. Authorization header malformed

**Debug:**
```bash
# Check token expiration
# Go to jwt.io, paste token, check "exp" claim

# Verify JWT_SECRET hasn't changed
cat .env | grep JWT_SECRET

# Check Authorization header format
# Should be: "Bearer eyJhbGc..."
# NOT: "eyJhbGc..." (missing Bearer)
# NOT: "Bearer:" (missing token)
```

**In browser DevTools:**
```javascript
// Check token in localStorage
localStorage.getItem('jwt_token')

// Check what's in it
const parts = token.split('.')
// parts[0] = header (base64)
// parts[1] = payload (base64)
// parts[2] = signature (base64)

// Decode payload
JSON.parse(atob(parts[1]))
// Should show: {user_id: "...", email: "...", exp: ...}
```

---

### Endpoint Creation Failures

**Error:** `duplicate key value violates unique constraint "endpoints_user_id_url_key"`

**Cause:** User already has an endpoint with that URL

**Fix:** Use different URL or delete old endpoint

**Why?** Schema has `UNIQUE(user_id, url)` constraint to prevent duplicates

---

### Password Hash Issues

**Error:** `Login works, but password comparison fails`

**Causes:**
1. Truncated password in database (needs 60 chars)
2. Incorrect bcrypt cost (should be 10)
3. Password modified after hashing

**Debug:**
```bash
# Check password hash length in database
docker exec postgres-uptime psql -U postgres -d uptime_monitor -c \
  "SELECT email, LENGTH(password_hash) FROM users;"
# Should show 60 for bcrypt
```

---

### TypeScript/Build Errors

**Error:** `error TS6133: 'axios' is declared but its value is never read`

**Cause:** Import statement but variable not used

**Fix:** Remove unused imports

**Prevention:** Add to tsconfig.json:
```json
{
  "compilerOptions": {
    "noUnusedLocals": true,  // Error if variable unused
    "noUnusedParameters": true
  }
}
```

---

## 7. DEBUGGING GUIDE

### Step 1: Identify the Layer (Frontend vs Backend vs Database)

```
User sees error
  ↓
Check browser console (F12 → Console tab)
  ├─ JavaScript error? → Frontend issue
  ├─ Network error? → Backend or network issue
  └─ No error but wrong data? → Backend/Database issue

Check backend logs (terminal where `go run` is)
  ├─ No log line for request? → Request didn't reach backend
  ├─ Log shows error? → Backend logic error
  └─ Log shows success? → Backend returned data, check frontend

Check database
  ├─ No data created? → Database issue
  └─ Wrong data? → Backend inserted wrong values
```

### Step 2: Frontend Debugging

```bash
# Check if backend is responding
curl -X GET http://localhost:8000/health

# Check if API call is actually being made
# Open DevTools → Network tab → Perform action → Look for request

# Check if token is in localStorage
# DevTools → Application → Local Storage → Check jwt_token

# Check console for JavaScript errors
# DevTools → Console → Look for red errors
```

### Step 3: Backend Debugging

```bash
# Look at logs (already in terminal)
# Look for:
#   - "Creating endpoint for user X: url=..."
#   - "Error fetching endpoints: ..."
#   - "Database error: ..."

# Add more logging if needed
log.Printf("DEBUG: value=%v", someVariable)
go run main.go auth.go handlers.go  # Logs appear here

# Test endpoint directly with curl
curl -X POST http://localhost:8000/api/v1/endpoints \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"url":"https://example.com","name":"test"}'
```

### Step 4: Database Debugging

```bash
# Connect to database
docker exec -i postgres-uptime psql -U postgres -d uptime_monitor

# Check if user exists
SELECT * FROM users WHERE email='test@example.com';

# Check if endpoint was created
SELECT * FROM endpoints WHERE user_id='YOUR_USER_ID';

# Check recent health checks
SELECT * FROM health_checks ORDER BY checked_at DESC LIMIT 10;

# Check for constraint violations
SELECT * FROM pg_constraint WHERE conname LIKE '%unique%';
```

---

## Common Error Scenarios & Fixes

### Scenario 1: "Failed to add endpoint"

**Check Order:**
1. Is backend running? `curl http://localhost:8000/health`
2. Is JWT token valid? Open DevTools, check localStorage
3. Is URL valid? Must start with http:// or https://
4. Does endpoint already exist? Check dashboard for duplicates

**If Step 1 fails:** Backend crashed or not running
```bash
# Restart backend
go run main.go auth.go handlers.go
```

**If Step 2 fails:** Token expired, user needs to login again
```javascript
// In frontend
if (err.response?.status === 401) {
  localStorage.removeItem('jwt_token')
  // Redirect to login
}
```

**If Step 3 fails:** Form validation, check browser console

**If Step 4 fails:** Unique constraint violation, try different URL

---

### Scenario 2: "Dashboard shows no endpoints but added some"

**Check Order:**
1. Did creation request return 201? Check Network tab
2. Is endpoint in database? Check with psql
3. Does getEndpointsHandler query return results? Add logging

**If Step 1 fails:** Endpoint not created, see Scenario 1

**If Step 2 fails:** Database issue
```bash
# Check if table exists
docker exec postgres-uptime psql -U postgres -d uptime_monitor \
  -c "\d endpoints"

# Check for data
SELECT COUNT(*) FROM endpoints;
```

**If Step 3 fails:** Backend query issue, add logging:
```go
log.Printf("Query returned %d endpoints", len(endpoints))
```

---

### Scenario 3: "Login works, but dashboard shows 'Failed to fetch endpoints'"

**Check Order:**
1. Is JWT token in localStorage? `localStorage.getItem('jwt_token')`
2. Is Authorization header being sent? Check Network tab
3. Does backend authMiddleware validate token? Add logging

**If Step 1 fails:** Login didn't save token
```javascript
// Check signupHandler/loginHandler response
console.log(response.data.jwt_token)
```

**If Step 2 fails:** Frontend not sending Authorization header
```javascript
// Check axios call has headers
{
  headers: { Authorization: `Bearer ${token}` }
}
```

**If Step 3 fails:** Token invalid, check JWT_SECRET matches
```bash
# Backend uses this:
os.Getenv("JWT_SECRET")

# Make sure .env has it:
cat .env | grep JWT_SECRET
```

---

## Performance Considerations

### Why queries are fast (or slow)

```sql
FAST:
SELECT * FROM endpoints WHERE user_id = $1 AND is_active = true

→ Index on (user_id, is_active) exists
→ Database can jump directly to data
→ ~1ms

SLOW:
SELECT * FROM endpoints WHERE url LIKE '%example%'

→ No index on url pattern
→ Database must scan all 1M rows
→ ~500ms

SOLUTION: Index frequently queried columns
CREATE INDEX idx_endpoints_url ON endpoints(url);
```

### Why denormalization exists

```
Normalized (SLOW):
SELECT endpoints.*, health_checks.status_code
FROM endpoints
JOIN health_checks ON endpoints.id = health_checks.endpoint_id
WHERE endpoints.user_id = $1
ORDER BY health_checks.checked_at DESC

Denormalized (FAST):
SELECT id, url, last_is_healthy, last_response_time_ms FROM endpoints
WHERE user_id = $1

→ No JOIN needed
→ All data in one table
→ 100x faster
```

---

## Understanding the Stack

### Why Go?

✅ Single binary (easy deploy)
✅ Fast (compiled, not interpreted)
✅ Built-in concurrency (goroutines)
✅ Good for APIs

### Why React?

✅ Component reusability
✅ Reactive updates (state changes → UI updates automatically)
✅ Large ecosystem
✅ Good for SPAs (Single Page Applications)

### Why PostgreSQL?

✅ ACID guarantees (data doesn't corrupt)
✅ Foreign keys (relationships between tables)
✅ Indexes (fast queries)
✅ Better than SQLite for production

### Why Docker?

✅ PostgreSQL runs in isolated container
✅ Same database on all machines
✅ Easy cleanup (just delete container)
✅ Production uses same setup

### Why JWT?

✅ Stateless (server doesn't store sessions)
✅ Can be verified without database lookup
✅ Scalable (API servers don't need shared state)
✅ Secure (signature proves token wasn't modified)

---

## Summary

When something breaks:

1. **Identify the layer:** Frontend? Backend? Database?
2. **Check logs:** Browser console, backend terminal, database
3. **Isolate:** Is it auth? Is it CRUD? Is it database?
4. **Test directly:** Use curl, DevTools, psql
5. **Fix incrementally:** Don't change 5 things at once

You now understand the entire architecture. 🎯

---

## 8. HEALTH CHECK SYSTEM (Phase 2)

This is the **core feature** of the product. Everything else — alerts, dashboards,
status pages, uptime percentages — is built on top of the data this system produces.

### 8.1 What a health check actually is

A health check answers one question: **"Is this endpoint behaving the way the user
expects right now?"**

To answer it, the backend:
1. Makes a real HTTP request to the endpoint's URL
2. Times how long the response took
3. Compares the returned status code to the *expected* status code
4. Records the verdict (healthy / unhealthy) plus the raw numbers
5. Stores a permanent time-series record AND updates a fast "latest status" snapshot

### 8.2 The file: `health_check.go`

This file contains four functions. Three do one job each; the fourth orchestrates them.

```
PerformHealthCheck(endpoint)      → makes the HTTP request, returns a result (no DB)
SaveHealthCheck(userID, id, res)  → INSERTs the result into health_checks (history)
UpdateEndpointStatus(id, res)     → UPDATEs the endpoint's "last_*" snapshot columns
CheckEndpointHealth(userID, ep)   → calls the three above in order
```

**Why split it like this?**
- `PerformHealthCheck` has **no database dependency** — it can be unit-tested with any
  URL and never touches Postgres. Pure input → output.
- `SaveHealthCheck` and `UpdateEndpointStatus` are the only two that talk to the DB,
  so if a write fails you know exactly which of the two it was from the error wrapper.
- `CheckEndpointHealth` is the single public entry point handlers call. Handlers never
  call the three lower functions directly — they call the orchestrator.

### 8.3 Data flow: triggering a check

```
User / curl
  │
  │  POST /api/v1/endpoints/:id/check
  │  Authorization: Bearer <jwt>
  ↓
authMiddleware()
  │  ├─ validates JWT signature + expiry
  │  └─ puts user_id into the request context
  ↓
checkEndpointHealthHandler()
  │  ├─ userID := GetUserID(c)
  │  ├─ endpointID := c.Param("id")
  │  ├─ SELECT the endpoint WHERE id = $1 AND user_id = $2
  │  │     (the user_id filter is what stops you checking someone else's endpoint)
  │  └─ CheckEndpointHealth(userID, endpoint)
  ↓
CheckEndpointHealth()
  │  ├─ PerformHealthCheck(endpoint)      ──► makes the real HTTP call
  │  ├─ SaveHealthCheck(userID, id, res)  ──► INSERT row into health_checks
  │  └─ UpdateEndpointStatus(id, res)     ──► UPDATE endpoints.last_* columns
  ↓
200 OK {"message": "health check completed"}
```

### 8.4 What gets written to the database

**Two writes happen per check, on purpose.**

**Write 1 — `health_checks` table (append-only history):**
```sql
INSERT INTO health_checks (
  id, user_id, endpoint_id, status_code, response_time_ms,
  is_healthy, error_message, checked_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW());
```
Every check ever run is a new row here. This is the **time-series** data that powers
uptime graphs ("99.95% over 30 days") and incident history. It grows forever (until a
cleanup job prunes old rows).

**Write 2 — `endpoints` table (latest snapshot, overwritten):**
```sql
UPDATE endpoints
SET last_is_healthy = $1,
    last_response_time_ms = $2,
    last_checked_at = NOW(),
    last_status_code = $3,
    updated_at = NOW()
WHERE id = $4;
```
This **overwrites** the previous snapshot. It exists purely for dashboard speed: the
dashboard can show "is this endpoint up right now?" by reading one column on the
`endpoints` row, instead of running a `MAX(checked_at)` subquery against millions of
`health_checks` rows.

> This is the **denormalization** pattern from section 4 in action. History lives in
> `health_checks`; the fast "current state" lookup lives on `endpoints`.

### 8.5 The healthy/unhealthy decision

```go
result.IsHealthy = (resp.StatusCode == endpoint.ExpectedStatusCode)
```

That's the whole verdict. If the user said "I expect 200" and the endpoint returns 200,
it's healthy. If it returns 500, 404, 301 — anything other than the expected code — it's
unhealthy.

**Failure cases that never even produce a status code:**
- The request times out (the client has a 10-second timeout)
- DNS fails / connection refused
- TLS handshake fails

In all of those, `client.Do(req)` returns an `err`, we set `IsHealthy = false`, and we
store the error text in `error_message`. The check still "completes" — an endpoint being
down is a valid, recorded result, not a crash.

### 8.6 Why nullable columns matter here (a real bug we hit)

Before the first check ever runs, an endpoint's `last_is_healthy`, `last_response_time_ms`,
`last_checked_at`, and `last_status_code` are all **NULL** in the database.

Go's `database/sql` will **refuse** to scan a SQL `NULL` into a plain `bool` or `int`.
You must scan into a **pointer** (`*bool`, `*int`, `*time.Time`), which can hold `nil`.

```go
// WRONG — panics/errors the moment a column is NULL:
var isHealthy bool
rows.Scan(..., &isHealthy, ...)
// → "couldn't convert <nil> into type bool"

// RIGHT — a pointer can represent "no value yet":
var isHealthy *bool
rows.Scan(..., &isHealthy, ...)
```

When the JSON is serialized, a `nil` pointer becomes `null` — which is exactly what the
frontend sees for an endpoint that's never been checked. After the first check, the
columns are populated and the pointers hold real values.

### 8.7 New failure points introduced by Phase 2

| Symptom | Likely cause | Where to look |
|---|---|---|
| `null value in column "user_id"` on check | INSERT missing user_id | `SaveHealthCheck` column list |
| `couldn't convert <nil> into type bool` | NULL scanned into non-pointer | scan targets in `getEndpointsHandler` |
| `unsupported Scan ... into *time.Time` | COALESCE forcing a timestamp to text | the SELECT query, remove COALESCE on time columns |
| `POST .../check` returns 404 | route not registered OR stale server | `main.go` route list + restart backend |
| check "completes" but `last_is_healthy` still null | wrong endpoint id, or UPDATE silently matched 0 rows | confirm the id, check `UpdateEndpointStatus` WHERE clause |

### 8.8 How to test it manually

```bash
# 1. Get a fresh token (they expire after 15 min)
TOKEN=$(curl -s -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecurePass123!"}' \
  | grep -o '"jwt_token":"[^"]*"' | cut -d'"' -f4)

# 2. Trigger a check
curl -X POST http://localhost:8000/api/v1/endpoints/<ENDPOINT_ID>/check \
  -H "Authorization: Bearer $TOKEN"
# → {"message":"health check completed"}

# 3. Confirm the snapshot updated
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/endpoints/<ENDPOINT_ID>
# → last_is_healthy: true, last_response_time_ms: 101, last_status_code: 200

# 4. Confirm the history recorded it
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8000/api/v1/endpoints/<ENDPOINT_ID>/health
# → array of every check with timestamps
```

### 8.9 What's deliberately NOT here yet

Right now checks are **manual** — something has to POST to `/check`. The next phase adds a
**scheduler** (a background loop, later EventBridge + Lambda in AWS) that walks every
active endpoint on an interval and calls this exact same `CheckEndpointHealth` function.
The monitoring logic doesn't change — only *what triggers it* changes. That's why this
function was built as a clean, self-contained entry point.

---

*End of Phase 2 Day 1 additions.*

---

## 9. AUTOMATIC SCHEDULER (Phase 2 Day 2)

Day 1 made checks *possible* but *manual* — something had to POST to `/check`. Day 2 makes
them *automatic*. This is the piece that turns the app from "a thing you can ask to check"
into "a thing that watches your endpoints on its own."

### 9.1 The file: `scheduler.go`

Three functions:
```
StartScheduler()      → runs one pass immediately, then loops forever on a ticker
getCheckInterval()    → reads CHECK_INTERVAL_SECONDS (default 60), returns a Duration
runCheckPass()        → queries all active endpoints and checks each one
```

### 9.2 It runs as a background goroutine

In `main.go`, one line — `go StartScheduler()` — placed just before the Gin router starts.
The `go` keyword launches it on its own **goroutine** (a lightweight concurrent thread), so
the scheduler loop and the HTTP server run side by side in the same process without blocking
each other. The API answers requests while the scheduler independently ticks in the
background.

### 9.3 Why one pass runs immediately on boot

This is the non-obvious part worth understanding. A Go `time.Ticker` does **not** fire
immediately — its first tick lands only after one full interval elapses. So with a 60-second
interval, a ticker-only design would boot and then do *nothing* for 60 seconds before the
first check. To the operator watching logs, that looks dead-on-arrival.

The fix: run one `runCheckPass()` immediately, *then* start the ticker loop.

```go
runCheckPass()              // cover the gap before the ticker's first tick
ticker := time.NewTicker(interval)
for range ticker.C {
    runCheckPass()          // every interval thereafter
}
```

So checks start *now*, not one interval from now. This is a general pattern with any
interval timer: if you want "run now and then every N", you trigger once manually before
entering the tick loop.

### 9.4 runCheckPass: read first, then write

```go
// 1. SELECT all active endpoints, drain the cursor into a slice
// 2. THEN loop the slice and run CheckEndpointHealth on each
```

The two steps are deliberately separate. We read the *entire* result set into memory
**before** running any checks, because each check performs INSERTs and UPDATEs. Holding a
`SELECT` cursor open while issuing writes on the same connection pool invites contention and
subtle driver issues. Read fully, close the cursor, then write. This is a small habit that
prevents a class of hard-to-reproduce database problems.

### 9.5 Per-endpoint error isolation

Inside the loop, if one endpoint's check errors, it's logged and the loop `continue`s to the
next. One unreachable endpoint (or one bad row) never aborts the whole pass. A monitoring
tool that stops monitoring *everything* because *one* thing broke would be worse than
useless — resilience per item is the point.

### 9.6 It reuses Day 1's logic verbatim

The scheduler calls the exact same `CheckEndpointHealth(userID, endpoint)` the manual button
calls. No monitoring logic was rewritten. The *only* difference: the scheduler reads
`user_id` straight from the `endpoints` row, because a background job has no logged-in user
and therefore no JWT. This is why Day 1's entry point was built self-contained — so a second
caller could reuse it with zero changes.

### 9.7 Proof it works, hands-off

```bash
docker exec postgres-uptime psql -U postgres -d uptime_monitor \
  -c "SELECT COUNT(*) FROM health_checks;"
```
Run it twice, one interval apart. The count climbs on its own — no curl, no button. That
autonomous growth *is* the feature.

---

## 10. EDGE-TRIGGERED ALERTING (Phase 2 Day 3)

Day 3 stops the system merely *recording* down endpoints and makes it *notify* you. The
whole design rests on one principle.

### 10.1 Edge-triggered, not level-triggered

- **Level-triggered** = alert based on the *current state*: "it's down → alert." Run every
  30–60s, a down endpoint would fire an alert *every pass* — dozens per hour, identical.
  That's alert fatigue; the real signal drowns in noise.
- **Edge-triggered** = alert only on a *transition*, the moment state *changes*:
  - healthy → down  ⟶  🔴 send DOWN alert
  - down → healthy  ⟶  🟢 send RECOVERED alert
  - healthy → healthy / down → down  ⟶  **silence**

You alert on the *edge* (the change), not the *level* (the ongoing state). This is standard
terminology in monitoring and hardware interrupt design — worth knowing by name.

### 10.2 Detecting a transition needs the previous state

To know state *changed*, you need the *old* value to compare against. It already exists:
`endpoints.last_is_healthy` holds the prior verdict until `UpdateEndpointStatus` overwrites
it. So the order in `CheckEndpointHealth` matters:

```
1. read old last_is_healthy   (getPreviousHealth) — BEFORE it's overwritten
2. run the check
3. save history + update snapshot
4. compare old vs new → if changed, alert   (MaybeSendAlert)
```

Read the old state *before* step 3 overwrites it, or the comparison is impossible.

### 10.3 The first-check edge case (NULL)

A never-checked endpoint has `last_is_healthy = NULL`. The rule:
- `NULL → down` : alert (it genuinely is down on first observation)
- `NULL → healthy` : silent (no need to announce "it came up" on boot)

Handled in `determineAlertType` by treating a `nil` previous pointer specially. Same
nullable-pointer discipline as everywhere else.

### 10.4 Record first, then send

`MaybeSendAlert` writes the alert to `alert_events` with `is_sent = false` **before**
attempting the Slack POST, then flips it to `true` only after the send succeeds. Why this
order: if the send fails (Slack down, bad webhook), there's still a permanent audit record
that the event happened. A design that sent first and recorded second would lose the event
entirely on a send failure. The `alert_events` table even has a partial index on
`WHERE is_sent = false` — the schema was built for a future retry job to find unsent alerts.

### 10.5 Alerting never breaks the health check

`MaybeSendAlert` returns nothing and swallows every error (logs, doesn't propagate). A failed
Slack notification must **never** fail the health check that produced it. Monitoring
integrity is more important than notification delivery. This is a deliberate reliability
boundary: the core job (recording health) can't be taken down by a secondary concern
(sending a message).

### 10.6 Schema constraint drove the code

`alert_events` has `CHECK (event_type IN ('endpoint_down','endpoint_recovered','slow_response'))`.
We read this constraint *before* writing the INSERT, so the Go code emits exactly those
strings. Checking the schema first (`\d alert_events`) meant the alerting code compiled and
ran correctly on the first attempt — no constraint-violation round-trip. Schema-first
discipline paying off.

### 10.7 Webhook is nullable

`users.slack_webhook_url` is nullable — a user who hasn't configured Slack has NULL. The code
reads it as `*string`; if `nil` or empty, it logs "no webhook configured" and returns without
crashing. The alert is still recorded; only the send is skipped. Nullable column → pointer →
graceful skip.

---

## 11. FRONTEND: LIVE STATUS & MANUAL CHECK (Phase 2 Day 4)

Day 4 is the first Phase 2 work that touches no Go. Everything the backend knew was invisible
— readable only via curl and psql. Day 4 makes the dashboard *show* it. File:
`frontend/src/pages/Dashboard.tsx`.

### 11.1 The Check Now button

Each endpoint card has a button that calls the existing `POST /api/v1/endpoints/:id/check`.
Critically: the frontend does **not** run the check or touch the database — it *asks the
backend* to, and the backend does the real work. The frontend is an untrusted client; it
makes requests, the backend owns the data. (The frontend runs on the user's machine, where
an attacker controls it — so it can never hold data access or credentials.)

### 11.2 Re-fetch to defeat stale cache

After the check POST succeeds, the handler calls `fetchEndpoints()` again. Reason: the React
app holds a **cached copy** of server state (the `endpoints` array from page load). The check
changed the database, but the browser's copy is now stale. Re-fetching pulls fresh server
truth so the UI reflects reality. Without that one line, the check would run correctly but
the screen would show old data — looking broken while working.

### 11.3 Three-state status (the NULL echo)

`last_is_healthy` can be `true`, `false`, or `null`. The badge honors all three:
- `true` → ✅ Healthy
- `false` → 🚨 Down
- `null` → ⚪ Never checked

`null` exists because of the Day 1 `*bool` decision: a never-checked endpoint is SQL NULL →
Go nil pointer → JSON `null` → the UI must show "unknown," not "down." That's one backend
decision echoing across four layers (DB → Go → JSON → React). The old dashboard's
`!last_is_healthy` treated null as down, which was wrong; Day 4 fixes it in both the badge and
the healthy/down stat counts (`=== true` / `=== false`, so null counts as neither).

### 11.4 Two kinds of state: UI vs server

The dashboard holds two categories of `useState`, and telling them apart is the core lesson:
- **UI state** — `checkingId`, `flashingId`, `showAddForm`, `loading`. Local, ephemeral,
  about how the app *feels*. Never leaves the browser.
- **Cached server state** — `endpoints`. A copy of data that truly lives on the server, and
  therefore can go stale and needs re-syncing.

Knowing which bucket a piece of state is in tells you whether you must worry about staleness.

### 11.5 checkingId vs flashingId — one concept per variable

Two *separate* state variables, deliberately:
- `checkingId` = "a check is in flight for this endpoint" → drives the button's "Checking…"
  and its `disabled` guard.
- `flashingId` = "a check just *finished* for this endpoint" → drives a ~1.5s blue ring on
  the card, confirming the click worked even when the data looks unchanged.

They represent *different moments* (in-progress vs just-completed), so they get *different
state*. Cramming both into one variable would save a line and blur two concepts. Principle:
**one piece of state represents one concept.**

### 11.6 Ordering: flash fires AFTER the re-fetch

In `handleCheckNow`, `setFlashingId(id)` comes *after* `await fetchEndpoints()`. The `await`
guarantees the fresh data has landed before the flash fires. The flash is a *signal about the
refreshed data* ("look, this card just updated"), so it must fire at the moment the new data
is on screen — not before, when the card still shows stale values. Position relative to an
`await` is a deliberate statement about guaranteed sequence.

### 11.7 Known limitation logged here (see BUILD_LOG)

The `disabled` button prevents re-clicking *only while a request is in flight*. For endpoints
that respond near-instantly (e.g. `localhost/health` at microseconds), the button re-enables
so fast it offers effectively no spam protection. Real protection would be backend rate
limiting — because frontend guards are UX, not security (an attacker bypasses the button and
hits the API directly). Logged as accepted technical debt, not fixed in Day 4.

---

*End of Phase 2 Days 2–4 additions.*
