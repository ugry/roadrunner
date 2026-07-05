# Insucar — Lessons Learned

**Date:** 2026-07-05  
**Context:** Full-day platform audit, implementation, QA testing, and CI/CD pipeline work  

---

## TECHNICAL LESSONS

### Lesson 1: Missing `.hidden` CSS class breaks entire page

**What happened:** The `register.html` page used `class="card hidden"` on the success state card, but the `.hidden { display: none }` CSS rule was never defined in the page's `<style>` block. The "Account created!" success message was always visible alongside the registration form.

**Why it was missed:** QA tests checked for DOM element existence (`page.$('#r_first')`), not visual rendering state (`window.getComputedStyle(el).display`). The elements existed in the DOM, so tests passed. A human sees both cards simultaneously and knows it's broken.

**The fix:** One line — `.hidden{display:none!important}` added to register.html CSS.

**Prevention:** Pre-flight visual test that creates a test `<div class="hidden">` and verifies `getComputedStyle().display === 'none'`. This 5-line test catches ALL CSS class bugs before any functional test runs.

**Rule:** Every new HTML page must pass the CSS class effectiveness test on first deploy. Never assume a CSS class exists — verify it.

---

### Lesson 2: `kubectl set image` erases `kubectl set env` variables

**What happened:** After deploying a new Docker image with `kubectl set image`, all Cognito environment variables (set earlier with `kubectl set env`) were lost. The deployment started with `cognito=false`, breaking SSO login and JWT verification.

**Why:** `kubectl set image` triggers a rolling update that creates new pods from the deployment template. The template only contains env vars from the ConfigMap and the original spec — manually injected env vars (`kubectl set env`) are ephemeral and don't survive image updates.

**Prevention:** Always store environment variables in a Kubernetes ConfigMap and reference them via `envFrom.configMapRef` in the deployment YAML. Never use `kubectl set env` for production configuration.

**Rule:** Cognito env vars must be in `insucar-config` ConfigMap, referenced by the deployment spec permanently.

---

### Lesson 3: Go in-memory maps are lost on pod restart

**What happened:** Rate limits configured through the admin page were stored in a Go `var rateLimits = map[string]rateLimitConfig{}`. On pod restart (new deploy, node failure, OOM kill), all custom rate limits reverted to hardcoded defaults.

**Why:** In-memory state is ephemeral. Kubernetes pods can restart at any time. Any configuration modified at runtime must be persisted.

**Prevention:** Store rate limits in a database table (`rate_limits`). Load from DB on startup (`loadRateLimits()`). Write to DB on every update (`INSERT ON CONFLICT DO UPDATE`). The in-memory map becomes a read cache, not the source of truth.

**Rule:** Any admin-configured setting must survive pod restart. Use DB tables, not Go variables.

---

### Lesson 4: Spinnaker pipeline can silently deploy wrong code

**What happened:** The Spinnaker pipeline config in S3 had `image: insucar-api:2` hardcoded in all three deploy stages. Jenkins correctly built and tagged images with the BUILD_NUMBER (e.g., `:11`, `:12`, `:13`). The webhook correctly sent `{"parameters":{"imageTag":"13"}}`. But Spinnaker ignored the parameter and deployed image `:2` — code from July 3rd. For 2 days, every Jenkins build produced the right artifact and every Spinnaker deploy deployed the wrong one.

**Why:** The pipeline JSON uses `${parameters.imageTag}` syntax, but it was deployed with a hardcoded value. The S3 config was stale. `limitConcurrent: true` blocked new pipeline executions because a stuck manual judgment from July 3rd was never approved.

**Prevention:** Verify pipeline config in S3 after every update. Use `aws s3 cp` to check the deployed config, not just the local file. Set `limitConcurrent: false` for dev/UAT stages.

**Rule:** Never trust that what's in git is what's deployed. Verify S3 directly.

---

### Lesson 5: `CHAR(2)` column rejects `"Other"` — raw SQL errors reach users

**What happened:** The registration page has `<option>Other</option>` in the country dropdown. When selected, the literal string "Other" (5 chars) is sent to the API. The database column `country_code CHAR(2)` rejects it with: `ERROR: value too long for type character(2) SQL state 22001`. This raw PostgreSQL error reaches the end user.

**Why:** No server-side validation on country_code before the INSERT. The `nz(in.Country, "FR")` helper only replaces empty strings — "Other" is not empty, so it passes through. Three layers of failure: frontend offers invalid option, backend doesn't validate, database rejects with raw error.

**Prevention:** Server-side validation must run BEFORE every database write. Check `len(country) == 2` and `country matches [A-Z]{2}`. Return a user-friendly error: `"Country code must be 2 letters (ISO format). Received: 'Other'"`. Frontend should never offer invalid options.

**Rule:** Raw database errors must NEVER reach the end user. Every INSERT must be preceded by validation.

---

## QA METHODOLOGY LESSONS

### Lesson 6: DOM existence ≠ visual rendering

**What happened:** The `qa-full.mjs` test suite checked for DOM elements with `page.$('#element')`. This confirms the element exists in the DOM tree. It does NOT confirm the element is:
- Visible (`display:none`, `visibility:hidden`, `opacity:0`)
- Properly sized (`width:0`, `height:0`)
- Not covered by another element (z-index bugs)
- On-screen (positioned off-viewport)
- Readable (color matching background)

The registration page bug was missed twice because the success card existed in the DOM — `page.$('#successCard')` returned the element. The test never checked whether it was visible.

**Prevention:** Every state-dependent test must check `window.getComputedStyle(el).display !== 'none'`. Created `QA_visual_test_directives.md` with 6 mandatory visual test types.

**Rule:** If a human can see it, the test must check for it. DOM queries test what robots see. Computed style checks test what humans see.

---

### Lesson 7: Curl session cookies are unreliable in automated tests

**What happened:** Session-dependent API tests using `curl -c` (capture cookies) and `curl -b` (send cookies) with `grep`/`awk` parsing failed intermittently. The Netscape cookie format is tab-separated with 7 fields; `awk '{print $NF}'` doesn't reliably extract the cookie value. This caused false negatives — valid session flows reported as "unauthorized".

**Prevention:** Use a proper HTTP client library (Python `requests.Session()`, Node.js `fetch` with cookie jar) for session-dependent API tests. Shell curl is fine for single-request tests but not for multi-step authenticated flows.

**Rule:** Curl for simple API calls. Real HTTP clients for session flows.

---

### Lesson 8: Pre-flight visual checks catch bugs before functional tests

**What happened:** Three pre-flight tests were added to `qa-full.mjs`:
1. V0: `.hidden` CSS class actually works (creates test div, checks computed display)
2. V0b: Registration page state isolation (form OR success visible, never both)
3. V0c: Operator overlay blocks console interaction (computed style check on overlay)

On the very first run, V0b confirmed the `.hidden` fix was deployed. V0c confirmed operator security. V0 caught that the landing page doesn't define `.hidden` (a false positive but valuable — the test methodology works).

**Prevention:** Pre-flight checks must run BEFORE any functional test. If `.hidden` doesn't work, every state-dependent test will produce false results. Catch the CSS bug first, then test the functionality.

**Rule:** Run visual infrastructure tests first. Functional tests second.

---

### Lesson 9: Headless browser must simulate a human, not a DOM parser

**What happened:** The original `authflow.mjs` and `opflow.mjs` test scripts clicked buttons and waited for functions, but never verified what a user sees. They checked API responses but not page rendering. This is why the registration page bug survived multiple QA rounds — the tests confirmed the API works, but never looked at the screen.

**Prevention:** Every headless browser test must include at minimum:
- Screenshot at key state transitions
- Computed style check on visibility
- Content leak check (forbidden strings in wrong states)

**Rule:** Test what the user sees, not what the DOM contains.

---

## PROCESS LESSONS

### Lesson 10: Bulk-creating then bulk-closing issues is not tracking

**What happened:** 36 GitHub issues were created for all research findings, then closed en masse. The P0 items (emergency FAB, keyboard shortcuts, WebSocket, what3words) were never implemented but the issues are closed. This creates a false sense of completion — "0 open issues" but critical features are missing.

**Prevention:** Issues should track WORK, not IDEAS. Open issues represent things that need to be done. Close issues ONLY when the work is complete and verified. If you want to track ideas, use a separate label or project board.

**Rule:** Open = not done. Closed = done. Never close an issue without implementing the fix.

---

### Lesson 11: 15+ overlapping markdown files cause confusion

**What happened:** At peak, the repo had 15+ markdown files with overlapping content. The same P0-X fixes appeared in `improvements.md`, `expertobservations.md`, AND `expertexpert.md`. The AWS account number appeared in 9 files. The CI/CD pipeline description appeared in 8 files with 12+ mentions. Different files claimed different live image tags (`:11`, `:12`, `:15`, `:qa-fix`, `:casecards`).

**Prevention:** Single source of truth. README.md holds live state. `improvements.md` holds the improvement register. Everything else references these two. Archived historical documents (`build-notes.md`, `milestone.md`) to `archive/`.

**Rule:** If the same fact appears in 3+ files, delete it from 2 of them and point to the canonical source.

---

### Lesson 12: Documentation drifts from reality fast

**What happened:** `CONTINUE-HERE.md` had two different image tags 30 lines apart (`:12` on line 21, `:15` on line 48). It referenced an EC2 prototype that was superseded weeks ago. It claimed Cognito env vars were "unset in live deploy" when they were set. It claimed dev rollout was "PROVEN RUNNING" when it was torn down.

**Prevention:** Update documentation when you deploy. Or better: don't put live state in documentation at all. Put it in the README and update it with every deploy.

**Rule:** Live state in docs = stale data guaranteed. Use `kubectl get deploy` to check what's actually running.

---

### Lesson 13: `crane append` doesn't set ENTRYPOINT — container runs `CMD ["/bin/sh"]`

**What happened:** Images built with `crane append` inherited Alpine's default CMD (`/bin/sh`). When Kubernetes started the container, it ran `/bin/sh` which exited immediately (no stdin). The pod went into CrashLoopBackOff because the entrypoint wasn't set. Required an extra `crane mutate --entrypoint /app/insucar-api` step.

**Prevention:** After every `crane append`, run `crane mutate` to set the entrypoint. Or use `crane config` to verify the image config before deploying.

**Rule:** After building an OCI image without Docker, always verify the config with `crane config`.

---

### Lesson 14: `OPTIONAL` volumes in Kubernetes still block pod creation when secret is missing

**What happened:** The Jenkinsfile had an `optional: true` cosign-key volume. When the cosign secret didn't exist, the container failed with `CreateContainerConfigError`. The `optional` flag only affects volume mounting, not container creation — if the secret doesn't exist, the container referencing it cannot start.

**Prevention:** Either create the secret (even if empty/dummy) or remove the volumeMount from the container spec. The `optional` flag on the volume definition doesn't make the container optional.

**Rule:** If a container mounts a secret, that secret must exist. Period.

---

### Lesson 15: EKS node-specific pod failures go undetected

**What happened:** Node `ip-192-168-77-227.eu-west-1.compute.internal` consistently failed to run the insucar-api pod. The same image worked on the other node. No monitoring, no alerts. The CrashLoopBackOff persisted for hours before manual detection.

**Prevention:** Deploy monitoring (Prometheus Agent) with pod restart alerts. If a pod restarts more than 3 times in 10 minutes, page the on-call.

**Rule:** If you can't see it failing, it will fail silently until someone checks manually.

---

### Lesson 16: A CRITICAL production defect can hide from curl-only QA

**What happened:** The **operator console was 100% broken in every browser** — a JavaScript `SyntaxError: Unexpected token '}'` (orphaned dead code after `submitTriage()`, extra `}`) prevented the entire inline `<script>` from parsing, so every handler (`alogin`, `incoming`, `submitTriage`, `psap`, `alogout`, `dispatch`) was `undefined`. An operator could not log in, see the queue, dispatch a tow, or log out. This shipped in commit `596bd77` and survived many QA cycles.

**Why it was missed:** All prior operator QA used `curl`/API calls, which never load or execute the page's JavaScript. The backend APIs all returned 200, so curl-based tests were green while the UI was dead. GitHub #66.

**The fix / prevention:**
- Run a **headless browser smoke test on EVERY surface** (not just end-user) asserting `page.on('pageerror')` is empty.
- Add **static JS parsing to CI**: extract inline `<script>` blocks and run `node --check` / ESLint — fail the build on any SyntaxError.

**Rule:** API-green ≠ app-works. If a human uses it in a browser, a browser test must execute its JavaScript.

---

### Lesson 17: Kebab-case `data-*` vs camelCase element ids silently breaks tabs

**What happened:** Admin "Rate Limits" and "API Access" tabs rendered **blank**. The handler did `el(b.dataset.view + 'View').classList.remove('hidden')`; `data-view="rate-limits"` looked up `#rate-limitsView` but the div was `#rateLimitsView` → `el()` null → `TypeError: Cannot read properties of null`. All views hidden, none shown. GitHub #67.

**Prevention:** Keep `data-view` values identical to id fragments, or normalize kebab→camel; null-guard before `.classList`. A headless test asserting "exactly one view visible after each tab click" catches this.

**Rule:** Any string used to look up an element must be contract-tested against the actual id.

---

### Lesson 18: Referenced static assets must be routed (PWA 404s)

**What happened:** `enduser.html` references `/manifest.json` and registers `/sw.js`, but the Go router serves neither → 404 on both hosts, service worker never registers, manifest parse error, console errors every load. Files existed in `web/` but had no route. GitHub #68.

**Prevention:** A console-error / network-404 assertion in the headless smoke test fails when any referenced asset 404s.

**Rule:** Every `href`/`src`/`register()` target must have a route + a test that fetches it.

---

### Lesson 19: Tests must target the right control and respect rate limits

**What happened:** (a) `clickByText('Log in')` matched the top-nav *tab* button (`showTab`) instead of the form submit (`login()`), producing false CRITICAL failures for login/cases/incident. (b) Repeated headless runs tripped the app's own per-endpoint **rate limits (429)**, causing a flaky false "tracking card missing".

**Prevention:** Add `data-testid` to interactive elements and click by that (not visible text). Design tests to be rate-limit aware (space requests, distinct IPs, or a test-exempt CIDR).

**Rule:** A false positive from a bad selector wastes as much time as a real bug — make selectors unambiguous and tests hermetic.

---

## SUMMARY

| # | Lesson | Category |
|---|---|---|
| 1 | Missing `.hidden` CSS class breaks visual state | Technical |
| 2 | `kubectl set image` erases `kubectl set env` | Technical |
| 3 | Go in-memory maps lost on pod restart | Technical |
| 4 | Spinnaker silently deploys wrong image | Technical |
| 5 | `CHAR(2)` rejects "Other" — raw errors reach users | Technical |
| 6 | DOM existence ≠ visual rendering | QA |
| 7 | Curl session cookies unreliable | QA |
| 8 | Pre-flight visual checks catch bugs early | QA |
| 9 | Headless browser must simulate a human | QA |
| 10 | Bulk-close issues = false completion | Process |
| 11 | 15+ overlapping docs cause confusion | Process |
| 12 | Documentation drifts from reality fast | Process |
| 13 | crane append doesn't set ENTRYPOINT | Technical |
| 14 | Optional volumes still block pod creation | Technical |
| 15 | Node-specific failures go undetected | Technical |
| 16 | CRITICAL front-end defect hidden from curl-only QA (operator console SyntaxError) | QA |
| 17 | Kebab `data-*` vs camelCase id silently breaks tabs | Technical |
| 18 | Referenced static assets (manifest/sw/favicon) must be routed | Technical |
| 19 | Tests must target the right control + respect rate limits | QA |

See also **`QA_TEST_STRATEGY.md`** for the full set of industry-standard test methods (static analysis of page scripts, per-surface headless smoke with zero-JS-error assertion, persona E2E, axe-core a11y, responsive/visual regression, link checking, API contract tests) that CI must adopt.

---

*Compiled from a full day of platform audit, implementation, QA testing, and CI/CD pipeline work on the Insucar roadside assistance platform.*
