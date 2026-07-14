# Roadrunner — Session Memory

> Single-server roadside assistance platform. Evolved from insucar (multi-AZ cloud) to roadrunner (AWS t2.micro).  
> Domain: **unysolar.com** (Route53) · Server: **34.255.180.21** (t2.micro, eu-west-1)  
> Repo: **https://github.com/ugry/roadrunner** (public)

---

## Timeline

### 2026-07-14 06:14 — Clone & Strip Cloud Infrastructure
- Cloned `ugry/insucar` to `/home/semyaza/roadrunner`
- **Why:** New direction — single micro server, no multi-AZ, no backup plans, no capacity planning
- Removed: `terraform/`, `spinnaker/`, `k8s/`, `ci/`, `mermaidschemas/`, `prompt/`, `observations/`, `qa-evidence/`, `archive/`, `worker/`
- Removed 20+ conflicting .md docs (audits, assessments, rollout, security rotation)
- Kept: `prototype/` (Go backend + HTML frontends), `db/` (schema), `scripts/`, `design/` (logos)
- `188 files → 51 files`

### 2026-07-14 06:25 — AWS Deployment
- Launched t2.micro instance `i-07fbdd1671491e9cc` in eu-west-1
- Key pair: `roadrunner-key`, Security group: `sg-0acd68c9714f75af3` (22, 80, 443, 8080)
- **Mistake:** First deploy failed because Go compilation + vet + test on t2.micro (1GB RAM) caused OOM
- **Fix:** Pre-built Go binary locally, simplified Dockerfile to just COPY binary → Alpine
- Installed Docker + docker-compose on server

### 2026-07-14 06:39 — Email Auth Implementation
- **What:** Added forgot password / reset password flow with Resend API
- Added `sendEmail()`, `sendResend()`, `sendSMTP()` functions
- Created `handleForgotPassword`, `handleSendResetEmail`, `handleResetPassword` handlers
- CSRF exemptions added for `/api/reset-password`, `/api/send-reset-email`
- **Mistake #1:** Resend API key `re_gyerjvod_...` was invalid (401). Later replaced with working `re_8je44ss6_...`
- **Mistake #2:** SMTP via Hostinger failed — password `1tq|zl>l` rejected on both port 465 and 587
- **Mistake #3:** Forgot password sent email even when customer didn't exist — created broken links
- **Fix:** Only send email if customer exists in DB. Return generic message otherwise.

### 2026-07-14 06:42 — DNS Setup
- Created Route53 hosted zone `Z04880512E8D5P1KC4YNV` for `unysolar.com`
- A records: `unysolar.com`, `www.unysolar.com`, `app.unysolar.com`, `op.unysolar.com` → `34.255.180.21`
- **Mistake:** Forgot to create `www.unysolar.com` A record initially — only had `unysolar.com`, `app`, `op`
- **Fix:** Added `www` record after user reported domain not working
- Namecheap nameserver update took ~30 min to propagate at .com registry

### 2026-07-14 06:44 — HTTPS via Let's Encrypt
- Installed certbot + python3-certbot-nginx
- SSL cert for all 4 subdomains (unysolar.com, www, app, op)
- Nginx config: HTTP → 301 HTTPS redirect, proxy to localhost:8080
- Cert auto-renews

### 2026-07-14 06:49 — DNS Resolution Issues
- **Mistake:** Local machine DNS resolver `10.192.64.58` returned SERVFAIL for unysolar.com
- Google DNS (8.8.8.8) resolved correctly
- **Fix:** Changed system DNS to Google DNS via NetworkManager (`nmcli connection modify`)

### 2026-07-14 07:00 — QA Testing (API + Browser)
- Comprehensive API testing: 60 tests, 58 passed
- Headless browser testing: 36 tests, 36 passed
- Issues: Landing/operator page titles still say "Insucar" not "Roadrunner" (cosmetic)

### 2026-07-14 07:24 — CSRF Bug Discovery
- **Bug:** JavaScript `api()` function never sent `X-CSRF-Token` header
- **Impact:** ALL POST requests from browser silently failed (login worked because it's CSRF-exempt)
- **Fix:** Added CSRF cookie reading + header sending to `bearerApi()` in all 4 HTML files
- Applied to: `enduser.html`, `operator.html`, `admin.html`, `register.html`

### 2026-07-14 07:30 — Service Worker Cache Issue
- **Bug:** Service worker `sw.js` used cache-first strategy — browser always loaded stale HTML
- **Impact:** Even after deploying CSRF fix, users saw old cached pages without the fix
- **Fix:** Changed cache name `insucar-v2` → `roadrunner-v1`, changed to network-first strategy
- **Key learning:** Always use network-first for dynamic pages, cache-first only for static assets

### 2026-07-14 08:00 — Structured Logging
- Replaced simple `slog.Info("request", ...)` with comprehensive event-based logging
- New `logEvent(event, key, value, ...)` pattern
- Logged events: `http`, `auth.login.success`, `auth.login.failed`, `auth.register`, `incident.created`, `dispatch.success`, `dispatch.failed`, `security.csrf.missing`, `security.csrf.mismatch`, `security.rate_limit`, `auth.logout`, `auth.password.upgraded`
- Log file: `/var/log/roadrunner/app.log` (persistent via Docker volume mount)
- Response writer wrapper captures status codes, bytes written, latency

### 2026-07-14 08:10 — DB Persistence Fix
- **Bug:** Postgres Docker volume was anonymous — wiped on every `docker compose down`
- **Impact:** All test users and data lost on every deploy
- **Fix:** Added named volume `pgdata` to docker-compose.yml

### 2026-07-14 08:16 — Incident Creation Debugging
- User tried creating incident on Android app → failed silently
- Logs showed no POST to `/api/user/incident` at all
- Root cause: Service worker caching old HTML without CSRF fix
- **Fix:** User needed Ctrl+Shift+R to bypass cache

### 2026-07-14 08:24 — E2E Flow Verified
- End-to-end test: Login → Wizard → Incident Created → Operator Queue
- Case #1784017485 successfully created, visible in operator console

### 2026-07-14 08:28 — 3-Step Wizard Redesign
- **Before:** 4 steps (Type → Location → Describe → Summary + Press & Hold)
- **After:** 3 steps (Type + Press & Hold → Location → Describe + Auto-Submit)
- Press & Hold moved to first screen, incident auto-creates on step 3 "Submit Request"
- `panicMode()` now calls `submitIncident()` directly

### 2026-07-14 08:30 — GPS Location Fix
- **Bug:** GPS coordinates stored as text in `address_text`, not in PostGIS `geog` column
- **Bug:** Operator case detail API didn't return location data
- **Fix:** Store GPS as `ST_SetSRID(ST_MakePoint(lng, lat), 4326)` in geog column
- **Fix:** Added location query + `location: {address, lat, lng, what3words}` to `handleAgentCase` response
- Operator map auto-pans to incident coordinates, drops red pin

### 2026-07-14 09:02 — Android App
- Created WebView-based Android app wrapping `https://www.unysolar.com/app`
- Package: `com.roadrunner.app`, minSdk 26, targetSdk 35
- Runtime location permission request on launch
- GitHub Actions workflow builds debug APK on push to main
- Installed on user's phone via ADB

---

## Current State

### Server
| Component | Detail |
|---|---|
| Instance | `i-07fbdd1671491e9cc`, t2.micro, 34.255.180.21 |
| OS | Ubuntu 24.04 |
| Docker | v29.1.3, docker-compose |
| Nginx | Reverse proxy, HTTP→HTTPS redirect |
| SSL | Let's Encrypt (auto-renew), expires 2026-10-12 |
| App log | `/var/log/roadrunner/app.log` (structured JSON) |
| DB | PostgreSQL 15 + PostGIS, persistent volume `pgdata` |

### Domains (Route53 Z04880512E8D5P1KC4YNV)
| Domain | Purpose |
|---|---|
| `unysolar.com` | Landing page |
| `www.unysolar.com` | Landing page |
| `app.unysolar.com` | User app |
| `op.unysolar.com` | Operator console (hidden path `/ops-console-7f3a9c`) |

### Accounts
| Email | Password | Role |
|---|---|---|
| `ugur.yardimci@unygms.com` | `UgurTest2026!` | User |
| `OP-1001` | `Operator#2026` | Operator (Amelie Durand) |
| `SUP-2001` | `Supervisor#2026` | Supervisor (Marc Petit) |
| `PO-3001` | `Owner#2026` | Product Owner (Sophie Bernard) |

### APIs / Keys
| Service | Key | Status |
|---|---|---|
| Resend | `re_8je44ss6_ASZfbyVMoeDqN8Zpzot2GWRR` | Working |
| Resend from | `roadrunner@unysolar.com` | Domain verified |
| AWS | Account 326804802908, eu-west-1 | Working |
| GitHub PAT | `ghp_***` (classic, repo scope) | Working |
| SMTP | `ugur.yardimci@unygms.com` / `1tq|zl>l` | NOT working |

---

## Key Learnings

1. **Never cache-first for dynamic pages.** Service worker `sw.js` cache-first strategy caused all CSRF/UI fixes to be invisible to users. Always use network-first for HTML, cache-first only for static assets.

2. **CSRF tokens must be explicitly sent.** The double-submit cookie pattern requires JavaScript to read the cookie and send it as a header. Just setting `credentials: 'include'` is not enough.

3. **Pre-build Docker images for micro instances.** Go compilation on t2.micro (1GB RAM) causes OOM. Always build binary on dev machine, use simple COPY Dockerfile.

4. **Docker compose down wipes anonymous volumes.** Always use named volumes for stateful services (Postgres). Every deploy was destroying the database.

5. **Test both registered AND unregistered email flows.** Forgot password sent emails to non-existent users, creating broken reset links. Always validate customer exists before creating tokens.

6. **DNS propagation is slow.** TLD nameserver changes at .com registry can take 30+ minutes. Old delegation had 48h TTL. Local resolvers cache negative answers.

7. **Structured logging saves debugging time.** Event-based logs (`event: "auth.login.failed"`) with queryable fields make it trivial to trace exact user flows. Without it, the CSRF + service worker issues would have taken hours to diagnose.

8. **PostGIS geography vs text fields.** GPS coordinates stored as text are useless for spatial queries. Must use `ST_MakePoint(lng, lat)` with SRID 4326 in the `geog` column.

---

## Deploy Commands

```bash
# Build Go binary
cd /home/semyaza/roadrunner/prototype/backend
export GOROOT=/home/semyaza/go && export GOPATH=/home/semyaza/go/packages
export PATH="/home/semyaza/go/bin:$PATH"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/roadrunner-api .

# Upload and deploy
cp /tmp/roadrunner-api /home/semyaza/roadrunner/prototype/backend/roadrunner-api
cd /home/semyaza && tar czf /tmp/roadrunner-deploy.tar.gz roadrunner/prototype/backend/roadrunner-api roadrunner/prototype/backend/web/
scp -i ~/.ssh/roadrunner-key.pem /tmp/roadrunner-deploy.tar.gz ubuntu@34.255.180.21:/home/ubuntu/
ssh -i ~/.ssh/roadrunner-key.pem ubuntu@34.255.180.21 '
  cd /home/ubuntu && tar xzf roadrunner-deploy.tar.gz
  cd /home/ubuntu/roadrunner/prototype && sudo docker compose down && sudo docker compose up -d --build
'

# View logs
ssh -i ~/.ssh/roadrunner-key.pem ubuntu@34.255.180.21 'sudo tail -f /var/log/roadrunner/app.log'

# Push to GitHub
cd /home/semyaza/roadrunner && git add -A && git commit -m "..." && git push origin main
```

## SSH Access
```bash
ssh -i ~/.ssh/roadrunner-key.pem ubuntu@34.255.180.21
```

## Android Build
- Repo Actions build APK automatically on push to main
- Download from: https://github.com/ugry/roadrunner/actions
- Install: `adb install -r app-debug.apk`
