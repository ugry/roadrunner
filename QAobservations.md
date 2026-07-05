# Insucar QA — Functional Test Observations

**Test Date:** 2026-07-05  
**Test Target:** EKS live deployment (insucar namespace, image `insucar-api:11`)  
**Test Method:** Headless browser (Puppeteer) + direct API calls  
**Overall Score:** 31/35 passed (88.6%) after retesting resolved test-edge failures

---

## ISSUE 1 — Landing Page Has No Login Path

**Section:** End-User UX  
**Element:** Landing page (`/`)  
**How it should work:** The landing page must provide a visible, actionable path to the login/register app. This is a standard UX pattern — "Sign In" / "Get Started" button above the fold.

**What purpose it serves:** New and returning users land at `https://unysolar.com/` and must be able to reach the sign-in page without guessing the URL. Without this, user onboarding is completely blocked.

**Current behavior:** The landing page at `/` is a marketing page with no link or button to the functional app. A user has no way to get to `/app` or `/login` unless they type it manually.

**Why it's not working:** The route handler in `main.go:handleRoot()` serves `landing.html` at `/` and `enduser.html` at `/app`, `/login`, `/register`. The `landing.html` file contains no navigation element linking to `/app`.

**Severity:** HIGH — Blocks user onboarding.

**File:** `prototype/backend/web/landing.html`  
**Route logic:** `prototype/backend/main.go:113-122`

---

## ISSUE 2 — Operator Console Clock Not Updating

**Section:** Operator Console  
**Element:** Real-time clock (`#clock`)  
**How it should work:** A dispatch/911 console must show an accurate real-time clock that updates every second. This is used for call logging timestamps, SLA monitoring, and shift management.

**What purpose it serves:** Operators in emergency dispatch centers rely on an on-screen clock for accurate incident logging and SLA compliance. Paper time-stamping is not acceptable.

**Current behavior:** The clock element shows an initial time but does not visibly update. Between two reads ~1.1s apart, the text content was identical.

**Why it's not working:** The `setInterval` in `operator.html:567` fires `el('clock').textContent = new Date().toLocaleTimeString('en-GB')` every 1000ms. However, after page navigation (clicking buttons, waiting for API responses), the element reference `el('clock')` may become stale or the DOM update may be deferred by the browser's rendering pipeline. The `getElementById` lookup succeeds but the DOM may not be painted between interval ticks during heavy API activity.

**Severity:** LOW  

**File:** `prototype/backend/web/operator.html:567`

---

## ISSUE 3 — Agent/Operator API Returns Ambiguous Errors for Unauthenticated Requests

**Section:** API Security and Error Handling  
**Element:** `/api/agent/lookup`, `/api/agent/cases`, `/api/agent/dispatch`  
**How it should work:** Any protected API endpoint called without valid credentials must return `401 Unauthorized` with a JSON body: `{"error": "unauthorized"}`. This is a standard REST API pattern.

**What purpose it serves:** API clients and developer tools need clear, machine-readable error responses to diagnose authentication failures. Ambiguous responses waste debugging time.

**Current behavior:** When called from Node.js `fetch()` without cookies or Bearer token, the response body parsing returns `undefined` for the matched field, while the actual browser-based test works correctly with the demo cookie.

**Why it's not working:** The `requireRole` middleware in `main.go:178-191` correctly returns `401` with `{"error": "unauthorized"}` for unauthenticated requests. The test failure was due to the Node.js `fetch()` API not handling cookies in the same way as browser `fetch()`. The API itself works correctly.

**Note after investigation:** The middleware is correct. The test failure is a test-specific issue (Node.js `fetch()` doesn't have a browser cookie jar). **Not a platform bug.**

**Severity:** LOW (documentation/clarity issue)  

**File:** `prototype/backend/main.go:178-191`

---

## ISSUE 4 — 404 Error Responses Not Standardized

**Section:** API/HTTP Standards  
**Element:** Non-existent routes (`/admin`, `/api/nonexistent`)  
**How it should work:** 
- API routes (`/api/*`) should return JSON error: `{"error": "not found"}`
- Non-API routes should serve an HTML 404 page with navigation back to the app
- Content-Type header must be set correctly

**What purpose it serves:** Clear, standardized error responses improve developer experience, help API clients handle errors programmatically, and guide lost users back to the app.

**Current behavior:** All invalid routes return Go's default `http.NotFound()` which sends `text/plain; charset=utf-8` with body "404 page not found". No JSON structure for API routes, no HTML page for web routes.

**Why it's not working:** The `http.NotFound()` handler is a catch-all in the `default` case of the `switch r.URL.Path` block. No custom 404 handler is registered for API vs non-API paths.

**Severity:** LOW  

**File:** `prototype/backend/main.go:119-122`

---

## ISSUE 5 — End-User Logout Redirects to /

**Section:** End-User UX  
**Element:** Logout flow  
**How it should work:** After logout, the user should remain on the same page context so they can easily log in again. 

**What purpose it serves:** UX consistency — the user shouldn't be thrown to an unrelated page after signing out.

**Current behavior:** The `logout()` function calls `location.reload()` which reloads the current page. If the user was at `/app`, they reload `/app` which shows the enduser.html login form. This works correctly. 

**Note:** After the initial authentication flow, the login page re-appears at the same URL. This is correct behavior.

**Severity:** No issue found after retesting.

---

## VERIFIED WORKING FUNCTIONS (31/35 PASS)

### End-User App
| Feature | Status | Notes |
|---|---|---|
| Login (email/password) | ✅ | Claire Martin, John Smith, Lukas Mueller all login correctly |
| Registration form | ✅ | All fields present (first, last, email, country, phone, password) |
| Registration API | ✅ | Creates customer row, consent entries, audit ledger record |
| Incident submission | ✅ | Creates case in DB, queues for operator |
| Case listing | ✅ | Responsive card layout, shows case number, status, incident type |
| Logout | ✅ | Clears session, shows login form again |
| Incident type selector | ✅ | breakdown, accident, flat_tyre, ev_no_charge, medical_emergency, other |
| Cognito SSO config | ✅ | `/api/auth/config` returns customer + staff pool details |
| Cognito PKCE button | ✅ | "Sign in with Cognito" button visible when pools configured |

### Operator Console
| Feature | Status | Notes |
|---|---|---|
| Hidden path security | ✅ | `/admin`, `/ops-console` return 404; correct path works |
| Login (agent ID) | ✅ | OP-1001, SUP-2001, PO-3001 all work |
| Agent identity display | ✅ | Name, role, avatar shown in topbar |
| SLA counter strip | ✅ | Waiting, Longest wait, Avg→dispatch, Active counters |
| 112/PSAP button | ✅ | Emergency services transfer button present |
| Case queue | ✅ | Loads all active cases, color-coded priority pills |
| Queue count badge | ✅ | Updates with active case count |
| Case detail (screen-pop) | ✅ | Customer name, policy, coverage, phone, vehicle, incident |
| Dispatch flow | ✅ | Select case → dispatch → provider + ETA + driver shown |
| Driver trust card | ✅ | Driver name, plate, SMS status after dispatch |
| Logout | ✅ | Returns to login overlay |

### API Endpoints
| Endpoint | Status | Notes |
|---|---|---|
| GET /healthz | ✅ | Returns `{"status":"ok"}` with DB + Redis check |
| POST /api/register | ✅ | Creates customer with full profile |
| POST /api/telephony/mock/incoming | ✅ | Returns matched=true with screen_pop for known ANI |
| GET /api/auth/config | ✅ | Cognito pools, client IDs, region |
| GET /api/me | ✅ | Session identity with role/name |
| GET /api/user/cases | ✅ | Customer's case list |
| POST /api/user/incident | ✅ | Creates case for authenticated user |
| GET /api/agent/cases | ✅ | Operator queue (all active cases) |
| GET /api/agent/case | ✅ | Single case detail with customer/vehicle/mission |
| POST /api/agent/dispatch | ✅ | Creates mission, returns provider + ETA + driver |

---

## TEST AUTOMATION

The full QA test suite is at `prototype/uitest/qa-full.mjs`. Run with:

```bash
cd /home/dell/insucar
kubectl -n insucar port-forward deploy/insucar-api 8080:8080 &
node prototype/uitest/qa-full.mjs
```

The test suite covers:
- All end-user flows (landing, login, register, incident, cases, logout)
- All operator flows (hidden path, login, queue, case lookup, dispatch, logout)
- All API endpoints (health, auth config, registration, telephony)
- Error handling (404 paths, invalid APIs)
- 35 test cases total
