# Roadrunner — Agent Guide

## Project
Roadside assistance platform (Go backend, Vanilla HTML/JS SPA). Single-server AWS free tier deployment.

## Stack
- **Backend:** Go 1.25, PostgreSQL/PostGIS
- **Frontend:** Vanilla HTML/JS (SPA), Cognito OAuth2 PKCE
- **Deploy:** Docker Compose on a single EC2 instance

## Project Layout
```
roadrunner/
├── prototype/backend/   # Go API monolith (main.go + handlers)
│   └── web/             # HTML/JS frontends (enduser, operator, landing, admin, register)
├── db/                  # PostgreSQL schema + seed files
├── scripts/             # cognito-setup.sh
├── design/              # Logo SVGs
└── prototype/docker-compose.yml  # Local + production deployment
```

## How to Run
```bash
cd prototype && docker compose up -d --build
# http://localhost:8080/ (consumer) · /ops-console-7f3a9c (operator)
```

## Git Conventions
- Branch: `main`
- Commit format: `type(scope): description` (e.g. `feat(auth): add Cognito SSO`)

## Key Files
- `README.md` — Full project docs, API endpoints, features
- `prototype/backend/main.go` — All Go handlers and routing
- `db/schema.sql` — Database schema
- `scripts/cognito-setup.sh` — Cognito pool provisioning
