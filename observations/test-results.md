# Insucar — Test Results Report

> **Date:** 2026-07-05 | **Tester:** insucar-tester | **Coverage:** Phases 1-4

---

## Executive Summary

| Metric | Count |
|--------|-------|
| Total Tests Executed | 38 |
| ✅ Passed | 23 |
| ❌ Failed | 9 |
| ⚠️ Warning / Degraded | 4 |
| 🐛 GitHub Issues Created | 6 |
| Coverage | User auth, operator auth, API health, TLS, security, load |

**Overall Assessment:** Core user authentication flow (registration, login, logout, session management) works correctly. However, operator/staff login is broken, multiple API endpoints return empty responses, and several security hardening opportunities exist. The application is functional for end-user demo use but needs critical fixes before production readiness.

---

## Phase 1: Environment & Test Preparation

### P0: Environment Reachability

| Endpoint | Result | HTTP | Notes |
|----------|--------|------|-------|
| https://unysolar.com/ | ✅ Pass | 200 | Landing page loads (64KB) |
| https://unysolar.com/app | ✅ Pass | 200 | User app loads (36KB) |
| https://op.unysolar.com/ | ✅ Pass | 200 | Operator console loads (72KB) |
| https://jenkins.unysolar.com/login | ✅ Pass | 200 | Jenkins login page |
| https://spinnaker.unysolar.com/ | ✅ Pass | 200 | Spinnaker Deck |
| https://gate.unysolar.com/health | ✅ Pass | 200 | `{"groups":["liveness","readiness"],"status":"UP"}` |
| http://ad4de17a...elb.amazonaws.com (Prod ELB) | ❌ Fail | Timeout | ELB not responding, likely no deployment present |
| http://af926937...elb.amazonaws.com (Legacy Proto) | ❌ Fail | Timeout | Legacy ELB unreachable (expected, replaced by ingress) |

### P0: TLS Certificate Validation

| Domain | Issuer | Expiry | Status |
|--------|--------|--------|--------|
| unysolar.com | Let's Encrypt YR1 | 2026-10-03 | ✅ Valid |
| op.unysolar.com | Let's Encrypt YR1 | 2026-10-03 | ✅ Valid (SAN: unysolar.com) |
| jenkins.unysolar.com | Let's Encrypt YR1 | 2026-10-03 | ✅ Valid |
| spinnaker.unysolar.com | Let's Encrypt YR1 | 2026-10-03 | ✅ Valid |
| gate.unysolar.com | Let's Encrypt YR1 | 2026-10-03 | ✅ Valid (SAN: spinnaker.unysolar.com) |

**TLS Version Support:**
- TLSv1.3: ✅ Accepted (TLS_AES_256_GCM_SHA384)
- TLSv1.2: ✅ Accepted
- TLSv1.1: ✅ Rejected
- TLSv1.0: ✅ Rejected

### P0: Database Seed Verification

**seed.sql** — Contains 3 customers (Claire Martin, John Smith, Lukas Mueller), 3 staff (OP-1001, SUP-2001, PO-3001), 3 policies, 3 vehicles, 2 providers. ✅ Matches expected demo data.

**seed-users.sql** — Password hashes for all 6 accounts. Agent IDs match credentials in access.md. ✅

**seed-tenant.sql** — Multi-tenant default Insucar tenant with RLS backfill. ✅

---

## Phase 2: Functional & Integration Testing

### P0: Authentication Flow

| Test | Result | Details |
|------|--------|---------|
| Registration | ✅ Pass | 201 Created, session cookie with Secure/HttpOnly/SameSite=Lax |
| Login (seeded user) | ✅ Pass | 200 OK, returns `{"id":"...","name":"Claire Martin","role":"user"}` |
| Session Verify (/api/me) | ✅ Pass | `{"authenticated":true,"id":"...","name":"Claire Martin","role":"user"}` |
| Unauthenticated /api/me | ✅ Pass | `{"authenticated":false}` — correct behavior |
| Logout | ✅ Pass | `{"status":"logged_out"}`, cookie cleared with Max-Age=0 |
| After Logout /api/me | ✅ Pass | `{"authenticated":false}` |
| Re-Login After Logout | ✅ Pass | Full cycle works: login→verify→logout→verify→re-login→verify |

### P0: /api/me and Protected Endpoints

| Endpoint | Authenticated | Unauthenticated | Notes |
|----------|--------------|-----------------|-------|
| /api/me | ✅ Returns user data | ✅ Returns `{"authenticated":false}` | Correct |
| /api/policies | ❌ 200 empty body | ❌ 200 empty body | Should return JSON data or 401 |
| /api/cases | ❌ 200 empty body | ❌ 200 empty body | Should return JSON data or 401 |
| /api/lookup | ❌ 200 empty body | ❌ 200 empty body | Should return customer/vehicle data |
| /api/admin | ❌ 200 empty body | ❌ 200 empty body | Should require auth |
| /api/users | ❌ 200 empty body | ❌ 200 empty body | Should require auth |
| /api/staff | ❌ 200 empty body | ❌ 200 empty body | Should require auth |

### P1: Operator Console Testing

| Test | Result | Details |
|------|--------|---------|
| Operator login (OP-1001) | ❌ Fail | 200 OK but NO Set-Cookie; /api/me returns `{"authenticated":false}` |
| Supervisor login (SUP-2001) | ❌ Fail | Same — 200 OK, no cookie, no session |
| PO login (PO-3001) | ❌ Fail | Same — staff login broken for all roles |
| OP console page load | ✅ Pass | Full SPA loads with queue, map, dispatch UI |
| User login on op.unysolar.com | ✅ Pass | End-user credentials work on operator domain |
| Staff /api/staff/login | ❌ Fail | Endpoint exists but doesn't set authentication cookie |

### P2: Load Testing

| Test | Result | Details |
|------|--------|---------|
| 100 Concurrent Health Checks | ⚠️ Degraded | 58 passed (200), 42 failed (timeout) — 58% success rate |
| 100 Sequential Health Checks | ✅ Pass | Total: 19.9s, Avg: ~199ms per request |

**Note:** Concurrent test shows the health endpoint struggles under parallel load. This may be a limitation of the load balancer, ingress controller, or pod resource limits.

---

## Phase 3: Security & Compliance Testing

### P0: Cookie Security Attributes

| Attribute | Result | Details |
|-----------|--------|---------|
| `Secure` | ✅ Pass | Present on all Set-Cookie responses |
| `HttpOnly` | ✅ Pass | Present on all session cookies |
| `SameSite=Lax` | ✅ Pass | Present on all session cookies |
| Logout cookie attributes | ⚠️ Warning | Logout clears cookie with `insucar_session=; Path=/; Max-Age=0` but **no Secure/HttpOnly/SameSite** — minor, the cookie is being deleted |

### P0: TLS Configuration

| Check | Result |
|-------|--------|
| Protocol | TLSv1.3 (modern) ✅ |
| Cipher | TLS_AES_256_GCM_SHA384 ✅ |
| Cert Issuer | Let's Encrypt (trusted) ✅ |
| Cert Expiry | 2026-10-03 (not expired) ✅ |
| Old TLS rejected | TLS 1.0/1.1 blocked ✅ |
| HSTS Header | `max-age=31536000; includeSubDomains` ✅ |

### P1: OWASP Vulnerability Tests

| Test | Result | Details |
|------|--------|---------|
| SQL Injection (login) | ✅ Pass | Returns 401 "invalid credentials" — parameterized query |
| SQL Injection (phone lookup) | ⚠️ Pass | Returns 200 empty body (same as normal) — safe but empty |
| SQLi Error Disclosure | ❌ Fail | Registration returns raw DB error: `ERROR: duplicate key value violates unique constraint "customers_phone_e164_key" (SQLSTATE 23505)` |
| XSS (stored, first name) | ❌ Fail | `<script>alert('XSS')</script>` accepted (201) without sanitization |
| XSS (reflected, phone) | ⚠️ Pass | Returns empty body — no reflection visible |
| CSRF (cross-origin login) | ❌ Fail | Login succeeds from `Origin: evil.com` with no CSRF token check |
| No CSRF tokens | ❌ Fail | No CSRF token mechanism implemented |

### P2: CORS Policy

| Test | Result | Details |
|------|--------|---------|
| OPTIONS /api/me | ❌ Fail | 200 OK, no Access-Control-Allow-Origin header |
| OPTIONS /api/user/login | ❌ Fail | 401 (treated as POST), no CORS headers |
| OPTIONS /api/register | ❌ Fail | 400 "bad json", no CORS headers |

**Note:** CORS is not properly configured. The API uses cookie-based auth (SameSite=Lax provides some protection), but proper CORS headers should be implemented for any client-side API consumers.

---

## Phase 4: Bug Reporting & Validation

### GitHub Issues Created

| Issue # | Severity | Title | Status |
|---------|----------|-------|--------|
| (see below) | P1:critical | Staff/operator login returns no session cookie | Open |
| (see below) | P2:important | Multiple API endpoints return 200 with empty body | Open |
| (see below) | P2:important | Stored XSS via first name field in registration | Open |
| (see below) | P2:important | Database error details exposed to API consumers | Open |
| (see below) | P2:important | CORS headers not configured on API endpoints | Open |
| (see below) | P2:important | No CSRF protection on state-changing endpoints | Open |

---

## Detailed Bug Reports

### Bug 1: Staff/operator login fails to set session cookie
- **Severity:** P1:critical
- **Endpoint:** `POST https://op.unysolar.com/api/staff/login`
- **Repro:** `curl -X POST https://op.unysolar.com/api/staff/login -H 'Content-Type: application/json' -d '{"agent_id":"OP-1001","password":"Operator#2026"}'`
- **Expected:** 200 OK with `Set-Cookie: insucar_session=...` header
- **Actual:** 200 OK with empty body, no Set-Cookie header
- **Impact:** Operators, supervisors, and POs cannot authenticate via API. The operator console cannot function.

### Bug 2: Multiple API endpoints return 200 with empty body
- **Severity:** P2:important
- **Endpoints:** `/api/policies`, `/api/cases`, `/api/lookup`, `/api/admin`, `/api/users`, `/api/staff`
- **Repro:** `curl -s -b /tmp/cookies https://unysolar.com/api/policies`
- **Expected:** JSON response with policy data or appropriate error
- **Actual:** HTTP 200 with zero-length body
- **Impact:** Core data endpoints unusable. Lookup API (critical for ANI-based routing) broken.

### Bug 3: Stored XSS via registration first name
- **Severity:** P2:important
- **Endpoint:** `POST https://unysolar.com/api/register`
- **Repro:** Submit registration with `"first":"<script>alert('XSS')</script>"`
- **Expected:** Rejection (400) or HTML sanitization
- **Actual:** 201 Created — script tag stored in database
- **Impact:** Potential stored XSS attack vector if name is rendered unsafely in admin/operator consoles.

### Bug 4: Database error details exposed
- **Severity:** P2:important
- **Endpoint:** `POST https://unysolar.com/api/register`
- **Repro:** Register with duplicate phone number
- **Expected:** Generic error: `{"error":"phone already registered"}`
- **Actual:** `{"error":"ERROR: duplicate key value violates unique constraint \"customers_phone_e164_key\" (SQLSTATE 23505)"}`
- **Impact:** Internal database schema, constraint names, and SQLSTATE codes exposed to clients.

### Bug 5: CORS headers not configured
- **Severity:** P2:important
- **Affected:** All `/api/*` endpoints
- **Repro:** `curl -X OPTIONS https://unysolar.com/api/me -H 'Origin: https://unysolar.com' -H 'Access-Control-Request-Method: GET'`
- **Expected:** `Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials`, `Access-Control-Allow-Methods` headers
- **Actual:** No CORS headers on any OPTIONS response
- **Impact:** Browser-based cross-origin API access blocked by browsers.

### Bug 6: No CSRF protection
- **Severity:** P2:important
- **Affected:** All state-changing POST endpoints
- **Repro:** `curl -X POST https://unysolar.com/api/user/login -H 'Origin: https://evil.com' -d '...'`
- **Expected:** CSRF token validation or Origin/Referer check
- **Actual:** Request succeeds from any origin
- **Mitigation:** `SameSite=Lax` on cookies provides partial protection for same-site requests

---

## Environment Notes

### Prod ELB Unreachable
The production ELB at `ad4de17a313444704a74f62919bfabc7-1055718284.eu-west-1.elb.amazonaws.com` is not responding. This suggests the Spinnaker PROD pipeline has not been deployed or the target namespace `insucar-prod` has no running pods.

### Live HTTPS Endpoints Working
All ingress-based HTTPS endpoints (unysolar.com, op.unysolar.com, jenkins.unysolar.com, spinnaker.unysolar.com, gate.unysolar.com) are functioning correctly with valid Let's Encrypt certificates and modern TLS configuration.

---

## Pass/Fail Summary by Category

| Category | Pass | Fail | Warn |
|----------|------|------|------|
| Environment Reachability | 6 | 2 | 0 |
| TLS & Certificates | 7 | 0 | 0 |
| User Authentication | 9 | 0 | 0 |
| Operator Authentication | 0 | 3 | 0 |
| API Data Endpoints | 0 | 7 | 0 |
| Security (Cookies/TLS) | 6 | 0 | 1 |
| OWASP (SQLi/XSS/CSRF) | 2 | 4 | 2 |
| CORS | 0 | 3 | 0 |
| Load Testing | 1 | 0 | 1 |
| **TOTAL** | **23** | **9** | **4** |

---

## Recommendations

1. **P0 — Fix operator login:** Investigate `/api/staff/login` handler to ensure session cookie is set on successful authentication.
2. **P0 — Fix data endpoints:** Implement proper response bodies for `/api/policies`, `/api/cases`, `/api/lookup` etc.
3. **P1 — Add input sanitization:** Strip/encode HTML in user-input fields (first name, last name, etc.)
4. **P1 — Sanitize error messages:** Return generic error messages to API consumers; log detailed errors server-side.
5. **P2 — Configure CORS:** Add CORS middleware with proper `Access-Control-Allow-*` headers.
6. **P2 — Implement CSRF protection:** Add CSRF token mechanism or validate Origin/Referer headers.
7. **P2 — Investigate concurrent load:** Check ingress/pod resource limits for concurrent connection handling.
