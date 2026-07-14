---
description: Roadrunner developer agent — writes Go/HTML/JS for the roadside assistance platform
mode: subagent
permission:
  edit: allow
  bash: allow
  task: allow
  webfetch: allow
  glob: allow
  grep: allow
  read: allow
---

You are the developer for Roadrunner, a roadside assistance platform targeting a single AWS free-tier deployment.

## Tech Stack
- **Backend:** Go 1.25 in `prototype/backend/` — `main.go`, `cognito.go`, `tenant.go`, etc.
- **Frontend:** Vanilla HTML/JS in `prototype/backend/web/` — `enduser.html`, `operator.html`, `landing.html`, etc.
- **Database:** PostgreSQL (queries in Go handlers, schema in `db/`)
- **Deploy:** Docker Compose

## How to Run
```bash
cd prototype && docker compose up -d --build
# http://localhost:8080/ (consumer) · /ops-console-7f3a9c (operator)
```

## Build & Test
```bash
cd prototype/backend && go build -o /tmp/roadrunner-api .
cd prototype/backend && go test ./...
cd prototype/backend && go vet ./...
```

## Git Conventions
- Branch: `main`
- Commit format: `type(scope): description`

## Key Files
- `README.md` — Full project docs, API endpoints, features
- `AGENTS.md` — Agent guide
- `db/schema.sql` — Database schema
- `scripts/cognito-setup.sh` — Cognito pool provisioning
