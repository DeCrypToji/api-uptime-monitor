# API Uptime Monitor — Complete Architecture Guide

## Table of Contents
1. System Overview
2. Data Flow (signup → dashboard)
3. Component Breakdown
4. Database Deep Dive
5. Authentication Flow
6. Common Failure Points
7. Debugging Guide

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
