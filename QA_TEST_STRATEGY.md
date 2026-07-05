# Insucar — QA Test Strategy & Industry-Standard Methods

**Author:** QA/UX (headless-browser audit, 2026-07-05)
**Why this exists:** A **CRITICAL** defect (operator console 100% broken by a JS `SyntaxError`) shipped to production and survived every prior QA cycle because those cycles tested the operator/admin surfaces with `curl`/API calls only — **never a real browser executing the page JavaScript**. This document defines the test methods that were missing and must be adopted.

---

## 1. The gap that let a showstopper ship

| Prior method | What it verified | What it MISSED |
|---|---|---|
| `curl` API calls | Backend handlers, auth, DB | 100% of front-end JS execution |
| DOM-existence checks | Elements present in HTML | Whether the page's `<script>` even parses/runs |
| "It returns 200" | HTTP status | Uncaught JS errors, dead handlers, blank tabs |

**Rule 0: If a human uses it in a browser, a browser must test it.** API tests are necessary but never sufficient.

---

## 2. Test pyramid (target)

```
        /\        E2E (headless browser, real user journeys)   <-- was largely absent for operator/admin
       /  \       Integration (API + DB contract tests)
      /____\      Unit (Go handlers, JS pure functions)
     /______\     Static analysis / linting / type checks      <-- was absent for inline page JS
```

---

## 3. Mandatory gates (add to CI — `ci/Jenkinsfile`)

### 3.1 Static analysis of page scripts (would have caught #66 OP-SYNTAX, #67 ADMIN-TABS class)
- Extract every inline `<script>` from `web/*.html` and run `node --check` (parse) — **fail the build on any SyntaxError.**
- Run **ESLint** (`no-undef`, `no-unused-vars`, `no-unreachable`) on page scripts.
- Run an **HTML validator** (`html-validate`) — catches duplicate ids, unclosed tags.

### 3.2 Headless-browser smoke test for EVERY surface (landing, /app, operator, admin, register, status)
For each page assert:
- **Zero uncaught JS errors** — `page.on('pageerror')` must be empty. (Catches #66.)
- **Zero console errors** — `page.on('console', … type==='error')` empty, allow-list known. (Catches #68 PWA-404, #67 admin TypeError.)
- **No 4xx/5xx** for same-origin requests during load. (Catches #68.)
- **Critical handlers defined**: `typeof window.<fn> === 'function'` for the page's onclick handlers.

### 3.3 Persona journey E2E (Playwright/Puppeteer)
- **Client:** land → get help → register → login → **request tow (press & hold)** → tracking card appears → My Cases populated → logout.
- **Operator:** login → queue loads → open case (screen-pop) → triage → **dispatch tow** → mission/driver/SMS shown → resolve → logout.
- **Admin:** session → each of 4 tabs renders its view (state isolation: exactly ONE view visible) → create operator (list grows) → rate-limit toggle → RBAC: non-admin gets 403.
- Assert **redirects/navigation** at each auth transition (see §4).

### 3.4 Accessibility — `axe-core` per page (would catch #71)
- Fail on serious/critical violations; track moderate. Enforce labels, alt text, focus-visible, 44×44 targets, contrast ≥4.5:1.

### 3.5 Responsive / visual regression (would catch #70)
- Load each page at **375 / 768 / 1440**; assert **no horizontal scroll** (`scrollWidth <= clientWidth`); pixel-diff against baselines.

### 3.6 Link checker (would catch #69)
- Crawl landing/footer; fail on `href="#"` or 404 targets for real nav (allow explicit "#top" skip-nav).

### 3.7 API contract tests (catches the field-name drift class)
- Assert request/response field names against a schema. Historic drift bugs of this class: `cognito_subject` vs `keycloak_subject`, `caseID` vs `case_id`, ambiguous SQL columns, `data-view` vs div id. A contract/schema test freezes these.

---

## 4. Redirect & navigation assertions (explicitly required)

The app is a **SPA** — most "page changes" are client-side div swaps, **not** URL navigations. Document and assert the expected behavior:

| Transition | Expected | Verified |
|---|---|---|
| Landing `/` → "Get help now" | real nav to `/app` | ✅ |
| `/app` login success | **no URL change**; auth card hides, dashboard shows | ✅ (div swap) |
| `/app` refresh after login | session restored via `/api/me`; stays on dashboard | ✅ |
| `/app` logout | dashboard hides, login card returns (no URL change) | ✅ |
| Operator login | overlay `#loginOverlay` display:none; console shows | ❌ blocked by #66 |
| Admin tab click | target `…View` un-hides; exactly one view visible | ❌ 2/4 tabs #67 |
| SMS `/status/<token>` | serves tracking page (200) | ✅ (fixed earlier) |

**Assertion pattern:** capture `page.on('framenavigated')` + verify the correct div is the *only* visible state after each transition (state-isolation check).

---

## 5. Test-design hygiene (lessons from this run)

- **Target the RIGHT control.** `clickByText('Log in')` matched a nav *tab* button, not the form submit — caused false CRITICALs. Prefer stable hooks: click by `onclick`/`data-testid`, not visible text. **Add `data-testid` to all interactive elements.**
- **Be rate-limit aware.** Repeated test runs tripped the app's own per-endpoint rate limits (429), producing flaky "tracking card missing" false negatives. Space requests, use distinct source IPs per case, or exempt a test CIDR.
- **Visibility ≠ existence.** Always check computed style + bounding box (`offsetHeight>0`), never just `querySelector`.
- **Accumulate results across runs.** Per-process result arrays overwrote the issues file; use an append-only store keyed by issue id.

---

## 6. Definition of Done (per surface, before "works")
- [ ] 0 uncaught JS errors, 0 unexpected console errors, 0 same-origin 4xx/5xx on load
- [ ] All onclick handlers defined; every button/menu performs its action (asserted)
- [ ] Persona E2E green; state isolation holds on every transition
- [ ] axe-core: no serious/critical; no horizontal scroll at 375/768/1440
- [ ] Screenshots captured at each state; diffed vs baseline
