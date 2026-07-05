# Insucar — E2E Test Suite

> Generated: 2026-07-05 | Tester: insucar-tester

## Overview

This document describes the curl-based end-to-end test suite for the Insucar roadside assistance platform. All tests are designed to be run against live endpoints.

## Live Endpoints

| Environment | URL |
|------------|-----|
| Production (ingress) | https://unysolar.com/ |
| User App | https://unysolar.com/app |
| Operator Console | https://op.unysolar.com/ |
| Jenkins | https://jenkins.unysolar.com |
| Spinnaker Deck | https://spinnaker.unysolar.com |
| Spinnaker Gate | https://gate.unysolar.com |
| Prod ELB (Spinnaker) | http://ad4de17a313444704a74f62919bfabc7-1055718284.eu-west-1.elb.amazonaws.com/ |
| Legacy Proto ELB | http://af9269372141a4fdba7953b3679d6189-59590199.eu-west-1.elb.amazonaws.com/ |

## Test Credentials

### End-Users (login by email)
| Name | Email | Password |
|------|-------|----------|
| Claire Martin | claire.martin@example.fr | Claire#2026 |
| John Smith | john.smith@example.co.uk | John#2026 |
| Lukas Mueller | lukas.mueller@example.de | Lukas#2026 |

### Operators (login by agent ID)
| Role | Agent ID | Password |
|------|----------|----------|
| Operator | OP-1001 | Operator#2026 |
| Supervisor | SUP-2001 | Supervisor#2026 |
| Product Owner | PO-3001 | Owner#2026 |

---

## Test Suite

### 1. Environment Health Checks

#### 1.1 HTTPS Endpoint Reachability
```bash
# Expected: HTTP 200 for all
curl -s -o /dev/null -w "%{http_code}" https://unysolar.com/
curl -s -o /dev/null -w "%{http_code}" https://unysolar.com/app
curl -s -o /dev/null -w "%{http_code}" https://op.unysolar.com/
curl -s -o /dev/null -w "%{http_code}" https://jenkins.unysolar.com/login
curl -s -o /dev/null -w "%{http_code}" https://spinnaker.unysolar.com/
curl -s -o /dev/null -w "%{http_code}" https://gate.unysolar.com/health
```

#### 1.2 API Health Endpoints
```bash
# Expected: {"status":"ok"}
curl -s https://unysolar.com/healthz

# Expected: {"groups":["liveness","readiness"],"status":"UP"}
curl -s https://gate.unysolar.com/health
```

### 2. TLS Verification

#### 2.1 Certificate Validity
```bash
# Expected: Let's Encrypt issuer, not expired
echo | openssl s_client -connect unysolar.com:443 -servername unysolar.com 2>/dev/null | openssl x509 -noout -issuer -dates -subject
```

#### 2.2 TLS Version Check
```bash
# Expected: TLSv1.3 accepted; TLSv1.0, TLSv1.1 rejected
echo | openssl s_client -connect unysolar.com:443 -tls1_3 2>/dev/null
echo | openssl s_client -connect unysolar.com:443 -tls1 2>/dev/null  # should fail
```

### 3. Authentication Flow

#### 3.1 User Registration
```bash
curl -s -D- -c /tmp/cookies -X POST https://unysolar.com/api/register \
  -H 'Content-Type: application/json' \
  -d '{"first":"Test","last":"User","email":"<unique>@test.com","country":"FR","phone":"<unique>","password":"Test12345!","consents":["terms","privacy"]}'
# Expected: 201 Created, Set-Cookie with Secure/HttpOnly/SameSite=Lax
```

#### 3.2 User Login
```bash
curl -s -D- -c /tmp/cookies -X POST https://unysolar.com/api/user/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"<email>","password":"<password>"}'
# Expected: 200 OK, returns user object with id/name/role, Set-Cookie present
```

#### 3.3 Session Verification
```bash
curl -s -b /tmp/cookies https://unysolar.com/api/me
# Expected: {"authenticated":true,"id":"...","name":"...","role":"user"}
```

#### 3.4 Unauthenticated Access
```bash
curl -s https://unysolar.com/api/me
# Expected: {"authenticated":false} with HTTP 200
```

#### 3.5 Logout
```bash
curl -s -D- -c /tmp/cookies -b /tmp/cookies -X POST https://unysolar.com/api/logout
# Expected: {"status":"logged_out"}, Set-Cookie clears session
```

#### 3.6 Re-Login After Logout
```bash
# Step 1: Login → Step 2: /api/me (authenticated) → Step 3: Logout → Step 4: /api/me (false) → Step 5: Login again → Step 6: /api/me (authenticated)
# Expected: Full cycle works correctly
```

### 4. Operator/Staff Authentication

#### 4.1 Operator Login
```bash
curl -s -D- -c /tmp/cookies -X POST https://op.unysolar.com/api/staff/login \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"OP-1001","password":"Operator#2026"}'
# Expected: 200 OK, Set-Cookie with session, returns staff profile
```

#### 4.2 Staff Session Verification
```bash
curl -s -b /tmp/cookies https://op.unysolar.com/api/me
# Expected: {"authenticated":true,"role":"operator",...}
```

### 5. Policy & Data Endpoints

#### 5.1 Policy List (Authenticated)
```bash
curl -s -b /tmp/cookies https://unysolar.com/api/policies
# Expected: JSON array of policies belonging to authenticated user
```

#### 5.2 Policy List (Unauthenticated)
```bash
curl -s https://unysolar.com/api/policies
# Expected: 401 or {"authenticated":false} with empty/null policies
```

#### 5.3 Cases List
```bash
curl -s -b /tmp/cookies https://unysolar.com/api/cases
# Expected: JSON array of cases
```

#### 5.4 Phone Lookup (ANI)
```bash
curl -s https://unysolar.com/api/lookup?phone=%2B33600000001
# Expected: JSON with customer/policy/vehicle data
```

### 6. Security Tests

#### 6.1 Cookie Security Attributes
```bash
curl -s -D- -c /tmp/cookies -X POST https://unysolar.com/api/user/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"claire.martin@example.fr","password":"Claire#2026"}' \
  | grep -i 'set-cookie'
# Expected: HttpOnly; Secure; SameSite=Lax
```

#### 6.2 SQL Injection Protection
```bash
# Login bypass attempt
curl -s -X POST https://unysolar.com/api/user/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"'"'"' OR 1=1 --","password":"anything"}'
# Expected: 401, "invalid credentials"

# Phone lookup injection
curl -s "https://unysolar.com/api/lookup?phone=%27%20OR%201=1%20--"
# Expected: 400 or empty safe response (not raw SQL error)
```

#### 6.3 XSS Protection
```bash
# Stored XSS in registration
curl -s -X POST https://unysolar.com/api/register \
  -H 'Content-Type: application/json' \
  -d '{"first":"<script>alert(1)</script>","last":"Test","email":"xss@test.com","country":"FR","phone":"+33600000099","password":"Test12345!","consents":["terms"]}'
# Expected: 400 rejection or sanitized storage (script tags stripped)
```

#### 6.4 CORS Headers
```bash
curl -s -D- -X OPTIONS https://unysolar.com/api/me \
  -H 'Origin: https://unysolar.com' \
  -H 'Access-Control-Request-Method: GET'
# Expected: Access-Control-Allow-Origin, Access-Control-Allow-Credentials headers present
```

#### 6.5 CSRF Protection
```bash
curl -s -D- -X POST https://unysolar.com/api/user/login \
  -H 'Content-Type: application/json' \
  -H 'Origin: https://evil.com' \
  -H 'Referer: https://evil.com/fake' \
  -d '{"email":"claire.martin@example.fr","password":"Claire#2026"}'
# Expected: Should reject or at minimum not set session cookie
```

#### 6.6 Information Disclosure
```bash
# Check for verbose error messages
curl -s -X POST https://unysolar.com/api/register \
  -H 'Content-Type: application/json' \
  -d '{"first":"Test","last":"User","email":"existing@email.com","country":"FR","phone":"+33600000001","password":"Test12345!","consents":["terms"]}'
# Expected: Generic error message (no SQLSTATE, constraint names)
```

### 7. Load Testing

#### 7.1 Concurrent Load (100 connections)
```bash
for i in $(seq 1 100); do curl -s -o /dev/null -w "%{http_code}" --max-time 5 https://unysolar.com/healthz & done; wait
# Expected: 100x 200; success rate ≥ 95%
```

#### 7.2 Sequential Response Time
```bash
for i in $(seq 1 100); do curl -s -o /dev/null -w "%{time_total}\n" https://unysolar.com/healthz; done
# Expected: Average < 500ms
```

### 8. HSTS & Security Headers

#### 8.1 HSTS Header
```bash
curl -s -D- https://unysolar.com/api/me 2>&1 | grep -i 'strict-transport'
# Expected: max-age=31536000; includeSubDomains
```

---

## Expected Outcomes Summary

| Test ID | Test Name | Expected Result |
|---------|-----------|----------------|
| 1.1 | HTTPS Endpoints Reachable | All return 200 |
| 1.2 | API Health | healthz: `{"status":"ok"}`, gate: `{"status":"UP"}` |
| 2.1 | TLS Certificate | Let's Encrypt, valid dates, correct subject |
| 2.2 | TLS Version | TLSv1.3 accepted, old versions rejected |
| 3.1 | Registration | 201, session cookie with Secure/HttpOnly/SameSite |
| 3.2 | Login | 200, user object, session cookie |
| 3.3 | Session Verify | `{"authenticated":true,...}` |
| 3.4 | No-Auth Access | `{"authenticated":false}` |
| 3.5 | Logout | Cookie cleared, subsequent access unauthenticated |
| 3.6 | Re-login | Full cycle: login→logout→re-login all work |
| 4.1 | Operator Login | 200, session cookie, staff profile |
| 5.1-5.4 | Policy/Data Endpoints | Proper JSON responses (not empty bodies) |
| 6.1 | Cookie Attributes | HttpOnly, Secure, SameSite=Lax |
| 6.2 | SQL Injection | Rejected with generic error, not raw SQL |
| 6.3 | XSS | Script tags rejected or sanitized |
| 6.4 | CORS | Proper CORS headers on OPTIONS preflight |
| 6.5 | CSRF | Cross-origin requests handled appropriately |
| 6.6 | Info Disclosure | Generic errors only (no SQLSTATE/exceptions) |
| 7.1-7.2 | Load Test | ≥95% success rate, avg <500ms |
| 8.1 | HSTS | Header present with proper values |
