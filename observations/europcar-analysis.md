# Europcar Roadside Assistance — Technical Analysis

> Reconnaissance via read-only inspection (HTTP headers, DNS, rendered markup).
> No intrusive scanning performed. Date: 2026-07-03.

## Detected Technology Stack (evidence-based)

| Layer | Finding | Evidence |
|---|---|---|
| Frontend | Nuxt.js (Vue 2 / SSR), statically generated + hydrated; in-house design system (`os-` prefixed components) | `data-n-head-ssr`, `id="__nuxt_static"`, `/_nuxt_static/*.modern.js`, PWA `manifest.json` |
| CMS | Contentful (headless) | assets served from `images.ctfassets.net` |
| CDN / edge | Akamai | CNAME chain `*.edgekey.net`, `*.akamaiedge.net` |
| Bot / WAF | Akamai Bot Manager | `_abck`, `bm_sz` cookies |
| Web tier | nginx behind AWS ALB | `server: nginx`, `AWSALB` / `AWSALBCORS` cookies |
| APIs | `api.europcar.com` on Akamai; internal name `caas_platform_clusters` (Content/Commerce-as-a-Service) | DNS CNAME chain |
| Observability | Dynatrace RUM/APM | `x-ruxit-js-agent`, `x-oneagent-js-injection`, `server-timing: dtTrId/dtRpid` |
| Legacy core | Likely a legacy .NET "Dotcar" reservation backend still present | `robots.txt` disallows `/DotcarClient/` |
| Security | HSTS preload, TLS, referrer-policy | response headers |
| Marketing | Separate domains `europcar-infos.com` (newsletter), Facebook app id | footer links |

## Key Competitive Insight

There is **no dedicated public assistance web app or subdomain** — `assistance.europcar.com`
does **not** resolve (NXDOMAIN). Assistance is surfaced through `/p/customer-support` and
`/contact-us` content pages plus **country-specific phone numbers**. It is a
phone / PDF / agent-driven process, not a self-service digital product.

**This gap is the biggest opening for a competitor.**

## SWOT-style Analysis

### Software
- **Positives:** modern SSR frontend (good SEO + perceived speed); headless CMS decouples
  content from code; reusable in-house component library; PWA-capable; strong RUM instrumentation.
- **Negatives:** Vue 2 / Nuxt 2 lineage is EOL-era; heavy JS bundle (many chunks); legacy .NET
  reservation core implies dual-stack maintenance and integration debt; no assistance-specific
  software product at all.

### Infrastructure
- **Positives:** Akamai CDN + AWS ALB = proven scale / DDoS posture; Akamai Bot Manager for
  scraping/fraud defense; multi-region multi-domain footprint (per-country TLDs).
- **Negatives:** many moving parts (Akamai + AWS + Contentful + Dynatrace + legacy DC) = high
  vendor cost and operational surface; per-country domain sprawl adds config/cert overhead.

### Architecture
- **Positives:** clear separation — edge -> SSR web -> CaaS API -> legacy core; content managed
  independently; observability baked in.
- **Negatives:** monolithic legacy booking core is a bottleneck; no evidence of an
  event-driven / real-time layer (no WebSocket/geo-dispatch signals) — live tracking, dispatch,
  and telematics are not first-class, which is exactly what a modern assistance product needs.

### End-user
- **Positives:** fast landing pages, localized, familiar booking flow, mobile app promoted.
- **Negatives:** assistance is a phone-tree + fine-print experience — no live ETA, no in-app
  breakdown request, no real-time tracking, no status transparency. Bot protection can add
  friction. Dated UX for emergencies.

## Where a Competitor Wins

1. **Self-service digital assistance** — in-app breakdown request with GPS, not a phone number.
2. **Real-time dispatch + live ETA tracking** (the missing event-driven layer).
3. **Transparent status & pricing** during an incident.
4. **Modern, event-driven architecture** (no legacy core anchor) -> faster iteration.

## Suggested v1 Scope for Insucar

- Customer request app (breakdown request + GPS location).
- Geo-dispatch / matching backend (nearest available provider).
- Live ETA tracking (real-time map).
- Recommended stack: event-driven backend (Node/TS or Go + WebSockets/Kafka),
  Postgres + PostGIS for geo, React / React Native clients.
