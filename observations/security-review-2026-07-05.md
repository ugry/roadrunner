# Insucar — Security & Code Analysis
Date: 2026-07-05

## 1. Cognito Login Bounce-Back Bug — Root Cause (P0)

### Root Cause: Pool Mismatch (ALREADY FIXED in Build #41)

The bug identified in `build-notes.md` has been correctly diagnosed and fixed. The fix follows best practices:

1. **Primary cause (fixed):** `COGNITO_ISSUER` was pointing to the staff pool (`eu-west-1_DhDKa73Dn`) while customer tokens came from the customer pool (`eu-west-1_MFJIcHYbC`). Different JWKS keys → verification always failed.

2. **Secondary cause (fixed):** `bearerApi()` in `enduser.html` conditionally set `credentials: 'include'`. Now always set — see line 140 of enduser.html: `o.credentials = 'include';`

3. **Tertiary cause (fixed):** Session cookie missing `Secure: true` flag. Now set at line 239-240 of main.go.

### Current cognito.go Architecture — Analysis

The multi-pool verifier pattern in `bearerIdentity()` (lines 176-193) is well-designed:
```go
for _, v := range []*cognitoVerifier{cognito, cognitoStaff} {
    if v != nil {
        if id, err := v.verify(tok); err == nil {
            return id, true
        }
    }
}
```

**Strengths:**
- Tries customer pool first, then staff — correct priority
- Nil-safe — only attempts configured pools
- Uses `jwt.WithValidMethods([]string{"RS256"})` — algorithm pinning prevents alg=none attacks
- JWKS auto-refreshes after 1 hour (line 119)

**Potential issues (not blocker, but good to address):**
- JWKS refresh uses `context.Background()` (line 122) — should honor the request context for cancellations
- No rate limiting on JWKS fetch — a DNS failure would block all verifications
- The JWKS fetch timeout is 5 seconds (line 79) — reasonable
- `jwt.ParseWithClaims` with `jwt.WithIssuer(v.issuer)` correctly ensures the token matches the verifier's pool

### Remaining Auth Vulnerability

The `currentSession()` function (main.go:241-247) uses HMAC-SHA256 cookies as a **fallback when Cognito is disabled**. When Cognito IS enabled, `resolveCaller()` tries Cognito first, then falls back to `currentSession()`. This dual-path is deliberate for the demo mode but creates ambiguity:

- A stale Cognito token could fail, and then the cookie session would be used — which may not match the user's intent
- Recommendation: When Cognito is enabled AND the Authorization header is present, do NOT fall back to cookies. This prevents confusion.

## 2. Go Dependencies — CVE & Upgrade Analysis (P0)

### Current State
- Go 1.25.5 (latest stable)
- 4 direct dependencies, all actively maintained

### Outdated Dependencies (from `go list -m -u all`)

| Dependency | Current | Latest | Gap | Severity |
|-----------|---------|--------|-----|----------|
| `github.com/jackc/pgx/v5` | v5.6.0 | v5.10.0 | 4 minor | **High** — includes bug fixes, performance |
| `golang.org/x/crypto` | v0.17.0 | v0.53.0 | 36 minor | **Critical** — Go crypto CVEs often in this package |
| `golang.org/x/net` | v0.10.0 | v0.56.0 | 46 minor | **Critical** — HTTP/2 DoS CVEs |
| `golang.org/x/text` | v0.14.0 | v0.38.0 | 24 minor | **Medium** — Unicode handling fixes |
| `golang.org/x/sync` | v0.1.0 | v0.21.0 | 20 minor | **Low** |
| `golang.org/x/sys` | v0.30.0 | v0.46.0 | 16 minor | **Medium** |
| `golang.org/x/term` | v0.15.0 | v0.44.0 | 29 minor | **Low** |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | **(current)** | 0 | None |
| `github.com/aws/aws-sdk-go-v2` | v1.42.1 | **(current)** | 0 | None |
| `github.com/redis/go-redis/v9` | v9.21.0 | **(current)** | 0 | None |
| `github.com/jackc/pgservicefile` | v0.0.0-20221227 | v0.0.0-20240606 | 18 months | **Low** |

### Key Findings

1. **golang.org/x/net v0.10.0 is critically outdated.** This package handles HTTP clients used in the provider connector (`connector.go:338`) and JWKS fetching (`cognito.go:79`). The Go Team has patched multiple CVEs in x/net since v0.10.0, including HTTP/2 rapid reset (CVE-2023-44487/CVE-2023-39325) and various DoS vectors.

2. **golang.org/x/crypto v0.17.0 is critically outdated.** This is used transitively by pgx/v5 for SCRAM-SHA-256 authentication and by go-redis for TLS. Go crypto CVEs are frequent.

3. **pgx/v5 v5.10.0** includes significant improvements to connection pool behavior — directly relevant to the known RLS issue (SET LOCAL doesn't carry across pooled connections).

### Upgrade Plan (Priority Order)
```bash
go get golang.org/x/net@latest
go get golang.org/x/crypto@latest
go get github.com/jackc/pgx/v5@v5.10.0
go get golang.org/x/text@latest
go get golang.org/x/sys@latest
go get golang.org/x/sync@latest
go mod tidy
```

### Test Coverage Assessment
- **Total coverage: 1.0%** — effectively zero
- Only tested: `nz()`, `s()`, `getenv()`, `roleFromGroups()` (4 utility functions)
- No tests for: auth, handlers, connector, Cognito verification, dispatcher, SMS journey, photo upload, webhook handling, rate limiting
- **This is a production-readiness blocker.** Critical paths (login, dispatch, webhook) have zero test coverage.

## 3. OWASP Top 10 (2025) Assessment (P1)

### A01: Broken Access Control
- **Status: PARTIALLY ADDRESSED**
- `requireRole()` (main.go:248) checks role before handler execution — good
- `requireAdmin()` (main.go:1305) adds staff role check — good
- `handleCaseRate()` (main.go:1041) verifies case ownership — good
- `handleCaseArrived()` (main.go:1092) verifies case ownership — good
- **GAP:** Agent handlers like `handleDispatch()` don't verify if the agent belongs to the same tenant as the case
- **GAP:** Photo uploads are public (`/api/upload/photo`) — no auth required
- **GAP:** Status page is publicly accessible via token only — acceptable for tracking links

### A02: Cryptographic Failures
- **Status: ADEQUATE for prototype, INADEQUATE for production**
- JWT: RS256 with algorithm pinning — correct
- JWKS: fetched over HTTPS, key ID validation — correct
- Session cookies: `Secure: true, HttpOnly: true, SameSite: Lax` — correct
- Password hashing: SHA-256 (NOT bcrypt/argon2) at `main.go:209,295` — **UNACCEPTABLE for production**
- Webhook HMAC: SHA-256 HMAC at `connector.go:419-433` — acceptable
- The `sessionKey` default is hardcoded: `"insucar-demo-session-key-change-me"` — must be rotated

### A03: Injection
- **Status: MOSTLY SAFE**
- All database queries use `$1, $2, ...` parameterized placeholders — correct
- No observed string concatenation into SQL
- `tenantMiddleware()` (tenant.go:108-121) uses `fmt.Sprintf` to construct `SET LOCAL app.current_tenant = '%s'` — **this is SQL injection through tenant ID!** If a malicious subdomain is registered or someone controls Host header, they could inject SQL. Fix: use parameterized SET or validate tenant_id against known IDs.

**Critical Finding — SQL Injection via Tenant ID:**
```go
// tenant.go:114 — UNSAFE
fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tid)
```
This concatenates an unsanitized tenant ID into SQL. Tenant IDs come from `resolveTenantID()` which extracts from the untrusted `Host` header. While the lookup `tenantMap[host]` provides some protection, the fallback to `defaultTenantID` and the subdomain extraction logic could potentially be exploited.

**Fix:** Validate `tid` is a UUID or known-safe format before using in SET LOCAL.

### A04: Insecure Design
- **Status: NEEDS REVIEW**
- Rate limiting exists (`rateLimitMW`) — good
- No CSRF tokens on state-changing endpoints — cookies are SameSite=Lax which provides partial protection

### A05: Security Misconfiguration
- **Status: PARTIALLY ADDRESSED**
- `go vet` passes clean — good
- `webhookSecretKey` defaults to hardcoded value at `connector.go:415` — must be overridden in production
- CORS on SSE endpoint (`sse.go:66`): `Access-Control-Allow-Origin: *` — permissive but intentional for SSE
- Health endpoint is public — acceptable
- 404 response for non-existent API paths returns JSON — prevents information leakage

### A06: Vulnerable Components
- **Status: NEEDS UPDATE** (see dependency analysis above)
- x/net and x/crypto are critically outdated

### A07: Auth Failures
- **Status: MOSTLY ADDRESSED**
- Password strength enforcement: minimum 8 characters at register (main.go:364)
- No account lockout after failed attempts — **missing**
- No MFA support — **missing** (Cognito can provide this, but not configured in app)

### A08: Software & Data Integrity
- **Status: NOT ADDRESSED**
- No SBOM signing verification in the build pipeline (Jenkinsfile generates SBOM but doesn't verify)
- No cosign image signing
- No SLSA provenance

### A09: Security Logging & Monitoring
- **Status: PARTIALLY ADDRESSED**
- Structured JSON logging throughout (`connector.go`, `main.go`)
- Audit ledger exists in DB schema
- No centralized log aggregation (CloudWatch mentioned but not verified)
- No security alerting configured

### A10: Server-Side Request Forgery (SSRF) — 2025 Addition
- **Status: MOSTLY SAFE**
- `callProviderURL()` (connector.go:330) makes outbound HTTP calls — URL comes from database, not user input
- `refresh()` JWKS fetch uses a constructed URL from issuer — not user-controlled
- No user-supplied URLs are fetched

## 4. Cognito Configuration Assessment (P2)

### Current Configuration
- 3 user pools: customer (`eu-west-1_MFJIcHYbC`), staff (`eu-west-1_DhDKa73Dn`), partner
- PKCE OAuth2 used on frontend (enduser.html:125-134)
- Dual pool verification in backend (cognito.go:176-193)
- Spinnaker uses staff pool for OIDC auth (spinnakerservice.yml:35-54)

### Recommendations
1. **Enable Advanced Security:** Cognito's advanced security features (adaptive auth, compromised credential detection) should be enabled for all pools
2. **MFA:** Require MFA for staff/operator pool — these users have access to PII and dispatch capability
3. **Token lifetime:** Consider shorter access token lifetimes (currently probably 60min default) for staff pool
4. **App client separation:** Each frontend (enduser/operator/admin) should use separate app clients with appropriate scopes
5. **Cognito domain:** Use custom domains (currently using `insucar-dev-customer.auth.eu-west-1.amazoncognito.com` style) — custom domains improve trust
6. **Group mapping:** Current group-to-role mapping in `roleFromGroups()` (cognito.go:200-207) is hardcoded. Move to DB-driven or config-driven mapping
