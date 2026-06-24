-- API Uptime Monitor — PostgreSQL Schema
-- Created: June 24, 2026

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- USERS TABLE
-- ============================================================================
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL, -- bcrypt hash (not plaintext)
  github_id VARCHAR(255) UNIQUE, -- for GitHub OAuth
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  last_login TIMESTAMP,
  slack_workspace_id VARCHAR(255), -- from Slack OAuth
  slack_webhook_url TEXT, -- encrypted in application (manually provided)
  slack_bot_token TEXT, -- encrypted in application (v1.1 OAuth)
  stripe_customer_id VARCHAR(255), -- Stripe customer ID for billing
  status VARCHAR(50) DEFAULT 'active', -- active, suspended, deleted
  
  CONSTRAINT valid_email CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}$')
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_github_id ON users(github_id);
CREATE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id);

-- ============================================================================
-- ENDPOINTS TABLE (What we monitor)
-- ============================================================================
CREATE TABLE endpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  
  -- Monitoring configuration
  url VARCHAR(2048) NOT NULL, -- https://api.example.com/health
  http_method VARCHAR(10) DEFAULT 'GET', -- GET, POST, HEAD, PUT, PATCH, DELETE
  expected_status_code INT DEFAULT 200, -- expected HTTP status (200, 201, 204, etc)
  expected_response_time_ms INT DEFAULT 5000, -- alert if slower (optional)
  
  -- Metadata
  name VARCHAR(255), -- friendly name: "API Health Check", "Database Ping"
  description TEXT, -- optional description
  
  -- Status
  is_active BOOLEAN DEFAULT true,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  
  -- Last check info (denormalized for quick dashboard access)
  last_checked_at TIMESTAMP,
  last_status_code INT,
  last_response_time_ms INT,
  last_is_healthy BOOLEAN,
  
  UNIQUE(user_id, url), -- user can't monitor same URL twice
  CONSTRAINT valid_url CHECK (url ~ '^https?://'),
  CONSTRAINT valid_method CHECK (http_method IN ('GET', 'POST', 'HEAD', 'PUT', 'PATCH', 'DELETE'))
);

CREATE INDEX idx_endpoints_user_id ON endpoints(user_id);
CREATE INDEX idx_endpoints_is_active ON endpoints(is_active);
CREATE INDEX idx_endpoints_user_active ON endpoints(user_id, is_active);

-- ============================================================================
-- HEALTH_CHECKS TABLE (Results of each check)
-- ============================================================================
CREATE TABLE health_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  
  -- Result data
  status_code INT, -- HTTP status code returned (200, 502, timeout, etc)
  response_time_ms INT, -- milliseconds to get response
  is_healthy BOOLEAN NOT NULL, -- matches expected_status_code
  error_message TEXT, -- "Connection timeout", "DNS failed", "TLS error", etc
  
  -- Metadata
  checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  attempt_number INT DEFAULT 1, -- which retry attempt (1-3)
  
  CONSTRAINT valid_response_time CHECK (response_time_ms >= 0)
);

-- Critical index: queries by endpoint + time for dashboard
CREATE INDEX idx_health_checks_endpoint_time ON health_checks(endpoint_id, checked_at DESC);
CREATE INDEX idx_health_checks_user_time ON health_checks(user_id, checked_at DESC);

-- For cleanup job (delete old records)
CREATE INDEX idx_health_checks_created ON health_checks(checked_at DESC);

-- ============================================================================
-- ALERT_EVENTS TABLE (Alerts sent to user)
-- ============================================================================
CREATE TABLE alert_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint_id UUID NOT NULL REFERENCES endpoints(id) ON DELETE CASCADE,
  
  -- Event type
  event_type VARCHAR(50) NOT NULL, -- 'endpoint_down', 'endpoint_recovered', 'slow_response'
  
  -- Alert delivery
  is_sent BOOLEAN DEFAULT false,
  sent_at TIMESTAMP,
  alert_method VARCHAR(50) DEFAULT 'slack', -- 'slack', 'email' (v1.1), 'webhook' (v1.1)
  
  -- Slack metadata
  slack_message_ts VARCHAR(255), -- Slack message timestamp (for threading/updates)
  slack_channel VARCHAR(255), -- which channel it was sent to
  
  -- Details
  message TEXT, -- "api.myapp.com is DOWN (502 Bad Gateway)"
  downtime_seconds INT, -- if recovery event, how long was downtime
  
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  
  CONSTRAINT valid_event_type CHECK (event_type IN ('endpoint_down', 'endpoint_recovered', 'slow_response'))
);

CREATE INDEX idx_alert_events_user ON alert_events(user_id, created_at DESC);
CREATE INDEX idx_alert_events_endpoint ON alert_events(endpoint_id, created_at DESC);
CREATE INDEX idx_alert_events_unsent ON alert_events(is_sent, created_at) WHERE is_sent = false;

-- ============================================================================
-- STATUS_PAGES TABLE (Public pages users can share)
-- ============================================================================
CREATE TABLE status_pages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  
  -- URL slug
  slug VARCHAR(255) NOT NULL, -- "myapp" -> uptime.decryptoji.com/myapp
  
  -- Display
  name VARCHAR(255) NOT NULL, -- "MyApp Status"
  description TEXT, -- optional description
  theme VARCHAR(50) DEFAULT 'light', -- 'light', 'dark'
  
  -- Configuration
  included_endpoints UUID[] DEFAULT ARRAY[]::UUID[], -- which endpoints to show
  is_public BOOLEAN DEFAULT true,
  
  -- Metadata
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  
  UNIQUE(user_id, slug)
);

CREATE INDEX idx_status_pages_user ON status_pages(user_id);
CREATE INDEX idx_status_pages_public ON status_pages(is_public);

-- ============================================================================
-- SUBSCRIPTIONS TABLE (Billing info)
-- ============================================================================
CREATE TABLE subscriptions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  
  -- Tier
  tier VARCHAR(50) DEFAULT 'free', -- 'free', 'starter', 'pro'
  
  -- Stripe
  stripe_subscription_id VARCHAR(255), -- Stripe subscription ID (null if free)
  stripe_product_id VARCHAR(255), -- Stripe product ID
  stripe_price_id VARCHAR(255), -- Stripe price ID
  
  -- Billing
  billing_email VARCHAR(255),
  current_period_start TIMESTAMP,
  current_period_end TIMESTAMP,
  
  -- Status
  status VARCHAR(50) DEFAULT 'active', -- 'active', 'past_due', 'canceled'
  cancel_at_period_end BOOLEAN DEFAULT false,
  
  -- Metadata
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  
  CONSTRAINT valid_tier CHECK (tier IN ('free', 'starter', 'pro')),
  CONSTRAINT valid_status CHECK (status IN ('active', 'past_due', 'canceled'))
);

CREATE INDEX idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_tier ON subscriptions(tier);
CREATE INDEX idx_subscriptions_stripe ON subscriptions(stripe_subscription_id);

-- ============================================================================
-- API_KEYS TABLE (For programmatic access)
-- ============================================================================
CREATE TABLE api_keys (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  endpoint_id UUID REFERENCES endpoints(id) ON DELETE CASCADE, -- null = all endpoints
  
  -- Key
  key_hash VARCHAR(255) NOT NULL UNIQUE, -- hashed (never store plaintext)
  key_prefix VARCHAR(20) NOT NULL, -- "uptime_..." for display (not secret)
  
  -- Metadata
  name VARCHAR(255), -- "Production API Key", "Monitoring Tool"
  last_used_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  
  -- Status
  is_active BOOLEAN DEFAULT true,
  revoked_at TIMESTAMP
);

CREATE INDEX idx_api_keys_user ON api_keys(user_id);
CREATE INDEX idx_api_keys_endpoint ON api_keys(endpoint_id);
CREATE INDEX idx_api_keys_active ON api_keys(is_active);

-- ============================================================================
-- AUDIT_LOG TABLE (Track all actions for compliance)
-- ============================================================================
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  
  -- Action
  action VARCHAR(50) NOT NULL, -- 'user_signup', 'endpoint_created', 'alert_sent', etc
  resource_type VARCHAR(50), -- 'user', 'endpoint', 'alert'
  resource_id UUID, -- which resource was affected
  
  -- Details
  details JSONB, -- flexible data: { old_value, new_value, reason, etc }
  ip_address INET,
  user_agent TEXT,
  
  -- Timestamp
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_user ON audit_logs(user_id, created_at DESC);
CREATE INDEX idx_audit_logs_action ON audit_logs(action, created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);

-- ============================================================================
-- VIEWS (For common queries)
-- ============================================================================

-- View: Current uptime percentage for each endpoint (last 7 days)
CREATE VIEW endpoint_uptime_7d AS
SELECT
  endpoint_id,
  COUNT(*) as total_checks,
  SUM(CASE WHEN is_healthy THEN 1 ELSE 0 END) as healthy_checks,
  ROUND(100.0 * SUM(CASE WHEN is_healthy THEN 1 ELSE 0 END) / COUNT(*), 2) as uptime_percent
FROM health_checks
WHERE checked_at > NOW() - INTERVAL '7 days'
GROUP BY endpoint_id;

-- View: Current uptime percentage for each endpoint (last 30 days)
CREATE VIEW endpoint_uptime_30d AS
SELECT
  endpoint_id,
  COUNT(*) as total_checks,
  SUM(CASE WHEN is_healthy THEN 1 ELSE 0 END) as healthy_checks,
  ROUND(100.0 * SUM(CASE WHEN is_healthy THEN 1 ELSE 0 END) / COUNT(*), 2) as uptime_percent
FROM health_checks
WHERE checked_at > NOW() - INTERVAL '30 days'
GROUP BY endpoint_id;

-- View: Recent incidents (status changes)
CREATE VIEW recent_incidents AS
SELECT
  user_id,
  endpoint_id,
  event_type,
  message,
  downtime_seconds,
  created_at
FROM alert_events
WHERE event_type IN ('endpoint_down', 'endpoint_recovered')
ORDER BY created_at DESC;

-- ============================================================================
-- FUNCTIONS (For application logic)
-- ============================================================================

-- Function: Update endpoint last_checked info
CREATE OR REPLACE FUNCTION update_endpoint_last_check()
RETURNS TRIGGER AS $$
BEGIN
  UPDATE endpoints
  SET 
    last_checked_at = NEW.checked_at,
    last_status_code = NEW.status_code,
    last_response_time_ms = NEW.response_time_ms,
    last_is_healthy = NEW.is_healthy,
    updated_at = CURRENT_TIMESTAMP
  WHERE id = NEW.endpoint_id;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger: Auto-update endpoint on new health check
CREATE TRIGGER trigger_update_endpoint_on_check
AFTER INSERT ON health_checks
FOR EACH ROW
EXECUTE FUNCTION update_endpoint_last_check();

-- Function: Get user tier limits
CREATE OR REPLACE FUNCTION get_tier_limits(tier VARCHAR)
RETURNS TABLE(
  max_endpoints INT,
  max_api_requests_per_day INT,
  min_check_interval_seconds INT,
  history_retention_days INT
) AS $$
BEGIN
  RETURN QUERY SELECT
    CASE 
      WHEN tier = 'free' THEN 2
      WHEN tier = 'starter' THEN 10
      WHEN tier = 'pro' THEN 50
      ELSE 0
    END,
    CASE 
      WHEN tier = 'free' THEN 1000
      WHEN tier = 'starter' THEN 10000
      WHEN tier = 'pro' THEN 999999
      ELSE 0
    END,
    CASE 
      WHEN tier = 'free' THEN 300 -- 5 minutes
      WHEN tier = 'starter' THEN 60 -- 1 minute
      WHEN tier = 'pro' THEN 30 -- 30 seconds
      ELSE 0
    END,
    CASE 
      WHEN tier = 'free' THEN 1 -- 24 hours
      WHEN tier = 'starter' THEN 30
      WHEN tier = 'pro' THEN 90
      ELSE 0
    END;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- COMMENTS (Documentation)
-- ============================================================================

COMMENT ON TABLE users IS 'User accounts with authentication and Slack integration';
COMMENT ON TABLE endpoints IS 'HTTP endpoints that users want to monitor';
COMMENT ON TABLE health_checks IS 'Results of each health check (one row per check)';
COMMENT ON TABLE alert_events IS 'Alerts sent to users (when endpoint status changes)';
COMMENT ON TABLE status_pages IS 'Public status pages users can share';
COMMENT ON TABLE subscriptions IS 'Billing information and subscription tier';
COMMENT ON TABLE api_keys IS 'API keys for programmatic access to monitoring data';
COMMENT ON TABLE audit_logs IS 'Audit trail of all user actions (for compliance)';

-- ============================================================================
-- INITIAL DATA (Optional)
-- ============================================================================

-- You can optionally seed test data here, e.g.:
-- INSERT INTO users (email, password_hash) VALUES ('test@example.com', 'bcrypt_hash_here');
-- INSERT INTO subscriptions (user_id, tier) VALUES ('user_uuid', 'free');

-- ============================================================================
-- CLEANUP JOBS (Run periodically)
-- ============================================================================

-- Function: Delete old health checks (by tier retention)
CREATE OR REPLACE FUNCTION cleanup_old_health_checks()
RETURNS void AS $$
BEGIN
  -- Delete health checks older than retention period
  DELETE FROM health_checks
  WHERE checked_at < NOW() - INTERVAL '90 days'; -- Pro tier keeps 90 days
  
  -- Free tier: keep only 24 hours
  DELETE FROM health_checks h
  USING endpoints e, subscriptions s
  WHERE h.endpoint_id = e.id
    AND e.user_id = s.user_id
    AND s.tier = 'free'
    AND h.checked_at < NOW() - INTERVAL '1 day';
  
  -- Starter tier: keep only 30 days
  DELETE FROM health_checks h
  USING endpoints e, subscriptions s
  WHERE h.endpoint_id = e.id
    AND e.user_id = s.user_id
    AND s.tier = 'starter'
    AND h.checked_at < NOW() - INTERVAL '30 days';
END;
$$ LANGUAGE plpgsql;

-- Run with: SELECT cleanup_old_health_checks();
-- Schedule with cron: every day at 2am UTC

-- ============================================================================
-- END OF SCHEMA
-- ============================================================================