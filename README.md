# Roadrunner — Roadside Assistance Platform

> Roadside assistance platform. Human-coordinator-first, emergency-first.
> Single-server deployment on AWS free tier: 1 EC2 micro instance + 1 PostgreSQL.

## Stack

- **Backend:** Go 1.25 (single binary), pgx, golang-jwt
- **Frontend:** Vanilla HTML/JS (SPA), Leaflet.js maps, Cognito PKCE OAuth2
- **Auth:** Amazon Cognito (3 pools: customer, staff, partner), RS256 JWT, group RBAC
- **Data:** PostgreSQL 15 + PostGIS
- **Deploy:** Docker Compose on a single EC2 t2.micro

## Repository Map

```
├── prototype/backend/     Go API monolith
│   ├── main.go            All handlers + routing + Cognito JWT
│   ├── cognito.go         RS256 JWKS verification + JIT provisioning
│   ├── cache.go           Redis cache (disabled by default)
│   ├── events.go          EventBridge integration (disabled by default)
│   └── web/               HTML/JS frontends (SPA)
│       ├── enduser.html   Consumer app: login · register · incident · cases · GPS
│       ├── operator.html  Dispatch console: queue · triage · dispatch · map · timeline
│       ├── landing.html   Marketing landing
│       ├── status.html    Live tracking page (Leaflet map + ETA countdown)
│       ├── register.html  Standalone registration page
│       ├── admin.html     Admin panel
│       └── cognito-callback.html  OAuth2 PKCE callback handler
├── db/                    PostgreSQL schema + seed files
├── scripts/               cognito-setup.sh (idempotent pool provisioning)
├── design/                Logo SVGs
├── prototype/uitest/      Headless browser QA suite
└── prototype/docker-compose.yml  Local + production deployment
```

## Quick Start

```bash
cd prototype && docker compose up -d --build
# http://localhost:8080/ (consumer) · /ops-console-7f3a9c (operator)
```

## API Endpoints

### Public
| Method | Path | Description |
|---|---|---|
| GET | `/healthz` | Liveness (DB check) |
| GET | `/` | Marketing landing |
| GET | `/app`, `/login`, `/register` | End-user app |
| GET | `/app/callback` | Cognito OAuth2 PKCE callback |
| GET | `/api/auth/config` | Cognito pool config |
| POST | `/api/register` | Customer registration |
| POST | `/api/telephony/mock/incoming` | Mock inbound call |
| GET | `/api/status/:token` | Live tracking |

### User (Requires Auth)
| Method | Path | Description |
|---|---|---|
| POST | `/api/user/login` | Email/password login |
| POST | `/api/user/incident` | Submit help request |
| GET | `/api/user/cases` | List own cases |
| POST | `/api/logout` | Clear session |
| GET | `/api/me` | Current identity |

### Agent/Operator (Requires Auth)
| Method | Path | Description |
|---|---|---|
| POST | `/api/agent/login` | Agent ID/password login |
| GET | `/api/agent/cases` | Case queue |
| GET | `/api/agent/case?id=` | Case detail |
| GET | `/api/agent/lookup?phone=` | ANI screen-pop lookup |
| POST | `/api/agent/dispatch` | Dispatch provider → create mission → send SMS |
| POST | `/api/agent/stats` | Queue statistics |
| GET | `/api/agent/providers` | Ranked provider list |

## Features

### End-User App
- Email/password login + Cognito SSO (PKCE OAuth2)
- Customer registration with consent
- 6 incident types: breakdown, accident, flat tyre, EV no charge, medical, other
- GPS auto-detect
- Case listing with status pills
- Immutable SHA-256 hash-chained audit ledger

### Operator Console
- Hidden path security (404 for wrong paths)
- Agent ID login + Cognito SSO (staff pool)
- Live case queue with color-coded priority pills
- Queue auto-refresh with SLA timers
- Screen-pop with auto-bind on incoming call
- Coverage decision panel
- Safety triage panel
- Provider ranking with scores
- One-click dispatch with SMS
- Interactive Leaflet.js map
- SLA breach detection (15min warn, 30min alert)
- Provider fallback chain

### Auth
- Amazon Cognito: 3 user pools (customer, staff, partner)
- RS256 JWT verification against Cognito JWKS
- Group-based role mapping
- JIT customer provisioning
- Demo cookie auth as fallback

## DB Setup

```bash
psql -U postgres -d insucar \
  -f db/schema.sql -f db/seed.sql -f db/schema-v3-additions.sql \
  -f db/schema-v4-auth.sql -f db/schema-v5-cognito.sql -f db/seed-users.sql
```

## Cognito Setup

```bash
bash scripts/cognito-setup.sh          # Create pools + domains + clients + users
bash scripts/cognito-setup.sh --destroy # Tear down
```

## AWS Deployment (Single Server)

1. Launch a t2.micro EC2 instance (Amazon Linux 2 or Ubuntu)
2. Install Docker and docker-compose
3. Clone this repo
4. Run `cd prototype && docker compose up -d --build`
5. Point a security group to allow ports 80/443
6. Optionally set up Let's Encrypt with a reverse proxy (nginx/Caddy)

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:test@db:5432/insucar?sslmode=disable` | PostgreSQL connection |
| `SESSION_SECRET` | `insucar-demo-session-key-change-me` | HMAC cookie secret |
| `COGNITO_ISSUER` | (empty) | Cognito pool issuer URL |
| `COGNITO_STAFF_ISSUER` | (empty) | Staff pool issuer |
| `COGNITO_PARTNER_ISSUER` | (empty) | Partner pool issuer |
| `PROVIDER_API_URL` | (empty) | External provider connector |
| `STATUS_LINK_BASE` | `https://app.unysolar.com/status` | Tracking link base URL |
