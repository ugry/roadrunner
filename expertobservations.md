# Insucar — Expert Observations: Real-User Journey Analysis

**Methodology:** Role-play walkthrough from three perspectives — stranded motorist, dispatch operator, and expert reviewer. Each step documents what actually happens vs what should happen.

---

## PART 1: END-USER JOURNEY — "My car broke down on the A6"

### I am Claire Martin. It's Saturday, 8 PM. My Renault Clio won't start on the A6 southbound near Beaune. I open my phone.

---

### Step 1: I go to unysolar.com

**What I see:** A beautiful marketing landing page. Golden-ratio layout. "24/7 global roadside assistance." "Stranded? Help is one tap away." Hero section with phone mockup showing a tow truck and ETA.

**What I need:** HELP. Now. I'm stressed. I need a single obvious button that says "GET HELP" or "I NEED ASSISTANCE."

**What actually exists:** There IS a "Get help now" button in the header and hero section. These both link to `/app`. The hero also has a "Log in" button.

**GAP:** The buttons are styled identically — a stranded user scanning for "EMERGENCY" won't immediately recognize the green "Get help now" as the right path. The "Log in" button is the same size and color as the help button. In an emergency, visual hierarchy matters: the help button should be LARGER and RED/AMBER, while "Log in" should be secondary.

**GAP:** If I'm on mobile (most likely for a roadside breakdown), the desktop "Log in" button is hidden. The hero "Get help now" button is visible but requires scrolling past the hero copy. On a small screen, this button may be below the fold.

**SCORE: 6/10** — Buttons exist but lack emergency visual hierarchy. Mobile lacks visible login.

---

### Step 2: I tap "Get help now" → /app

**What I see:** A login form. Email field pre-filled with "claire.martin@example.fr". Password field. "Log in" button. Register tab. And — if Cognito is configured — a "Sign in with Cognito" button.

**What I need:** I want to type my password and get help. I do NOT want to register as a new user. I do NOT want to read about Cognito.

**What actually works:** I type "Claire#2026" and click "Log in". It works. I'm now on the dashboard.

**GAP:** The login form assumes I already know my account. If I'm a first-time user who just bought the insurance policy, I DON'T have an account. There's no "Continue as guest" or "Request help without account" option. A stranded person may be a passenger, a rental car driver, or someone who never registered.

**GAP:** If Cognito is NOT configured (which it wasn't until our sprint), the "Sign in with Cognito" button is hidden — that's fine. But when it IS configured, both buttons appear side by side. A distressed user sees TWO login options and has to decide which to use. Decision paralysis in an emergency.

**SCORE: 7/10** — Login works for known users. No guest/passenger path.

---

### Step 3: I'm on the dashboard. I need to request help.

**What I see:** Two cards side by side on desktop, stacked on mobile:
- Left: "Request assistance" — dropdown (What happened?), textarea (Describe it), input (Location), submit button
- Right: "My cases" — list of previous cases

**What I need:** I want to press ONE button and have help come. I don't want to:
- Think about what incident TYPE to select from a dropdown
- Type a description of what's happening
- Manually type my location address

**What I actually have to do:**
1. Focus my eyes on the dropdown — select "Car won't start"
2. Move to the textarea — type "won't start on A6 km 290"
3. Move to the Location input — type "A6 southbound km 290"
4. Find and click the red "Request assistance now" button
5. Dismiss a browser `alert()` dialog

**GAP — COGNITIVE LOAD:** This is 5 distinct interactive actions from a person in distress. NN/Group research shows that cognitive load in high-stress situations TRIPLES error rates. I might select the wrong incident type. I might type my location wrong. I might miss the submit button entirely.

**GAP — LOCATION:** I have to TYPE my location. On a phone keyboard. In the dark. On the side of a highway. The GPS auto-detect button ("📍 Use my location") exists now (added in P1-8) but it's a small text button INSIDE a label. I might not even notice it exists. A stranded user needs location to be AUTO-DETECTED on page load, not manually activated.

**GAP — ALERT DIALOG:** After submitting, the app shows a browser `alert()` dialog: "Case created: CASE-1783234933 — a coordinator will assist you." This is a blocking alert. On mobile, it covers the screen. If my phone screen locks while the alert is showing, I might not see it. Alerts should be in-app notifications or inline confirmation, not browser dialogs.

**GAP — FEEDBACK:** After submitting, my new case appears in "My Cases" as a card showing "Case #XXX — triaging". That's it. There's no ETA. No "help is coming" reassurance. No map. No driver info. I'm left staring at a status pill that says "triaging" with no idea what happens next or when.

**SCORE: 4/10** — Too many steps, too much typing, blocking alert, no post-submission feedback.

---

### Step 4: I'm waiting. What happens?

**What I see:** The "My Cases" card shows my case with status "triaging". If I refresh the page (manually — no auto-refresh), it might change to "dispatched".

**What I need:** I need to know:
- Is someone coming? (YES/NO)
- WHO is coming? (driver name, photo, vehicle)
- WHEN will they arrive? (live ETA countdown)
- WHERE are they now? (map with moving provider pin)
- Can I call them? (one-tap call button)

**What actually happens:** ABSOLUTELY NOTHING VISIBLE. The case status might change in the DB (operator dispatched) but the page doesn't auto-refresh. There's no push notification. There's no ETA countdown. There's no map. There's no driver info. The case card shows the SAME information as when it was created.

**CRITICAL GAP:** The end-user has ZERO visibility into what happens after they submit a help request. They get one SMS with driver/ETA (if operator dispatches — which requires the operator to be logged in and click buttons). But in the app itself? Nothing updates. This is the biggest experience gap in the entire platform.

**If the SMS arrives** (from operator dispatch), it says: "Insucar: help is on the way. AXA Roadside FR, driver Pierre L. (TOW-77-FR), ETA ~22 min. Track: [link]"

The tracking link exists (P2-2 was implemented) — this is GOOD. But it requires:
1. The operator to dispatch
2. The customer to receive and OPEN the SMS
3. The customer to click the link

There's no in-app tracking. If the SMS is delayed or missed, the user is completely in the dark.

**SCORE: 2/10** — Post-submission experience is effectively blank. SMS is the only feedback channel.

---

### Step 5: The provider arrives. Now what?

**Current state:** Nothing. The case sits at "dispatched" or disappears from the queue. No confirmation. No rating. No closure.

**GAP:** No "Did the right person show up?" confirmation. No post-service rating. No feedback loop.

**SCORE: 0/10** — Post-service experience doesn't exist.

---

## PART 2: OPERATOR JOURNEY — "A call lands on my console"

### I am Amelie Durand, operator OP-1001. I'm on shift at the Insucar dispatch center.

---

### Step 1: I log in

**What I see:** A dark-themed console. URL: `https://op.unysolar.com/ops-console-7f3a9c`. Login form with Agent ID (OP-1001) and password fields, plus Cognito SSO button.

**What I need:** A professional, fast-loading console that shows me everything I need at a glance.

**What works:** Login is fast. After login, I see:
- My name (Amelie Durand), role (operator), avatar (AD)
- SLA counters: Waiting, Active, Longest wait
- 112/PSAP emergency button
- Phone input for mock Connect
- Case queue table
- Provider list
- Service selector
- Dispatch button

**PRO:** The dark theme is good for 24/7 ops. The layout is clean. Color-coded priority pills (emergency/high/normal) are instantly recognizable.

**GAP:** The console URL is discoverable only by knowing the exact path `/ops-console-7f3a9c`. Security by obscurity is not security. This path IS committed in the source code (operator.html). Any developer who reads this code knows the operator URL. Real security requires Cognito authentication (which exists but needs MFA enforced).

**GAP:** When I log in, the queue loads. But if a case was created while I was logging in (those 2 seconds), I won't see it until the 8-second auto-refresh fires. First-time queue load should be immediate; subsequent refreshes can be 8s.

**SCORE: 8/10** — Professional look, works well. URL shouldn't be a "secret" committed in source.

---

### Step 2: A call comes in. Claire Martin needs help.

**How I know:** I see the phone input field pre-filled with "+33600000001". I click "Answer incoming call." The screen-pop header shows: "📞 Claire Martin · INS-FR-1001 · Renault Clio"

**What I see after answering:**
The screen-pop (P0-3 auto-bind) auto-opens the first case in the queue. I see:
- Customer: Claire Martin
- Phone: +33600000001
- Policy: INS-FR-1001 (premium, active)
- Vehicle: Renault Clio AB-123-CD
- Incident: breakdown
- Priority: HIGH
- SLA age bar showing case age
- Coverage Decision panel: premium coverage, excess, callout limit
- Safety Triage panel: 5 yes/no questions
- Service selector: 6 options
- Provider list: ranked by score
- Map: Leaflet with incident pin
- Dispatch button (enabled)

**PRO:** The auto-bind (P0-3) works — the case opens automatically after answering. This is excellent UX.

**PRO:** The coverage decision panel shows what's covered and what's not. The safety triage questions help assess hazard level.

**PRO:** The provider list shows ranked providers with scores and availability.

**GAP — SOFT PHONE:** There's no softphone. The "Answer incoming call" button is mock. In a real production system, Amazon Connect Streams would embed a softphone directly in this panel. The mock button is fine for development but the transition to real Connect must be seamless.

**GAP — CALL TIMER:** When I answer, there's no call duration timer. In a real dispatch center, operators need to track call duration for SLA and shift metrics.

**GAP — CALLER VERIFICATION:** The screen-pop auto-binds assuming the phone number lookup works. If the ANI is unknown (no customer in DB), I get "Unknown caller — create case manually." But there's no guided form to quickly create a case for an unknown caller. I have to fill in every field manually.

**GAP — DUPLICATE DETECTION:** If Claire calls AGAIN (same breakdown, already has a case), the system creates a NEW case instead of linking to the existing one. There's no duplicate detection.

**SCORE: 7/10** — Screen-pop is good. Auto-bind works. Missing softphone, call timer, duplicate detection.

---

### Step 3: I triage the situation

**What I see:** The Safety Triage panel with 5 questions:
- Everyone safe? YES/NO
- In live traffic? YES/NO
- On hard shoulder? YES/NO
- Vulnerable occupants? YES/NO
- Night/darkness? YES/NO

**What I need:** Quick answers to determine priority. If ANY "dangerous" answer, the case should auto-escalate.

**GAP:** The triage buttons are YES/NO toggles but they don't DO anything. They change visual state but don't:
- Auto-set case priority to "emergency" if "NO" on "Everyone safe"
- Auto-enable 112/PSAP button if "YES" on "In live traffic"
- Auto-warn about children/pets if "YES" on "Vulnerable occupants"
- Save responses to the database

**GAP:** The triage is manual-only. In a real system, Amazon Lex would ASK these questions during the IVR ("Are you in a safe location? Press 1 for yes, 2 for no") and pre-fill the triage form before the operator even answers.

**SCORE: 4/10** — Visual exists, but not wired to priority logic or database. No IVR pre-fill.

---

### Step 4: I decide coverage

**What I see:** Coverage Decision panel shows:
- Coverage level: premium
- Excess/franchise: shown if available
- Callouts remaining: shown if available
- Towing: ✓ Covered
- Roadside repair: ✓ Covered

**What I need:** Quick confirmation that this service IS covered. If not covered, I need to inform the customer and offer payment options.

**GAP:** The coverage check is hardcoded for premium/comprehensive plans. It doesn't check the SPECIFIC service selected vs the policy's entitlements. A premium plan might cover towing but NOT repatriation.

**GAP:** If service is NOT covered (e.g., basic plan requesting vehicle repatriation), there's no payment flow. No "Customer will pay €75 excess — confirm?" prompt. No payment link generation. No PCI-scoped payment capture.

**SCORE: 6/10** — Shows coverage but doesn't enforce entitlement checks or payment flows.

---

### Step 5: I dispatch

**What I see:** I click "Dispatch to selected" (or press A/D shortcut). The mission box shows provider "AXA Roadside FR", ETA "22 min". Driver info card appears: "Pierre L." (TOW-77-FR). SMS sent confirmation.

**What works:** Dispatch flow is fast. The driver info loaded correctly and persisted after queue refresh (BUG-1 fixed). SMS is sent with driver identity and tracking link.

**PRO:** Real provider connector works (when PROVIDER_API_URL is set). Mock works as fallback.

**GAP:** The dispatch button dispatches to the first provider in the list regardless of ranking. P1-3 (provider fallback chain) was implemented but only triggers if the API returns an error. If the API returns SUCCESS but the provider never actually arrives, there's no detection.

**GAP:** There's no "estimated cost" or "customer will be charged" warning before dispatch. Operators need to know if this will cost the customer money.

**GAP:** After dispatch, the mission ETA is a static number. It doesn't count down. There's no "ETA updated to 18 min" when the provider gets closer. The ETA only changes if the operator manually re-opens the case and the backend returns a new value.

**SCORE: 7/10** — Dispatch works end-to-end. Missing dynamic ETA, cost warning, provider no-show detection.

---

### Step 6: I monitor the mission

**What I see:** After dispatch, the case leaves my main queue view (it's now "dispatched" status). If I want to check on it, I need to find it in the queue or search.

**What I need:** A dedicated "Active Missions" view showing all dispatched cases with:
- Live ETA timers counting down
- Provider location on the map
- Customer status (waiting, picked up, arrived)
- Last contact timestamp

**GAP:** There's NO active mission monitoring view. Dispatched cases vanish from the operator's primary attention. In a professional CAD system, dispatched cases move to a "Monitoring" panel with live updates.

**GAP:** If the provider is late (ETA exceeded), there's no alert. If the customer calls back asking "where is my help?", the operator has to search for the case manually.

**SCORE: 3/10** — No mission monitoring. Dispatched cases effectively disappear.

---

## PART 3: EXPERT CROSS-CUTTING OBSERVATIONS

### 3.1 Architecture Strengths

| Strength | Detail |
|---|---|
| **Event-driven foundation** | Go + PostGIS + EventBridge ready. Once WebSocket is added (P0-10), the system naturally supports push updates |
| **Immutable audit ledger** | SHA-256 hash-chained — every action is cryptographically verifiable. Competitors don't have this |
| **Cognito SSO with PKCE** | Modern OAuth2 flow — no passwords stored in app, JWT verification via JWKS, group-based RBAC |
| **CI/CD maturity** | Jenkins + Kaniko + Spinnaker gated pipeline. Every commit can go to production |
| **IaC completeness** | Full Terraform for VPC/EKS/RDS/Cognito/ElastiCache. No manual AWS Console work |
| **Security by design** | Hidden operator surface, 3-tier IAM, JIT break-glass — above industry standard |
| **Single Go binary** | No microservice overhead during prototype phase — fast iteration |

### 3.2 Architecture Weaknesses

| Weakness | Impact |
|---|---|
| **No WebSocket/real-time layer** | Polling-based updates. 8-second queue lag. No push to motorist |
| **Single Go monolith** | All services in one binary. Splitting into case/dispatch/provider/notification services becomes harder the longer it's deferred |
| **No API versioning** | Breaking changes to `/api/agent/case` response format affect both operator.html and any future mobile app |
| **Flat data model** | `customer <-> policy <-> vehicle` relationships exist but aren't fully normalized |
| **No rate limiting** | API endpoints have no rate limits — vulnerable to abuse |
| **No backend input validation** | `json.NewDecoder(r.Body).Decode(&in)` without schema validation — accepts any JSON shape |
| **In-cluster Postgres** | Not managed RDS. No automated backups, PITR, or Multi-AZ failover |
| **Redis dependency not configured** | `redis=false` in startup logs — ElastiCache never wired |
| **Mock telephony** | Amazon Connect not provisioned. The entire IVR/screen-pop flow is simulated |

### 3.3 UX Strengths

| Strength | Detail |
|---|---|
| **Operator dark theme** | Proper dark mode from day one — better than most startups |
| **Responsive case cards** | End-user "My Cases" with card layout — works on mobile |
| **SLA monitoring** | SLA age bar with breach detection in operator console |
| **Provider ranking** | Database-driven provider list with scores and availability windows |
| **Coverage decision panel** | Shows what's covered vs not at dispatch time |
| **Auto-refreshing queue** | 8-second interval with change detection |
| **Color-coded priorities** | Red/amber/blue pills in queue |
| **Interactive map** | Leaflet.js with incident + provider markers (P1-5) |

### 3.4 UX Weaknesses (Critical)

| Weakness | User Impact | Priority |
|---|---|---|
| **No post-submission feedback for motorist** | User submits case → sees "triaging" → NOTHING ELSE happens in the app. No ETA, no map, no driver info. They stare at a static card. | P0 |
| **Login wall before help** | Stranded motorist cannot request help without logging in. No guest path. No "call us" fallback button. | P0 |
| **Browser alert() for confirmation** | Blocks mobile screen. Can't be styled. Doesn't work if phone locked. | P0 |
| **6-step help request flow** | Dropdown selection + type description + type address + click button + dismiss alert = too many steps in distress | P0 |
| **No mission monitoring view for operators** | Dispatched cases vanish from active view. No live ETA countdown. | P0 |
| **No duplicate call detection** | Same caller creates new case instead of linking to existing | P1 |
| **Triage buttons don't save or change priority** | Visual state only — no DB write, no logic | P1 |
| **No estimated cost display** | Operator can't tell customer what this will cost | P1 |
| **Source code contains operator URL** | Security-by-obscurity doesn't work when the secret is committed | P1 |

### 3.5 The Biggest Gap: The Emotional Journey Gap

Let me trace Claire's emotional state at each step of the current flow:

| Step | What Claire Feels | What App Shows | Emotional Mismatch |
|---|---|---|---|
| Car breaks down | Panic, fear, cold | Beautiful marketing page with golden-ratio design | App looks calm. Claire is not calm. |
| Finds "Get help now" | Relief — finally! | Login form with email/password | Relief turns to frustration. "I need HELP not a login form!" |
| Logs in | OK, progressing | Dashboard with two cards | "Where's the help button? Why do I need to fill a form?" |
| Fills form | Anxiety — "what if I type wrong?" | Dropdown, textarea, address input | Cognitive load in high-stress state |
| Submits | Temporary relief | Alert dialog "Case created" | "OK... now what? When? Who?" |
| Waits... waits... | Escalating anxiety | Static card "Case #XXX — triaging" | "Did it work? Is anyone coming? How long?" |
| Gets SMS | Relief again | Phone notification | "Finally! ... but why wasn't this in the app?" |

**The core problem:** The app breaks the trust loop. Claire submits her request and the app goes silent. She has no visibility into what happens next. The app should be a REASSURING COMPANION during the wait, not a form processor that goes mute after submission.

---

## 4. RECOMMENDED IMMEDIATE FIXES (P0 — Must fix before any real user)

| # | Fix | File | Lines to change |
|---|---|---|---|
| 1 | **Post-submission tracking view** — After incident, replace the form with a tracking page showing "Help is coming. ETA: ~22 min. Pierre L., TOW-77-FR. [Track on map]" | enduser.html | Dashboard panel replacement (~30 lines) |
| 2 | **Auto-detect GPS on dashboard load** — Don't wait for user to click "Use my location". Auto-detect on page load and pre-fill. | enduser.html | Call `getGPSLocation()` in `enter()` |
| 3 | **Replace alert() with inline confirmation** — Remove `alert()` and show a green banner: "✅ Help requested! Case #XXX. A coordinator is finding help now." | enduser.html | submitIncident() handler |
| 4 | **Add "Call us" button on login page** — "Can't log in? Call +33800000000" visible ABOVE the login form | enduser.html | Auth card HTML |
| 5 | **Post-dispatch mission monitoring panel** — New panel in operator console showing all dispatched cases with live ETA timers | operator.html | New panel after dispatch panel |
| 6 | **Wire triage buttons to DB + priority logic** — On any "NO" to safety → priority=emergency. On any dangerous answer → enable PSAP. Save all answers. | operator.html | triage button onclick handlers + backend endpoint |
| 7 | **Duplicate call detection** — Before creating new case, check if customer has existing OPEN case. If yes, link incoming call to existing. | main.go | handleMockIncoming handler |

---

## 5. FINAL SCORECARD

| Perspective | Score | Key Issue |
|---|---|---|
| **End-user (stranded motorist)** | 3.5/10 | Post-submission experience is silent. No trust loop. |
| **Operator (dispatch agent)** | 6/10 | Works for manual dispatch. Missing monitoring, triage wiring, softphone. |
| **Architecture** | 7/10 | Strong foundation. Needs WebSocket, API versioning, input validation, managed DB. |
| **Overall readiness** | 5.5/10 | Functions technically, fails emotionally. The gap between "it works" and "it helps" is large. |
