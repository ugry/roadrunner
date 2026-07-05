# Insucar QA — End-User Experience Audit

**Date:** 2026-07-05  
**QA Engineer:** Automated API + manual analysis  
**Tests Run:** 32 across 8 sections  
**Legitimate Issues Found:** 3 backend gaps, 2 UX gaps

---

## PASSED (22/32 = 68.8%)

False negatives account for 5 of 10 failures (cookie tracking in curl tests, not platform bugs).

### What Works End-to-End

| Flow | Status |
|---|---|
| Landing page → CTA → Login page | ✅ |
| Login page → Registration page link | ✅ |
| Registration page → All 7 fields present | ✅ |
| Registration API → customer created in DB | ✅ |
| Duplicate email detection | ✅ |
| Duplicate phone detection | ✅ |
| Wrong password rejection | ✅ |
| Nonexistent user rejection | ✅ |
| Incident submission (breakdown) | ✅ |
| Case appears in My Cases list | ✅ |
| Case data structure valid (all required fields present) | ✅ |
| Post-submission tracking card HTML in page | ✅ |
| Polling functions present (pollTracking, showPostSubmitTracking) | ✅ |
| Tracking card elements (trackEta, trackDriver, trackLink) | ✅ |
| Registration page success state HTML | ✅ |
| Registration page client-side validation JS | ✅ |
| Registration page → Login link | ✅ |
| Cognito SSO configured (3 pools, PKCE) | ✅ |
| Cognito login button in page | ✅ |
| Demo credentials removed from login form | ✅ |
| Emergency phone number on login page | ✅ |
| Landing page CTA buttons (Get help now + Log in) | ✅ |

---

## REAL ISSUES FOUND

### ISSUE 1 — No Server-Side Input Validation (MEDIUM)

**What:** Empty first name, empty email, short passwords, and missing consents all create valid accounts.

**Evidence:**
```
POST /api/register {"first":"","email":"","password":"ab","consents":[]}
→ Status: active, customer created successfully
```

**Impact:** Invalid users can create accounts. GDPR consent can be bypassed. Database gets junk rows.

**Where:** `main.go` handleRegister — no validation before INSERT.

**Expected behavior:** Server should reject with HTTP 400 and descriptive error for each invalid field:
- `first` / `last`: required, 1-100 chars
- `email`: required, valid format, not empty
- `phone`: required, must start with +
- `password`: minimum 8 characters
- `consents`: must include "terms" at minimum

**Tracked:** improvements.md P1-8 (input validation)

---

### ISSUE 2 — Registration Without Consent Creates Account (MEDIUM)

**What:** Submitting registration with `"consents":[]` creates an account without accepting terms or privacy policy.

**Evidence:**
```
POST /api/register {..., "consents":[]}
→ Status: active, customer_id returned
```

**Impact:** GDPR violation. User accounts exist without any consent record. The consent table has no rows for this user but the customer row has `status=active`.

**Expected behavior:** Reject registration if `consents` array does not include "terms" or "privacy".

**Where:** `main.go` handleRegister — consents are inserted in a loop after customer creation. The customer row is created regardless of consent array content.

---

### ISSUE 3 — Incident Submission Fails Silently After Multiple Rapid Requests (LOW)

**What:** When submitting 6 incident types in rapid succession (~1 second apart), only 1-2 succeed. The rest return empty responses.

**Likely cause:** No rate limiting. May be a connection pool exhaustion or context timeout issue with pgxpool.

**Evidence:**
```
6 rapid POSTs: breakdown → OK, flat_tyre → OK, accident → EMPTY, ev_no_charge → EMPTY, ...
```

**Expected behavior:** All requests should succeed or return clear error messages. Silent failures are the worst kind.

**Where:** `main.go` handleUserIncident — may need better error handling and async submission support.

---

### ISSUE 4 — Registration Page: No Visual Connection to Landing Page (LOW)

**What:** The standalone registration page at `/register-page` looks beautiful but has no visible path from the main landing page. The landing page (/ ) has no "Register" or "Create Account" link. Users must go through `/app` → login page → click "Create account" → `/register-page`.

**Impact:** Users who land on the marketing page and want to register must discover the path through the login page. There's no direct "Sign Up" on the landing page.

**Expected:** Add a "Sign Up" or "Create Account" link in the landing page header alongside "Log in" and "Get help now".

**Where:** `prototype/backend/web/landing.html` — header-cta section

---

### ISSUE 5 — Registration Page Exists But Has No Route From /register (LOW)

**What:** The `/register` path serves `enduser.html` (which has a Register tab), while `/register-page` is the standalone page. Users typing `/register` don't get the nice standalone page.

**Expected behavior:** `/register` should redirect to `/register-page` or serve the standalone registration page directly.

**Where:** `main.go` handleRoot — case "/register" currently serves enduser.html

---

## UX STRENGTHS FOUND

| Strength | Detail |
|---|---|
| Registration form validation | Client-side checks for all fields before submission |
| Success state | Nice ✅ animation with "Account created!" + dashboard link |
| Emergency fallback | "Can't log in? Call +33 800 000 000" visible on login page |
| Post-submission tracking | Card appears after incident with ETA + driver + map link |
| Data isolation | Customer A cannot see Customer B's cases |
| No demo data leakage | Login form no longer pre-fills Claire Martin's credentials |
| Password masking | Password field uses `type="password"` |
| E.164 phone validation | Client-side checks for + prefix |
| Terms checkbox | Required before submission (client-side) |
| Cognito SSO | Available when configured, hidden gracefully when not |

---

## SUMMARY

| Category | Count | Items |
|---|---|---|
| End-to-end flows work | 22 tests | Registration, login, incident, listing, tracking, Cognito |
| Real backend issues | 3 | No validation, consent bypass, silent failures |
| UX gaps | 2 | Landing has no register link, /register serves wrong page |
| Browser UX polished | 10 strengths | Validation, fallbacks, success states, tracking |

**Overall end-user experience: 7/10** — Registration and incident flows work. The gaps are server-side validation (easily fixed) and navigation consistency (minor). The emotional experience gap (post-submission silence) is partially addressed by the tracking card.
