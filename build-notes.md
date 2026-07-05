# Insucar — Build Notes & Incident Log

## 2026-07-05: Login bounce-back bug fix (Build #41)

### Symptom
User `ugur.yardimci@unygms.com` registered successfully but when logging in, got thrown
back to the login screen. This affected all users who had ever interacted with the
Cognito "Sign in with Cognito" button in the same browser.

### Root Cause (3 interacting issues)

#### 1. Cognito User Pool Mismatch (PRIMARY)
The `COGNITO_ISSUER` env var pointed to the **staff** pool
(`eu-west-1_DhDKa73Dn`), but customer tokens were issued by the **customer** pool
(`eu-west-1_MFJIcHYbC`). These pools have **different JWKS signing keys**.

When the backend tried to verify a customer Cognito token, it fetched keys from the
staff pool JWKS endpoint and failed verification because the token was signed with
the customer pool's private key.

**Evidence:**
```
Staff pool JWKS:    kid=3T7tyXecHeimk7Kn6rCg... + kid=utAfKE/uFMTwYWGsLliM...
Customer pool JWKS: kid=7chJiFt34X1ueR9xCXXC... + kid=/dvwd5aM/Sn2yUiiKCsn...
→ Different keys → tokens from one pool cannot be verified by the other.
```

#### 2. Stale Cognito Tokens Blocked Cookie Fallback (SECONDARY)
The frontend `bearerApi()` function in `enduser.html` had conditional logic:
- If Cognito token exists AND is not expired → send `Authorization: Bearer` header
- Else → set `credentials: 'include'` to send cookies

When a stale (but still valid-by-date) Cognito customer-pool token was in localStorage,
`bearerApi` sent the Bearer header. The backend failed Cognito verification (wrong pool keys),
but because `credentials` was not explicitly set, the browser's default `same-origin`
behavior sent cookies too. However, in some browser configurations (especially with
`SameSite=Lax` + no `Secure` flag), cookies were not reliably sent alongside custom
`Authorization` headers, causing the backend to see the request as unauthenticated.

#### 3. Missing `Secure` Flag on Cookies (TERTIARY)
The session cookie was set without `Secure: true`. On HTTPS origins, some browsers
restrict non-Secure cookies. Added in this fix.

### Fixes Applied (commit 91ea6a5)

#### Backend: `prototype/backend/cognito.go`
- Added `cognitoStaff` verifier for the staff pool (reads `COGNITO_STAFF_ISSUER` env var)
- Refactored `bearerIdentity()` to try **both** verifiers (customer first, then staff)
- Extracted `newVerifierFromIssuer()` factory function

```go
func bearerIdentity(r *http.Request) (*cognitoIdentity, bool) {
    tok := extractBearerToken(r)
    for _, v := range []*cognitoVerifier{cognito, cognitoStaff} {
        if v != nil {
            if id, err := v.verify(tok); err == nil {
                return id, true
            }
        }
    }
    return nil, false
}
```

#### Backend: `prototype/backend/main.go`
- Initialized `cognitoStaff` verifier at startup
- Added `Secure: true` to session cookie in `setSession()`:
  ```go
  http.SetCookie(w, &http.Cookie{
      ..., Secure: true, SameSite: http.SameSiteLaxMode,
  })
  ```

#### Frontend: `prototype/backend/web/enduser.html`
- Changed `bearerApi()` to **always** set `credentials: 'include'`, regardless of Cognito token state:
  ```js
  function bearerApi(p, o) {
    o = o || {};
    o.headers = Object.assign({ 'Content-Type': 'application/json' }, o.headers || {});
    o.credentials = 'include';  // ← NOW ALWAYS SET
    var tok = ...;
    if (tok && tok.access_token && Date.now() < tok.expires_at) {
      o.headers['Authorization'] = 'Bearer ' + tok.access_token;
    }
    return fetch(p, o)...;
  }
  ```

#### Infrastructure: `k8s/insucar-api.yaml`
- Fixed `COGNITO_ISSUER` from staff pool to customer pool
- Added `COGNITO_STAFF_ISSUER` and `COGNITO_STAFF_CLIENT_IDS`

#### Live Deployment Patches
- Patched `insucar` namespace deployment env vars:
  - `COGNITO_ISSUER` → customer pool (`eu-west-1_MFJIcHYbC`)
  - Added `COGNITO_STAFF_ISSUER` → staff pool (`eu-west-1_DhDKa73Dn`)
  - Added `COGNITO_STAFF_CLIENT_IDS` → `2emse8epipp11skn1irea09q3m`
- Updated image tag from `:40` to `:41`
- Created `insucar-config` ConfigMap and `insucar-app` Secret for future deployments

### Verification
All 7 end-to-end tests pass:
1. ✅ Register fresh user → 201, Secure cookie set
2. ✅ `/api/me` with cookie → `authenticated: true`
3. ✅ Login with registered credentials → 200, Secure cookie set
4. ✅ Logout → cookie cleared
5. ✅ Re-login after logout → 200, Secure cookie set
6. ✅ `/api/me` after re-login → `authenticated: true`
7. ✅ Secure flag present on all Set-Cookie headers

### Deployment Topology (updated 2026-07-05 19:10 UTC — Ingress moved to insucar-prod)

| Namespace | Image | Traffic | Managed By |
|-----------|-------|---------|------------|
| `insucar-prod` | `:42` | **Live** (ingress → unysolar.com) | Spinnaker pipeline |
| `insucar` | `:42` (scaled to 0) | None (retired) | — |
| `insucar-dev` | varies | DEV staging | Spinnaker pipeline |
| `insucar-uat` | varies | UAT staging | Spinnaker pipeline |

**✅ Gap resolved (2026-07-05):** The Ingress was moved from the `insucar` namespace
to `insucar-prod`. After each Spinnaker CI/CD build, the `insucar-prod` deployment
is automatically updated and immediately serves live traffic. No more manual `kubectl set image`.
