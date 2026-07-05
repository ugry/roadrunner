# Insucar — Improvement Register & Competitive Comparison

**Document type:** QA Engineering / Competitive Analysis  
**Date:** 2026-07-05  
**Sources:** Redion, Europcar, Hexagon CAD, Motorola CallWorks, Agero/Swoop, Autura, Towbook, AAA, industry CAD standards  
**Comparison target:** Insucar EKS live deployment (image `insucar-api:11`)

---

## 1. HOW COMPETITORS WORK — END-USER (CUSTOMER) APPLICATIONS

### 1.1 Redion (Europ Assistance / Generali Group)
The global leader in roadside assistance. 200+ countries, 12,000 providers, 35,000 trucks, 80% <45min arrival.

**Mobile App Features:**
| Feature | Purpose | Insucar Status |
|---|---|---|
| **Incident-type selector** | User picks: Start problem, Accident, Flat tyre, Other breakdown — helps route to correct response team | Covered (6 types in #i_type dropdown) |
| **Accurate geolocation** | GPS pin sent with request, no verbal description needed | Covered (address field present, but no GPS button) |
| **Real-time tow-truck geo-tracking** | Live map showing provider vehicle approaching — reduces anxiety | MISSING — no map in end-user app |
| **Status link via SMS** | Clickable link to live tracking page if app unavailable | Covered (sent via SNS) |
| **Driver identity preview** | Shows tow driver name, plate number, photo before arrival — safety/trust | Partially covered (name/plate shown in operator dispatch only) |
| **Local-language assistance centers** | Per-country operators speaking the caller's language | Covered (11 languages architected) |

**Service Catalog (what they offer that Insucar doesn't yet):**
| Service | Purpose |
|---|---|
| Vehicle repatriation | Cross-border recovery — car transported across country borders to home or preferred garage |
| Tyre protection | Puncture replacement covered including valve and labor — product add-on |
| EV → ICE car swap | Customer swaps EV for petrol car for long trips via rental network |
| Service-activated RSA | RSA renewed 12 months when car serviced at authorized garage (loyalty loop) |
| Car pick-up & delivery | Doorstep-to-workshop concierge |
| Micromobility assistance | Bikes, e-bikes, e-scooters, monowheels, segways — growing segment |

### 1.2 AAA / Agero / Allstate Patterns
| Feature | Purpose | Insucar Status |
|---|---|---|
| **One-tap emergency button** | Single button to request help; auto-attaches GPS, vehicle, policy | Covered (Request Assistance button) |
| **Photo upload** | User photographs damage/scene for operator assessment | MISSING |
| **Estimated arrival countdown** | Live ETA timer counting down minutes | MISSING — only text-based "Case #XXX — triaging" |
| **Provider chat/messaging** | In-app communication with tow driver | MISSING |
| **Service history** | List of all past assistance requests with details | Covered (My Cases with card layout) |
| **Post-service rating** | Rate provider after service completion | MISSING |
| **Connected vehicle auto-trigger** | eCall/bCall — car automatically calls when airbag deploys or crash detected | MISSING |

### 1.3 Europcar — What NOT to do
Europcar has **no digital assistance product at all** — assistance is phone+PDF+agent-driven only. This is the competitive gap Insucar exploits, but Insucar must go ALL-IN on digital to maintain the advantage.

---

## 2. HOW OPERATOR CONSOLES WORK — INDUSTRY STANDARDS

### 2.1 Reference Systems
- **Hexagon HxGN OnCall Dispatch** — CAD system tailored specifically for roadside assistance (tow/tyre/battery/jump-start)
- **Motorola CallWorks + Spillman Flex CAD** — Integrated call-taking + dispatch single screen
- **Autura (formerly Omadi)** — Heavy-duty towing dispatch with embedded payments
- **Agero/Swoop** — Algorithmic dispatching using 100+ TB of historical data
- **Towbook** — 20+ motor-club integrations, drag-and-drop dispatch board

### 2.2 Universal Dispatch Console Layout

Every professional dispatch console follows the same 3-panel layout:

```
┌──────────────────────────────────────────────────────────────┐
│ TOP BAR: Operator identity · Status pill · SLA counters · Date/Time │
├──────────┬────────────────────────────────┬──────────────────┤
│ LEFT     │ CENTER                          │ RIGHT            │
│          │                                │                  │
│ Queue    │ MAP (incident locations +       │ CASE DETAIL       │
│ of       │ provider units in real-time)    │                  │
│ incoming │                                │ - Caller info    │
│ calls    │ SCREEN-POP                       │ - Vehicle        │
│          │ (caller/policy/vehicle/priority)│ - Policy          │
│ Soft     │                                │ - Coverage        │
│ phone    │ SAFETY TRIAGE                   │                  │
│ controls │ (injury? hazard? PSAP?)         │ DISPATCH          │
│          │                                │ - Ranked providers│
│ Filter   │ INCIDENT FORM                   │ - ETA             │
│ by       │ (what happened, where, notes)   │ - Driver info     │
│ priority │                                │ - One-click dispatch│
│          │ COVERAGE DECISION               │                  │
│ Status   │ (covered? pre-auth? customer    │ MISSION MONITOR   │
│ dashboard│  informed?)                      │ - Live ETA timer │
│          │                                │ - Provider status │
├──────────┴────────────────────────────────┴──────────────────┤
│ BOTTOM: Case timeline · Interaction log · Quick actions     │
└──────────────────────────────────────────────────────────────┘
```

### 2.3 Detailed Function-by-Function Operator Console Requirements

#### TOP BAR
| Element | Purpose | In Insucar? |
|---|---|---|
| Operator name + photo/avatar | Identity confirmation; who handled the call | YES — #a_name, #a_avatar, #a_role |
| Availability status pill | Green=Available, Amber=On-call, Red=Busy. Other operators see who can take calls | Partial — shows "On-call" always, no toggle |
| Live queue counters | Number of calls waiting, longest wait time, avg dispatch time — visible at all times in peripheral vision | YES — #q-wait, #q-longest, #q-active |
| 112/PSAP emergency button | Always-visible red button for immediate emergency services transfer | YES — .btn-112 |
| Shift timer / clock | Accurate time for call logging timestamps | Partial — clock present but may not update reliably |
| Logout button | Secure session termination | YES |

#### LEFT PANEL — Queue & Softphone
| Element | Purpose | In Insucar? |
|---|---|---|
| **Incoming call notification** | Visual/audible alert when call arrives. Shows caller number, auto-lookup result | Partial — manual "Answer" button only, no auto-alert |
| **Softphone controls** | Answer, Hold, Mute, Transfer, Hang-up — integrated in console | MISSING — mock Connect, no real softphone |
| **Call timer** | Duration counter for active call | MISSING |
| **Hold music / reassurance message** | "Please hold, I'm looking up your policy" — audio feedback to caller | MISSING |
| **Queue list** | All active cases with color-coded priority (RED=emergency, AMBER=high, BLUE=normal) | YES — queue table with .qp priority pills |
| **Queue filtering** | Filter by priority, status, incident type, time waiting | MISSING — no filter controls |
| **Queue sorting** | Default: emergency first, then by creation time. Sortable columns | Partial — ORDER BY is hardcoded |
| **Queue auto-refresh** | New cases appear without manual reload | MISSING — manual only |
| **Drag-and-drop assignment** | Drag a case onto a provider or operator to assign | MISSING |

#### CENTER PANEL — Map & Screen-Pop
| Element | Purpose | In Insucar? |
|---|---|---|
| **Interactive map** | Shows incident locations (pinned), provider units (moving real-time), and traffic layer | MISSING — static SVG placeholder |
| **Screen-pop auto-bind** | When call comes in, caller details automatically populate the center panel | Partial — requires clicking a queue row |
| **Caller identity card** | Photo (if available from app), full name, phone, language preference | YES — pop-name, kv details |
| **Policy details** | Policy number, coverage level, status, expiry, covered services | YES — #c_pol, #c_cov |
| **Vehicle details** | Make, model, year, color, plate, fuel type | YES — #c_veh |
| **Incident location** | Address + GPS coordinates + nearby landmarks + what3words | Partial — address only, no GPS/landmarks |
| **Safety triage** | Mandatory checklist: Are you injured? Is the vehicle in a dangerous position? Hazardous materials? Animals inside? — Answers affect priority + routing | MISSING — triage panel exists but not functional |
| **Incident type selector** | Dropdown or button grid: Car won't start, Accident, Flat tyre, Out of fuel, Locked out, Medical, Other | YES — service types in dispatch |
| **Notes/description** | Free-text field for operator observations | YES — symptom_description in cases table |
| **Coverage decision** | Auto-check: Is coverage active? What services are covered? What's the excess? Results displayed clearly | MISSING — only shows coverage text, no decision logic |
| **Service selector** | Grid of services applicable to this incident + coverage — operator clicks to select | Partial — svc-grid exists but not interactive |

#### RIGHT PANEL — Dispatch & Mission
| Element | Purpose | In Insucar? |
|---|---|---|
| **Ranked provider list** | Nearest + highest rated + fastest SLA + available. Auto-sorted, "Recommended" badge on top choice | Partial — 3 static providers, no ranking algorithm |
| **Provider details per row** | Name, distance, rating, ETA, status (available/busy), cost estimate, SLA compliance % | Partial — name + ETA + available only |
| **One-click dispatch button** | Select case + select provider → dispatch. Single action | YES — #dispatchBtn |
| **Provider fallback chain** | If top provider declines, auto-offer to next. Manual escalation if all decline | MISSING |
| **Live mission monitor** | Once dispatched: ETA countdown timer (shrinking), provider vehicle moving on map, driver info with photo/plate/rating | Partial — ETA + driver name shown, no live timer or map |
| **Auto-generated SMS to customer** | "Help is on the way. Pierre L. (TOW-77-FR), ETA 22 min. Track: [link]" — sent automatically on dispatch | YES — SNS SMS with driver + ETA + link |
| **Status link for customer** | Live tracking page showing provider location + ETA updates | Partial — link sent but target page may not exist |
| **Case close/resolve** | Mark case as resolved with resolution notes, customer satisfaction survey trigger | MISSING |

#### BOTTOM PANEL — Timeline & Log
| Element | Purpose | In Insucar? |
|---|---|---|
| **Case timeline** | Chronological log: Call received → Screen-pop → Operator answered → Triage → Coverage check → Dispatch → Provider en route → On scene → Resolved | MISSING — interaction_log exists in DB, not rendered |
| **Call recording playback** | Integrated audio player for call review (GDPR-consented calls) | MISSING |
| **Notes/quick actions** | Pre-set quick messages/actions: "Sending SMS", "Provider contacted", "Calling customer back" | MISSING |
| **Shift handover** | "Hand over case" button → transfers case ownership to next operator with context summary | MISSING |

### 2.4 Supervisor-Specific Features (Not in Insucar at All)
| Feature | Purpose |
|---|---|
| **Barge/Monitor** | Listen in on operator's call silently for training/QA |
| **Whisper** | Talk to operator without caller hearing for coaching |
| **Take-over** | Supervisor takes over the call from operator if escalation needed |
| **Real-time dashboard** | Wallboard view: all operators, their status, active calls, SLA performance |
| **Historical reporting** | Time-to-dispatch, time-to-arrival, resolution rate, NPS, operator performance, provider performance |
| **Call recording QA** | Score operator calls against quality checklist, track improvement |
| **SLA breach alerts** | Configurable alerts when wait time or dispatch time exceeds thresholds |
| **Shift scheduling** | Operator roster, skills-based routing assignments |

### 2.5 Advanced Features (Industry Leaders)
| Feature | Provider | What it does |
|---|---|---|
| **Algorithmic dispatch** | Agero | ML model (100+ TB training data) predicts best provider by location, time, weather, vehicle type, provider history |
| **WinBack recovery** | Agero | If original provider declines, auto-searches for alternative before alerting operator |
| **Alternative transport** | Agero/Lyft | If tow would take too long, auto-books rideshare for customer |
| **Connected vehicle auto-trigger** | OnStar/eCall | Crash sensor → auto-call → auto-GPS → auto-dispatch without human initiation |
| **Embedded credit card processing** | Autura | Customer pays excess/fee via console-generated payment link during call |
| **Geolocation text link** | Autura | One-click "share my location" SMS — customer taps link to share GPS |
| **Drag-and-drop dispatch board** | Towbook | Visual Kanban-style board: drag case card to provider column |
| **Equipment inspection check-in** | Towbook | Provider checks in equipment/truck before shift, verified by photo |
| **Automated customer SMS journey** | Various | Sequence: "Help requested" → "Provider assigned" → "Provider 5 min away" → "Provider arrived" → "Rate your experience" |
| **Multi-motor-club integration** | Towbook | Single console handles calls from 20+ different insurance clubs with different SLAs |

---

## 3. INSUCAR CURRENT STATE — FEATURE COMPLETENESS MATRIX

### 3.1 End-User App
| Feature | Present | Completeness | Notes |
|---|---|---|---|
| Login with email/password | YES | 100% | Demo auth + Cognito SSO |
| Registration form | YES | 100% | All fields present |
| Incident submission | YES | 90% | 6 incident types, description, address. Missing: GPS auto-detect |
| Case listing | YES | 100% | Card layout with status pills |
| Live ETA tracking | NO | 0% | No map or countdown timer in user view |
| Driver identity preview | NO | 0% | Driver shown only in operator console |
| Photo upload | NO | 0% | Not implemented |
| Provider chat | NO | 0% | Not implemented |
| Post-service rating | NO | 0% | Not implemented |
| One-tap emergency | PARTIAL | 50% | Button exists, but must navigate through dash first |

### 3.2 Operator Console
| Feature | Present | Completeness | Notes |
|---|---|---|---|
| Hidden path security | YES | 100% | 404 for wrong paths |
| Agent ID login | YES | 100% | Demo auth + Cognito SSO |
| Agent identity display | YES | 100% | Name, role, avatar |
| SLA counters | YES | 100% | Waiting/Active/Longest/Avg |
| 112/PSAP button | YES | 100% | Present (stub — alerts only) |
| Case queue | YES | 90% | Loads, color-coded. Missing: auto-refresh, filtering, drag-and-drop |
| Case detail (screen-pop) | YES | 85% | Shows all fields. Missing: auto-bind on incoming call |
| Incident type selector | YES | 100% | Via dispatch service selector |
| Coverage decision | NO | 0% | Only shows coverage text, no auto-evaluation |
| Provider ranking | NO | 10% | Static list of 3 providers, no real ranking/ETA calculation |
| One-click dispatch | YES | 100% | Works end-to-end with real SMS |
| Live ETA monitor | PARTIAL | 30% | Shows static ETA number, no countdown, no map |
| Driver identity | YES | 100% | Name + plate + SMS status |
| Interactive map | NO | 0% | Static SVG placeholder only |
| Softphone controls | NO | 0% | Mock Connect, no real softphone |
| Call timer | NO | 0% | Not implemented |
| Safety triage | NO | 10% | Panel exists but not functional |
| Service selector | PARTIAL | 50% | Grid layout exists, not interactive |
| Case timeline | NO | 0% | DB has interaction_log, not rendered |
| Shift handover | NO | 0% | Not implemented |
| Supervisor barge/whisper | NO | 0% | Not in scope yet |
| Queue auto-refresh | NO | 0% | Must manually reload |
| Keyboard shortcuts | NO | 0% | Not implemented |
| Fallback chain | NO | 0% | Only dispatches first provider |

### 3.3 API / Infrastructure
| Feature | Present | Completeness | Notes |
|---|---|---|---|
| Health endpoint | YES | 100% | DB + Redis check |
| REST API for all actions | YES | 100% | Login, register, cases, dispatch, lookup |
| Cognito JWT auth | YES | 100% | RS256 validation against JWKS |
| Real SMS via SNS | YES | 100% | Sent on dispatch |
| Real provider connector | YES | 100% | HTTP connector with fallback |
| Mock telephony (Connect) | YES | 80% | Functional, but not real Connect |
| WebSocket (real-time) | NO | 0% | No push updates |
| Event-driven architecture | PARTIAL | 50% | EventBridge client exists, not wired |
| EKS HA (3 AZ) | YES | 100% | 2 nodes, auto-scaling |
| RDS Multi-AZ | NO (dev) | 0% | Single Postgres pod, not managed RDS |
| HTTPS/TLS | YES | 100% | Let's Encrypt via ingress-nginx |
| CI/CD pipeline | YES | 100% | Jenkins + Spinnaker gated |
| IaC (Terraform) | YES | 100% | Full stack defined, partial apply |

---

## 4. PRIORITIZED IMPROVEMENT REGISTER

Priority: P0=Blocker, P1=Critical, P2=Important, P3=Nice-to-have

### P0 — MUST FIX BEFORE PRODUCTION USE

| # | Improvement | Section | Current gap | Recommended implementation |
|---|---|---|---|---|
| P0-1 | **Landing page → App navigation** | End-User UX | No link/button from `/` to login | Add "Sign In" / "Get Started" button on `landing.html` pointing to `/app` |
| P0-2 | **Queue auto-refresh** | Operator | Queue never updates without page reload | Add `setInterval(() => loadQueue(), 5000)` to operator.html |
| P0-3 | **Screen-pop auto-bind** | Operator | Must click queue row manually after incoming call | Wire incoming() response to auto-select first case and trigger openCase() |
| P0-4 | **Landing page login path** | End-User UX | See QAobservations.md Issue 1 | Same as P0-1 |

### P1 — CRITICAL FUNCTIONALITY

| # | Improvement | Section | Current gap | Recommended implementation |
|---|---|---|---|---|
| P1-1 | **Service catalog parity with Redion** | End-User | Missing 8 service types competitors offer | Extend `service_line` enum: `vehicle_repatriation`, `tyre_protection`, `car_swap_ev`, `pickup_delivery`, `micromobility`, `service_activated_rsa`, `journey_continuation`, `car_pickup_delivery` |
| P1-2 | **Provider ranking algorithm** | Operator | Static provider list, no real ranking | Query providers by distance (PostGIS), availability, performance_score, priority_rank — sort by composite score |
| P1-3 | **Provider fallback chain** | Operator | Only dispatches first provider regardless | When dispatch returns "no provider available", auto-try next in ranked list. Manual escalation button |
| P1-4 | **Live ETA countdown timer** | Both | Static ETA number | JS interval updating ETA every 60s. Backend endpoint `/api/mission/eta?id=` returning updated ETA |
| P1-5 | **Interactive map** | Operator | Static SVG placeholder | Embed Leaflet/OpenStreetMap tile with incident pin + provider unit markers. Minimal: `<iframe>` with OpenStreetMap search |
| P1-6 | **Coverage decision engine** | Operator | Shows coverage text only | When case selected, auto-check policy coverage against selected service. Display green ✅ / red ❌ / amber ⚠ with excess |
| P1-7 | **Safety triage workflow** | Operator | Panel exists, non-functional | Wire triage yes/no buttons to set case.priority (any "yes" = emergency). Auto-enable PSAP button on hazardous answers |
| P1-8 | **End-user GPS auto-detect** | End-User | Must manually type address | `navigator.geolocation.getCurrentPosition()` on incident form. Reverse-geocode to address |

### P2 — IMPORTANT FUNCTIONALITY

| # | Improvement | Section | Current gap | Recommended implementation |
|---|---|---|---|---|
| P2-1 | **Case timeline in console** | Operator | interaction_log exists in DB, not shown | Render timeline panel from `/api/agent/case?id=` with `interaction_log` events sorted by created_at |
| P2-2 | **End-user live tracking page** | End-User | Status link target may not exist | Serve `/status/:token` page with ETA + provider location + driver info. Auto-refresh |
| P2-3 | **SMS journey automation** | Both | Single SMS on dispatch only | Add SMS events: "provider 5 min away", "provider arrived", "case resolved". Use EventBridge + SNS |
| P2-4 | **Queue filtering** | Operator | View all cases, no filter | Add filter buttons/dropdown: All, Emergency, High, Normal, Mine. Filter client-side on JSON response |
| P2-5 | **Call timer** | Operator | No call duration shown | JS timer starting when "Answer" clicked, displayed in softphone panel |
| P2-6 | **Post-service rating** | End-User | No feedback mechanism | Add star rating UI in case card after status=resolved. POST `/api/case/rate` |
| P2-7 | **Photo upload** | End-User | Can't share visual of scene | `<input type="file" accept="image/*" capture="environment">` on incident form. Upload to S3 via presigned URL |
| P2-8 | **Operator keyboard shortcuts** | Operator | Mouse-only interaction | Bind: `E`=emergency, `D`=dispatch, `N`=next case, `A`=answer, `H`=hold, `ESC`=close |
| P2-9 | **Automated SMS "share location"** | End-User | No mobile location sharing | SMS with `<a href="geo:lat,lng">` link that opens phone's maps. Or generate a tracking page URL |
| P2-10 | **Service selector interactivity** | Operator | Grid visible, not clickable | Wire svc-grid buttons to set selected service. Auto-fill dispatch payload |

### P3 — NICE TO HAVE

| # | Improvement | Section | Current gap | Recommended implementation |
|---|---|---|---|---|
| P3-1 | **Shift handover** | Operator | No case transfer mechanism | "Transfer Case" button → assign to another operator. Update `cases.assigned_to` |
| P3-2 | **Supervisor dashboard** | Operator | No overview of all operators | Wallboard-style view: operator list with status, active calls, performance metrics |
| P3-3 | **Drag-and-drop dispatch board** | Operator | Click-driven dispatch | Kanban board: columns = statuses, cards = cases. Drag card to "Dispatched" column |
| P3-4 | **B2B white-label portal** | Partner | Not built | Separate React SPA with tenant branding config, KPI dashboard, provider preferences |
| P3-5 | **Call recording playback** | Operator | Recordings exist in DB schema | AWS Connect recording → S3 → presigned URL → embedded audio player in timeline |
| P3-6 | **Multi-language UI switching** | Both | EN only in UI | i18n via JSON lang files. Language toggle in top bar. Detect from Cognito user pool setting |
| P3-7 | **Offline/resilience mode** | Operator | Requires full connectivity | Service worker caching console HTML. Queue state in localStorage. Sync when online |
| P3-8 | **Advanced analytics dashboard** | Admin | No BI/reporting | Grafana dashboards for time-to-dispatch, time-to-arrival, provider performance, NPS, call volume by hour/day |

---

## 5. COMPETITIVE POSITIONING

### Where Insucar Leads
| Strength | Why it matters |
|---|---|
| **Modern event-driven architecture** | Go + Rust on EKS with IaC — no legacy mainframe or .NET monolith dragging iteration speed |
| **Security posture** | Hidden operator surface, SHA-256 hash-chained audit ledger, 3-tier IAM with JIT break-glass — competitors don't have this |
| **Transparency features** | Tokenized status links, driver identity shared with stranded customer, real-time SMS — builds trust competitors lack |
| **CI/CD maturity** | Jenkins → Kaniko → ECR → Spinnaker gated pipeline — continuous delivery from day one |
| **AWS-native** | No on-premise dependency, fully managed services, Multi-AZ design |

### Where Insucar Lags
| Weakness | Competitor advantage |
|---|---|
| **No map** | Every competitor has interactive maps in both customer and operator views |
| **No real-time push** | Agero/Autura have WebSocket-driven live updates. Insucar is request/response only |
| **No provider network scale** | Redion has 12,000 providers. Insucar needs frictionless multi-provider onboarding |
| **No ML/algorithmic dispatch** | Agero uses 100+ TB data for ML matching. Insucar uses DB ORDER BY only |
| **No omnichannel** | Competitors handle phone + app + web + chat + eCall. Insucar is phone + single web app |
| **Single-region** | Production needs multi-region warm standby (RDS cross-region replicas, Route53 failover) |

---

## 6. RECOMMENDED NEXT ACTIONS (ordered by impact/effort)

| Order | Action | Impact | Effort | Section |
|---|---|---|---|---|
| 1 | Add "Sign In" button on landing page | HIGH | TRIVIAL | P0-1 |
| 2 | Queue auto-refresh (setInterval 5s) | HIGH | TRIVIAL | P0-2 |
| 3 | Screen-pop auto-bind on incoming call | HIGH | SMALL | P0-3 |
| 4 | Coverage decision engine | HIGH | MEDIUM | P1-6 |
| 5 | Provider ranking + fallback chain | HIGH | MEDIUM | P1-2, P1-3 |
| 6 | Live ETA countdown + end-user tracking page | HIGH | MEDIUM | P1-4, P2-2 |
| 7 | Leaflet/OSM interactive map | HIGH | MEDIUM | P1-5 |
| 8 | Safety triage workflow wiring | MEDIUM | SMALL | P1-7 |
| 9 | Case timeline rendering | MEDIUM | SMALL | P2-1 |
| 10 | End-user GPS + photo upload | MEDIUM | MEDIUM | P1-8, P2-7 |
| 11 | Post-service rating + SMS journey | MEDIUM | MEDIUM | P2-3, P2-6 |
| 12 | Keyboard shortcuts + queue filtering | LOW | SMALL | P2-4, P2-8 |
| 13 | Service catalog expansion (8 new types) | LOW | MEDIUM | P1-1 |
| 14 | Multi-language UI | LOW | LARGE | P3-6 |

---

## §7: EXPERT ROLE-PLAY FINDINGS (2026-07-05)

Added from `expertobservations.md` — 3-perspective empathy walkthrough.

### End-User Scores (Claire Martin, stranded on A6)

| Step | What Happens | Score | Emotional State |
|---|---|---|---|
| 1. Land on site | Beautiful marketing page, "Get help now" button exists but competes with "Log in" | 6/10 | Confused — "which button do I press?" |
| 2. Login | Works for known users. No guest/passenger path. | 7/10 | Frustrated — "I need help, not a login form" |
| 3. Request help | 6-step form: dropdown + textarea + address + button + alert. Too many actions in distress. | 4/10 | Anxious — "what if I type wrong?" |
| 4. Wait | Static card showing "triaging". No ETA, no driver, no map, no updates. | 2/10 | Escalating anxiety — "is anyone actually coming?" |
| 5. Provider arrives | Nothing. No confirmation. No rating. | 0/10 | Abandoned — "did it even work?" |
| **Overall** | | **3.5/10** | Trust loop broken after submission |

### Operator Scores (Amelie Durand, dispatch)

| Step | What Happens | Score |
|---|---|---|
| 1. Login | Dark console, identity display, SLA counters — professional | 8/10 |
| 2. Incoming call | Screen-pop auto-bind works, coverage shown, providers listed | 7/10 |
| 3. Triage | 5 yes/no buttons visible but DON'T save to DB or change priority | 4/10 |
| 4. Coverage | Shows coverage text but doesn't enforce entitlement checks vs selected service | 6/10 |
| 5. Dispatch | Works end-to-end. Driver info now persists after fix | 7/10 |
| 6. Monitor | Dispatched cases vanish from view. No live mission monitoring | 3/10 |
| **Overall** | | **6/10** |

### 7 Immediate P0 Fixes From Expert Analysis

| # | Issue | Fix Summary | Effort |
|---|---|---|---|
| X1 | Post-submission silent | Tracking view with ETA + driver + map replacing static card | 3h |
| X2 | GPS not auto-detected | Call getGPSLocation() in enter() on dashboard load | 15m |
| X3 | alert() blocks mobile | Inline green confirmation banner | 20m |
| X4 | No login bypass | "Can't log in? Call us" phone link above login form | 10m |
| X5 | No mission monitoring | "Active Missions" panel for dispatched cases | 4h |
| X6 | Triage not functional | Wire buttons to DB + auto-escalate priority | 3h |
| X7 | Duplicate cases | Check existing open case for caller's ANI before creating new | 2h |

### Architecture Weaknesses Identified

| # | Weakness | Severity |
|---|---|---|
| 1 | No WebSocket/real-time layer — polling only | HIGH |
| 2 | Single Go monolith — splitting into services becomes harder over time | MEDIUM |
| 3 | No API versioning — breaking response changes affect all clients | MEDIUM |
| 4 | No rate limiting on API endpoints | HIGH |
| 5 | No backend input validation — accepts any JSON shape | MEDIUM |
| 6 | In-cluster Postgres (not managed RDS) — no automated backups/PITR | HIGH |
| 7 | Redis not configured — `redis=false` in logs | MEDIUM |
| 8 | Mock telephony only — Amazon Connect not provisioned | HIGH |
| 9 | Operator URL committed in source code — not truly hidden | LOW |

### Overall Readiness Score: 5.5/10
"It works" ≠ "It helps." Platform functions technically but the emotional experience — the trust loop between requesting help and receiving it — is broken after the submission step.

---

*Document maintained by QA Engineering. Last updated: 2026-07-05 with expert role-play findings.*
