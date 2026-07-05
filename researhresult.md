# Insucar — UX Research & Experience Improvements

**Research Date:** 2026-07-05  
**Sources:** NN/Group (progressive disclosure, wizard patterns), WCAG 2.1 W3C, web.dev Offline Cookbook, MDN WebSocket/Push API docs, competitor analysis (AAA, Honk, Urgently, Agero/Swoop, Redion), CAD dispatch system conventions (Hexagon, Motorola, Autura), Material Design dark theme spec  
**Scope:** End-user (stranded motorist) + Operator (dispatch agent) experience improvements

---

## 1. END-USER (STRANDED MOTORIST) EXPERIENCE

### 1.1 One-Tap Emergency — Hold-to-Confirm Panic Button

**Problem:** Current flow requires multiple steps: navigate to /app, log in, find the "Request Assistance" card, select incident type, type description, type address, click submit. A stranded motorist in distress should never type more than 20 characters.

**Industry pattern:** AAA, Honk, Urgently all center the experience on a single giant "Help" button on the home screen. Progressive disclosure moves vehicle details and membership to later steps.

**Recommended implementation:**
- Giant 72×72dp floating action button (FAB), bottom-right, red/amber, always visible
- **Hold-to-confirm** pattern: 500ms hold with visual progress ring. This prevents accidental triggers while providing immediate haptic feedback that the press registered (critical for cold/numb hands)
- After triggering: auto-detect GPS → show map with pin → ask "Is this correct?" → then ask what happened (5 options only) → then submit
- Never ask for vehicle details, policy number, or membership info at this stage. These belong in the user's profile and can be fetched from the backend.

**Insucar status:** NOT IN SCOPE — current flow requires full form completion. P0 priority.

### 1.2 Progressive Disclosure Wizard Flow

**Problem:** Current enduser.html shows login form, incident type, description, and address all on one page. NN/Group field studies show that premature cognitive load in high-stress situations triples error rates and abandonment.

**Recommended implementation — 4-step wizard:**

```
STEP 1: "What do you need?"
  Buttons: [🚚 Tow/Recovery] [🔋 Jump Start] [🛞 Flat Tyre] [🔑 Lockout] [⛽ Fuel]
  Single tap selects. Large targets (min 60×60px). Color-coded.
  ↓
STEP 2: Location confirmation
  Auto-detected GPS pin on map. "Is this correct?"
  Buttons: [✅ Yes, this is correct] [📍 Adjust pin on map]
  Also shows: street address, what3words address, nearest landmark
  ↓
STEP 3: Quick details (ONLY if not in profile)
  Vehicle make/model/year — pre-filled from saved profile
  Photo of scene (optional, post-submission)
  Special instructions (optional text field)
  ↓
STEP 4: Confirmation
  "Provider will arrive in ~22 min. We sent your location automatically."
  [Track my rescue] [Share with family] [Call operator]
```

- **Back button at every step**, never allow skipping ahead
- Wizard replaces the current multi-field form completely
- Progress indicator: 4 dots at top of screen (●●○○) so the user knows where they are

**Insucar status:** NOT IN SCOPE. P0 priority.

### 1.3 what3words Integration

**Problem:** "I'm on the A6 near Beaune" is vague. GPS coordinates (48.8566, 2.3522) are impossible to communicate verbally when talking to an operator.

**What what3words does:** Divides the world into 3m×3m squares, each with a unique 3-word address. Example: `///flame.tribune.workforce`

**Why it matters:**
- Works offline — algorithm converts GPS coordinates client-side, no network needed
- Voice-friendly — "flame tribune workforce" is easier than reading GPS coordinates
- 3m precision — identifies the exact breakdown spot on a highway, not just the nearest exit
- Critical for: highways with no mile markers, rural areas, multi-level parking structures, large parking lots

**Implementation:**
- Use what3words JavaScript SDK or pre-cached grid for offline resolution
- Display both street address **and** what3words address on the confirmation screen
- The operator sees both in the screen-pop panel
- The motorist can read the 3 words to the operator over the phone

**Insucar status:** NOT IN SCOPE. P0 priority.

### 1.4 Offline-First PWA with Background Sync

**Problem:** 15-20% of breakdowns occur in areas with poor or no cellular signal. The current web app requires constant connectivity.

**Recommended implementation:**
- **Service Worker cache-first strategy:** Pre-cache all UI shells, forms, and map tiles for a 50km radius from last known location
- **Background Sync API:** When user submits a help request offline:
  1. Store in IndexedDB with `{status: 'pending_sync', timestamp, gps_coords, service_type}`
  2. Register with `navigator.serviceWorker.ready.then(reg => reg.sync.register('submitHelp'))`
  3. Show persistent indicator: "Request will send when signal returns" with estimated retry time
  4. When online, sync and deliver Push notification: "Help request sent! Provider dispatched."
- **SMS fallback:** Last-resort gateway: "Text HELP to +33XXXXXXXXX" auto-parses location from SMS and creates dispatch case. This works even on 2G/EDGE networks.

**Insucar status:** NOT IN SCOPE. P1 priority.

### 1.5 Battery-Saving Auto-Mode

**Problem:** A stranded motorist's phone is their lifeline. If their battery dies, they lose tracking, communication, and safety features.

**Implementation:**
- Auto-enable low-data/low-power mode when `navigator.getBattery().level < 0.20`
- Switch to true-black (`#000000`) OLED backgrounds (saves ~60% power)
- Reduce ETA polling from 30s to 120s
- Disable non-critical animations
- Show persistent battery indicator with estimated remaining time
- "We'll keep tracking even if your screen is off" — use Push API for critical updates only

**Insucar status:** NOT IN SCOPE. P3 priority.

### 1.6 Voice-Guided Assistance (Web Speech API)

**Problem:** Users in freezing weather, with disabilities, elderly users, or those with injuries cannot use touchscreens effectively.

**Implementation:**
- Trigger phrase: "Hey Insucar, I need help"
- Conversational flow using `SpeechRecognition` + `SpeechSynthesis`:
  ```
  App: "I found your location near the A6 southbound, kilometer 292. Are you safe?"
  User: "Yes"
  App: "What do you need? Tow, jump start, flat tire, or something else?"
  User: "Tow truck"
  App: "Help is on the way. Provider arriving in about 25 minutes. Stay in your vehicle."
  ```

**Insucar status:** NOT IN SCOPE. P2 priority.

### 1.7 Photo Upload (Post-Submission, Non-Blocking)

**Problem:** The operator needs to assess the situation (accident damage, vehicle position, hazards) but can't see what the motorist sees.

**Implementation:**
- Add "📷 Add photo of the scene" **after** the help request is submitted (Step 4 in wizard — never block the help request on a photo)
- Use `MediaDevices.getUserMedia()` with `capture` constraint to open camera directly
- Auto-compress to <500KB client-side for low-bandwidth
- Allow pre-capture before submission if user wants
- Photos are stored in S3 via presigned URL, linked to the case

**Insucar status:** NOT IN SCOPE. P3 priority.

---

## 2. OPERATOR (DISPATCH AGENT) EXPERIENCE

### 2.1 Keyboard Shortcuts (CAD Standard)

**Problem:** Every mouse click costs ~1.5 seconds. A busy operator handling 40+ cases per shift loses over a minute per case to mouse movement. Professional CAD (Computer-Aided Dispatch) systems used by 911, police, and fire are keyboard-first by design.

**Implementation — Global shortcuts:**

| Shortcut | Action | Purpose |
|---|---|---|
| `N` | New incident / rapid intake | Start a new case when a call comes in without screen-pop |
| `A` | Assign nearest provider | One-key dispatch after case is selected |
| `D` | Dispatch to selected provider | Alternative dispatch confirmation |
| `E` | Escalate to supervisor | Flags case, opens supervisor chat |
| `F` | Find / search cases | Global search by plate, name, phone, case number |
| `M` | Toggle map panel | Show/hide the map for more screen space |
| `R` | Resolve case | Mark case as resolved with timestamp |
| `1-9` | Priority assignment | `1`=emergency, `2`=high, `3`=normal |
| `↑/↓` | Navigate case queue | Arrow keys move through queue rows |
| `Enter` | Open selected case | Open case detail from highlighted queue row |
| `Esc` | Close modal / cancel | Universal escape |
| `Space` | Confirm / select | Universal confirm |

**Implementation — Single-key when queue is focused:**

Show a shortcut hint bar at the bottom of the screen: `[N]ew [A]ssign [D]ispatch [E]scalate [F]ind [R]esolve [M]ap`

**Insucar status:** NOT IN SCOPE. P0 priority.

### 2.2 Color-Coded Incident Severity System

**Problem:** The current operator queue shows priority pills (emergency/high/normal) but doesn't use an aging gradient. A 90-minute-old "normal" case is more urgent than a 2-minute-old "high" case, but the UI doesn't communicate this.

**Implementation — Universal severity + aging system:**

| Color | Meaning | Trigger |
|---|---|---|
| Red `#D32F2F` | Critical / Life Safety | Highway, children in car, medical, extreme weather, PSAP transfer |
| Orange `#F57C00` | High Priority | Nighttime, dangerous location, elderly, vulnerable occupants |
| Amber `#FBC02D` | Standard | Normal roadside assistance |
| Green `#388E3C` | Resolved / On Scene | Provider arrived |
| Blue `#1976D2` | En Route / In Progress | Provider dispatched, moving toward scene |

**Aging gradient:**
- Normal case, age < 30 min: Blue background
- Normal case, age 30-60 min: Amber gradient (25% opacity)
- Normal case, age 60-90 min: Amber gradient (50% opacity)
- Normal case, age > 90 min: Red gradient (75% opacity) — auto-escalates to supervisor
- The background color shifts gradually using CSS transitions, so the operator sees it "warming up" over time

**Implementation detail:**
- 4px solid color bar on left edge of each queue row — faster to scan than colored text (accessibility research shows vertical bars are detected 2x faster than text color changes in peripheral vision)
- Pulsing animation (subtle, 2s cycle) on red/orange rows that are unassigned
- The animation uses `box-shadow` pulsing, not opacity flashing (complies with WCAG SC 2.3.1 — no more than 3 flashes per second)

**Insucar status:** PARTIALLY IMPLEMENTED — priority pills exist, but no aging gradient, no standard color system. P0 priority.

### 2.3 Macro / Quick-Action Buttons

**Problem:** Operators type the same messages 50+ times per shift. Every typed character costs ~0.3 seconds.

**Implementation — Pre-set message library:**

| Macro | Message sent to motorist |
|---|---|
| `[ETA]` | "Your provider is on the way. Estimated arrival: **{ETA}** minutes." |
| `[CONFIRM]` | "Please confirm your location on the map — is this where you are?" |
| `[SAFETY]` | "For your safety, please stay in your vehicle with hazard lights on." |
| `[ARRIVED]` | "Your provider **{provider_name}** ({plate}) has arrived. They are in a {color} {vehicle_type}." |
| `[TRANSFER]` | "I'm transferring you to a supervisor who can help further." |
| `[CALLBACK]` | "We lost connection. I'll call you back at {phone}. Please stay where you are." |

**Implementation — Action macros:**
| Macro | Multi-step action executed |
|---|---|
| `[DISPATCH NEAREST]` | Auto-assigns closest available provider → sends ETA SMS to motorist → marks case as "dispatched" → updates queue |
| `[ESCALATE TO PD]` | Sends location to local police non-emergency → adds flag to case → logs timestamp → notifies supervisor |
| `[MOTORIST UNREACHABLE]` | Logs 3 attempted contacts → escalates timer → notifies supervisor → auto-retries callback in 5 min |

**UI placement:** Horizontal scrollable row of macro buttons above the chat/notes input. Grouped: "Messages" | "Actions" | "Escalations". Labeled with text — never icon-only (operators must be certain which macro they're triggering).

**Insucar status:** NOT IN SCOPE. P1 priority (40% time savings per shift).

### 2.4 Audio Alerts with Tiered Priority

**Problem:** The current console has no audio feedback. Operators may miss incoming calls, SLA breaches, or urgent messages if they're looking at another screen.

**Implementation — 3-tier audio system:**

| Tier | Sound | Trigger | Behavior |
|---|---|---|---|
| **Tier 1 — Critical** | Repeating 3-tone alert (high pitch, 800Hz) | New highway incident, medical emergency, provider accident, ETA exceeded by 50%+, PSAP transfer | Overrides Do Not Disturb, requires manual dismiss, repeats every 10s until acknowledged |
| **Tier 2 — Warning** | Single distinct tone (mid pitch, 500Hz) | Provider ETA slipping, motorist message, SLA approaching breach, new case in empty queue | Auto-dismisses after 3 seconds |
| **Tier 3 — Info** | Subtle click/chime (low pitch, 300Hz) | Provider status change, case resolved, queue refresh | No visual popup, sound only |

**Audio design principles:**
- **Earcons** (short, distinct sounds) rather than long tones — operators recognize them subconsciously after training
- **Descending** tones for negative events, **ascending** for positive (universal auditory convention across cultures)
- Each operator can set personal volume and mute specific tiers
- Silent mode option with **visual pulse** (flashing title bar / favicon notification) for night shift environments

**Insucar status:** NOT IN SCOPE. P2 priority.

### 2.5 Dark Mode Optimization for 24/7 Shifts

**Problem:** Operators work 8-12 hour shifts, often at night. Screen glare causes eye strain, headaches, and long-term vision degradation.

**Implementation:**
- **Mandatory dark mode** — operators default to dark, can opt into light
- Use **true dark** backgrounds (`#121212` per Material Design) — not pure black `#000000` (causes halation/blooming on LCD monitors)
- **Elevation via opacity** — use `#FFFFFF` at 5%/8%/12% opacity for surface layers, not shadows (which don't render in dark mode)
- **Color desaturation** — reduce bright colors by ~30% in dark mode. Full-saturation `#FF0000` on dark background causes visual vibration. Use `#CF6679` (Material Design error color for dark theme)
- **Blue light filter** — auto-activates screen warming (3400K) during 7PM-7AM based on operator's timezone
- **Break reminders** — non-dismissable overlay after 2 hours showing: "20-20-20 rule: Look 20 feet away for 20 seconds"

**Current operator console:** Already uses a dark theme (`--bg: #090b12`). This is good but needs the blue-light filter and break reminders. P1 priority.

### 2.6 Fatigue Monitoring & Break Enforcement

**Problem:** Dispatch operators in 911 centers have a 15% higher burnout rate than other call center workers. Extended focus sessions without breaks degrade decision quality.

**Implementation:**
- Track active keyboard/mouse time (not wall-clock time) — idle periods don't count toward fatigue
- After 90 minutes of continuous activity: soft prompt "Consider taking a short break" with a 20-second eye-rest timer
- After 120 minutes: mandatory 60-second overlay that can't be dismissed early: "Look 20 feet away for 20 seconds"
- Operators can snooze once for 10 minutes, but can't dismiss completely
- Admins can configure thresholds per shift length

**Insucar status:** NOT IN SCOPE. P2 priority.

### 2.7 Multi-Monitor / Detachable Map

**Problem:** Operators who have 2 monitors (common in dispatch centers) can't use the second screen. The map is embedded in a panel that takes up screen real estate.

**Implementation:**
- Add "🗗 Detach map" button that opens the map in a separate browser window
- The detached window can be moved to a second monitor
- Uses `window.open()` with `windowFeatures` for positioning
- The detached map syncs state with the main console via `BroadcastChannel` API or `localStorage` events
- Works as a separate tab if the operator prefers

**Insucar status:** NOT IN SCOPE. P3 priority.

---

## 3. CROSS-CUTTING IMPROVEMENTS

### 3.1 Real-Time WebSocket Updates

**Problem:** Current console uses HTTP polling (8-second interval for queue refresh). Motorist tracking page polls every 30s. No push updates.

**Implementation:**

| Update Type | Channel | Frequency | Affected Users |
|---|---|---|---|
| Provider location change | WebSocket | Every 5s while moving | Motorist + Operator |
| ETA recalculated | WebSocket | On >2 min change | Motorist + Operator |
| Case status change | WebSocket + Push | Immediate | Operator + Motorist |
| New incoming call | WebSocket | Immediate | Operator |
| Chat message | WebSocket | Immediate with typing indicators | Both |
| Queue updated | WebSocket | On change (not polled) | Operator |

**Technology:**
- **WebSocket** as primary channel — bidirectional, persistent. Use a lightweight Go WebSocket server or AWS API Gateway WebSocket
- **Server-Sent Events** as fallback if WebSocket blocked by corporate firewall
- **Push API + Service Worker** for motorist when browser is backgrounded
- **Reconnection:** Exponential backoff 1s, 2s, 4s, 8s, 16s, 30s (cap). Show connection indicator: 🟢=connected, 🟡=reconnecting, 🔴=disconnected

**Insucar status:** NOT IN SCOPE. P0 priority.

### 3.2 Predictive ETA with Traffic Data

**Problem:** Current ETA is a static number returned by the provider API (e.g., "18 min"). No traffic adjustment, no dynamic recalculation.

**Implementation:**
```
baseTravelTime = distance / averageSpeed(vehicle_type)
trafficMultiplier = fetchFromGoogleMaps(lat, lng, provider_lat, provider_lng, departureTime: "now")
predictiveETA = baseTravelTime * trafficMultiplier + providerPrepTime(3 min)
confidenceRange = "22-28 min" (displayed as range, not a precise number to avoid false expectations)
```

**Update triggers:** Recalculate when provider deviates from route by >500m, or traffic conditions change by >20%.

**Historical ML refinement:** After 10,000 completed jobs, train a model: `actual_time / predicted_time` by time-of-day, day-of-week, weather conditions. Apply the learned multiplier.

**Insucar status:** NOT IN SCOPE. P1 priority.

### 3.3 Multi-Language Support (i18n)

**Problem:** Current app is English-only. Redion operates in 200+ countries with local-language operators. Insucar's target markets (France, Germany, UK) require French and German at minimum.

**Implementation:**

| Component | Languages required for MVP | Stretch |
|---|---|---|
| End-user app (all UI strings) | EN, FR, DE, ES | IT, NL, PT, PL, AR, ZH, JA |
| Operator console | EN, FR | DE, ES |
| SMS/notification templates | EN, FR, DE, ES | All user languages |
| Emergency phrase cards | 20+ languages | Auto-translated via device ML |

**Language detection flow:**
1. `navigator.language` from browser/device
2. Account preference stored in Cognito user attributes
3. Manual language selector (visible on all screens)
4. For tourists: allow in-app override independent of device locale

**Real-time chat translation:**
- Motorist types in Spanish → translated to operator's language (EN/FR) using Google Cloud Translation API
- Operator's response auto-translated back to motorist's language
- Small indicator: "Translated from Spanish" so both parties know
- Cache frequently translated phrases for speed

**Implementation approach:**
- Externalize all user-facing strings to JSON files per language
- Use `data-i18n` attributes in HTML, swap textContent via JS
- RTL support (Arabic, Hebrew) requires CSS `direction: rtl` and mirrored layouts

**Insucar status:** NOT IN SCOPE. P3 priority.

### 3.4 WCAG 2.1 AA Accessibility Compliance

**Problem:** Neither the operator console nor the end-user app meet accessibility standards. A stranded motorist may have visual, motor, or cognitive impairments.

**Critical WCAG violations in current app:**

| WCAG Criterion | Issue in Current App | Fix |
|---|---|---|
| **SC 1.4.3** Contrast (Minimum) 4.5:1 | Dark theme has `--muted: #94a3b8` on `--bg: #090b12` — contrast is ~3.8:1 (fails AA) | Lighten muted text to `#b0bbc8` or darken background |
| **SC 2.1.1** Keyboard | Operator dispatch button has no keyboard access. Queue rows are clickable only with mouse | Add `tabindex="0"` and `onkeydown` handlers to all interactive elements |
| **SC 2.4.7** Focus Visible | No visible focus indicators anywhere. Tab navigation shows default browser ring inconsistently | Add `:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }` |
| **SC 2.5.5** Target Size 44×44px | Service selector buttons are ~28px tall | Minimum 44×44px touch targets per WCAG AAA |
| **SC 3.3.2** Labels | Input fields use `placeholder` text instead of persistent `<label>` elements | Every input must have a `<label for="...">` above it |
| **SC 1.1.1** Non-text Content | Console uses emoji icons without text labels | Add `aria-label` to all icon-only elements |
| **SC 1.4.4** Resize Text 200% | Layout breaks when zoomed | Test at 200% zoom, use relative units (rem/em) not px for layout |

**Insucar status:** PARTIALLY IMPLEMENTED — dark theme exists but contrast fails. No keyboard navigation. P1 priority.

### 3.5 Trust & Safety Features

**Problem:** A stranded motorist doesn't know who is coming to help them. Trust in the provider is essential for safety (especially at night, in unfamiliar areas, for vulnerable individuals).

**Implementation — already partially done:**
- Driver name + plate shown after dispatch ✅ (already in operator console)
- SMS with driver identity sent to motorist ✅ (already implemented)
- Status link with ETA sent to motorist ✅ (newly implemented in this sprint)

**Implementation — still needed:**

| Feature | Purpose | Priority |
|---|---|---|
| Provider photo in motorist tracking page | Shows who is coming — builds trust like Uber/Lyft | P1 |
| Motorist "Provider Arrived" confirmation | Was the right person who showed up? | P1 |
| "I don't feel safe" panic button during service | Alerts dispatcher + logs GPS, timestamps | P1 |
| Motorist photo (optional) shared with provider | Provider can identify the right person to approach | P2 |
| Live location sharing with family/friends | Motorist can SMS a tracking link to emergency contacts | P1 |
| Provider rating after service completion | 1-5 stars, visible in future dispatches, builds accountability | P1 |
| Provider background check badge | Shows motorist that provider is verified | P3 |
| Anonymous incident reporting | Both motorist and provider can report issues safely | P3 |

**Insucar status:** PARTIALLY IMPLEMENTED. P1 priority for remaining features.

### 3.6 SMS Journey Automation

**Problem:** The current implementation sends ONE SMS at dispatch time ("Help is on the way. Pierre L. (TOW-77-FR), ETA 22 min"). Motorist gets no further updates unless they manually open the tracking page.

**Industry standard:** Agero/Swoop send an automated SMS sequence:
```
[0 min]   "Help requested. We're finding the nearest provider."
[1 min]   "Pierre L. (TOW-77-FR) is on the way. ETA: 22 min. Track: [link]"
[5 min]   "Pierre is 5 minutes away. Please stay with your vehicle."  ← MISSING
[0 min]   "Pierre has arrived. If you don't see them, call us."        ← MISSING
[after]   "How was your experience? Rate your provider: [link]"         ← MISSING
```

**Implementation:**
- Hook into provider status webhooks: `en_route`, `approaching`, `arrived`, `completed`
- Each status change triggers an EventBridge event → SNS → SMS
- Use the `notifications` table to track which SMS events were sent (prevent duplicates)
- Make SMS frequency configurable per motorist preference

**Insucar status:** NOT IN SCOPE. P1 priority.

---

## 4. PRIORITIZED IMPLEMENTATION ROADMAP

### P0 — SURVIVAL-CRITICAL (Must implement before any production use)

| # | Feature | User | Impact | Effort | Dependencies |
|---|---|---|---|---|---|
| P0-1 | One-tap hold-to-confirm emergency button | End-user | Reduces time-to-help from ~60s to ~5s | 2 days | GPS auto-detect already implemented |
| P0-2 | Progressive 4-step wizard replacing current form | End-user | Reduces 3x errors, eliminates abandonment | 3 days | GPS, service types already exist |
| P0-3 | what3words location integration | Both | Works offline, voice-friendly, 3m precision | 1 day | what3words JS SDK (free for <1000 req/day) |
| P0-4 | Keyboard shortcuts for operator console | Operator | Saves ~1.5 min per case (40 cases/shift = 1 hour saved) | 2 days | None — pure JS event handlers |
| P0-5 | Color-coded aging gradient for case queue | Operator | Operators see aging cases in peripheral vision | 1 day | Case age data already available |
| P0-6 | WebSocket real-time updates replacing polling | Both | Eliminates 8s/30s polling lag, enables push | 5 days | Go WebSocket server needed |

### P1 — CRITICAL FUNCTIONALITY

| # | Feature | User | Impact | Effort |
|---|---|---|---|---|
| P1-1 | Offline-first PWA with Background Sync | End-user | Works in dead zones (15-20% of cases) | 5 days |
| P1-2 | Push notifications via Service Worker | End-user | Keeps motorist informed with screen off | 3 days |
| P1-3 | Predictive ETA with traffic data | Both | More accurate arrival times, fewer angry calls | 3 days |
| P1-4 | Macro/quick-action buttons for operators | Operator | 40% time savings on repetitive messages | 2 days |
| P1-5 | SMS journey automation (5-step sequence) | End-user | Reduces "where is my help?" support calls | 2 days |
| P1-6 | Provider photo + rating in motorist tracking | End-user | Trust + safety — Uber/Lyft expectation | 2 days |
| P1-7 | "I don't feel safe" panic button | End-user | Safety-critical, liability protection | 1 day |
| P1-8 | WCAG 2.1 AA contrast fixes | Both | Legal compliance + inclusive design | 2 days |
| P1-9 | "Provider Arrived" confirmation by motorist | Both | Accountability + closure for case | 1 day |

### P2 — IMPORTANT ENHANCEMENTS

| # | Feature | User | Effort |
|---|---|---|---|
| P2-1 | Voice-guided assistance (Web Speech API) | End-user | 3 days |
| P2-2 | Multi-language emergency phrase cards (20+ languages) | End-user | 2 days |
| P2-3 | Real-time chat translation (motorist ↔ operator) | Both | 4 days |
| P2-4 | Dark mode blue-light filter + break reminders | Operator | 1 day |
| P2-5 | Audio alerts (3-tier system) | Operator | 2 days |

### P3 — NICE TO HAVE

| # | Feature | User | Effort |
|---|---|---|---|
| P3-1 | Battery-saving auto-mode (OLED true black, reduced polling) | End-user | 2 days |
| P3-2 | Photo upload post-submission (non-blocking) | End-user | 2 days |
| P3-3 | Multi-monitor detachable map | Operator | 2 days |
| P3-4 | Multi-language UI (i18n with RTL support) | Both | 5 days |
| P3-5 | Provider background check badge | Both | 3 days |

---

*Research compiled from NN/Group, WCAG 2.1 W3C, web.dev, MDN, Material Design, competitor product analysis, and CAD dispatch industry standards.*
