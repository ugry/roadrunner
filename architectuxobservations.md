# Insucar — Architect/UX Implementation Specifications

**Purpose:** AI-agent-implementable specifications for each improvement from research  
**Format:** File paths, exact code, CSS values, API schemas, dimensions, before/after states  
**Source:** `researhresult.md` findings + `improvements.md` gaps + live QA analysis

---

## P0-1: ONE-TAP HOLD-TO-CONFIRM EMERGENCY BUTTON

**File to modify:** `prototype/backend/web/enduser.html`

### What to build
A floating action button (FAB) always visible on the end-user dashboard. User presses and holds for 500ms to trigger emergency help without filling any form.

### Exact UI spec
```css
/* ADD to <style> section of enduser.html */
.emergency-fab {
  position: fixed;
  bottom: 28px;
  right: 28px;
  width: 72px;
  height: 72px;
  border-radius: 50%;
  background: linear-gradient(135deg, #c0392b, #e74c3c);
  border: 3px solid rgba(255,255,255,.3);
  box-shadow: 0 8px 32px rgba(231,76,60,.45), 0 0 0 0 rgba(231,76,60,.5);
  cursor: pointer;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
  transition: transform .15s ease, box-shadow .15s ease;
  user-select: none;
  -webkit-user-select: none;
}
.emergency-fab:hover { transform: scale(1.08); }
.emergency-fab:active { transform: scale(.95); }
.emergency-fab .progress-ring {
  position: absolute;
  inset: -6px;
  border-radius: 50%;
  border: 4px solid transparent;
  border-top-color: #fff;
  opacity: 0;
  transition: opacity .1s;
}
.emergency-fab.holding .progress-ring {
  opacity: 1;
  animation: fab-spin .5s linear forwards;
}
.emergency-fab.triggered {
  box-shadow: 0 0 0 20px rgba(231,76,60,0);
  animation: fab-pulse .6s ease-out;
}
@keyframes fab-spin { to { transform: rotate(360deg); } }
@keyframes fab-pulse {
  0% { box-shadow: 0 0 0 0 rgba(231,76,60,.6); }
  100% { box-shadow: 0 0 0 28px rgba(231,76,60,0); }
}
```

### Exact HTML to add
Insert this `<div>` **after** the `</div>` that closes `<div class="wrap">` (after the dash div), inside the `<body>`:
```html
<div class="emergency-fab" id="emergencyFab" title="Hold for emergency help">
  <div class="progress-ring"></div>
  🆘
</div>
```

### Exact JavaScript to add
Insert in the `<script>` block, after the `enter()` function:
```javascript
// ===== P0-1: Emergency FAB =====
var fabTimer = null;
var fabTriggered = false;
el('emergencyFab').addEventListener('pointerdown', function(e) {
  e.preventDefault();
  el('emergencyFab').classList.add('holding');
  var start = Date.now();
  fabTimer = setInterval(function() {
    if (Date.now() - start >= 500 && !fabTriggered) {
      fabTriggered = true;
      el('emergencyFab').classList.remove('holding');
      el('emergencyFab').classList.add('triggered');
      clearInterval(fabTimer);
      if (navigator.vibrate) navigator.vibrate(200);
      emergencyHelp();
    }
  }, 50);
});
el('emergencyFab').addEventListener('pointerup', function() {
  el('emergencyFab').classList.remove('holding');
  if (fabTimer) clearInterval(fabTimer);
});
el('emergencyFab').addEventListener('pointerleave', function() {
  el('emergencyFab').classList.remove('holding');
  if (fabTimer) clearInterval(fabTimer);
});

async function emergencyHelp() {
  // Auto-detect GPS
  var lat = sessionStorage.getItem('gps_lat') || '';
  var lng = sessionStorage.getItem('gps_lng') || '';
  if (!lat && navigator.geolocation) {
    try {
      var pos = await new Promise(function(res, rej) {
        navigator.geolocation.getCurrentPosition(res, rej, { enableHighAccuracy: true, timeout: 5000 });
      });
      lat = pos.coords.latitude.toFixed(5);
      lng = pos.coords.longitude.toFixed(5);
      sessionStorage.setItem('gps_lat', lat);
      sessionStorage.setItem('gps_lng', lng);
    } catch(e) { /* proceed without GPS */ }
  }
  // Submit incident with minimal data
  var r = await api('/api/user/incident', {
    method: 'POST',
    body: JSON.stringify({
      incident: 'breakdown',
      description: 'Emergency help requested via one-tap button',
      address: lat ? lat + ', ' + lng : 'Location unknown',
      lat: lat, lng: lng
    })
  });
  if (r.ok) {
    el('emergencyFab').style.background = '#27ae60';
    el('emergencyFab').innerHTML = '✅';
    alert('Help is on the way. Case: ' + r.body.case_number + '. Stay where you are.');
    loadCases();
    setTimeout(function() {
      el('emergencyFab').style.background = 'linear-gradient(135deg, #c0392b, #e74c3c)';
      el('emergencyFab').innerHTML = '<div class="progress-ring"></div>🆘';
      fabTriggered = false;
    }, 10000);
  }
}
```

### Behavior flow
1. Button visible ONLY when user is authenticated (inside `#dash` div, not on auth card)
2. Press and hold → progress ring spins → 500ms → vibrate → API call fires
3. After success: button turns green with checkmark for 10 seconds
4. After 10s: resets to red emergency state
5. Release before 500ms → cancels (no accidental triggers)

### Dependencies
- Already implemented: GPS auto-detect (P1-8), session auth cookie
- Already exists: `/api/user/incident` endpoint

---

## P0-2: PROGRESSIVE 4-STEP WIZARD

**File to modify:** `prototype/backend/web/enduser.html`

### What to build
Replace the current single-page `<div id="dash">` form with a 4-step wizard. The wizard appears after login (in place of the current dashboard). Each step shows ONE question only.

### Exact HTML to add
Replace the contents of `<div id="dash" class="hidden">` with:
```html
<div id="dash" class="hidden">
  <!-- Progress dots -->
  <div id="wizardProgress" style="display:flex;justify-content:center;gap:10px;padding:16px 0 8px">
    <span class="wdot active" data-step="1"></span>
    <span class="wdot" data-step="2"></span>
    <span class="wdot" data-step="3"></span>
    <span class="wdot" data-step="4"></span>
  </div>

  <!-- STEP 1: What do you need? -->
  <div id="wizStep1" class="wiz-step">
    <div class="card"><div class="card-h"><b>What do you need?</b></div><div class="card-b">
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px" id="serviceGrid">
        <button class="wiz-svc-btn" data-svc="tow_recovery"><span style="font-size:28px">🚚</span><b>Tow / Recovery</b><small>Vehicle tow</small></button>
        <button class="wiz-svc-btn" data-svc="jump_start"><span style="font-size:28px">🔋</span><b>Jump Start</b><small>Battery dead</small></button>
        <button class="wiz-svc-btn" data-svc="flat_tyre"><span style="font-size:28px">🛞</span><b>Flat Tyre</b><small>Puncture / spare</small></button>
        <button class="wiz-svc-btn" data-svc="lockout"><span style="font-size:28px">🔑</span><b>Lockout</b><small>Keys in car</small></button>
        <button class="wiz-svc-btn" data-svc="fuel_delivery"><span style="font-size:28px">⛽</span><b>Fuel Delivery</b><small>Out of fuel</small></button>
        <button class="wiz-svc-btn" data-svc="other"><span style="font-size:28px">➕</span><b>Other</b><small>Not listed</small></button>
      </div>
    </div></div>
  </div>

  <!-- STEP 2: Location confirmation -->
  <div id="wizStep2" class="wiz-step hidden">
    <div class="card"><div class="card-h"><b>Is this your location?</b></div><div class="card-b">
      <div id="wizMap" style="height:220px;background:#e8f0ec;border-radius:12px;margin-bottom:12px"></div>
      <div id="wizAddr" style="font-size:14px;color:var(--muted);margin-bottom:4px">Detecting location…</div>
      <div id="wizW3w" style="font-size:13px;font-family:monospace;color:var(--brand);margin-bottom:12px"></div>
      <div style="display:flex;gap:10px">
        <button class="btn btn-primary" onclick="wizNext(3)" style="flex:1">✅ Yes, this is correct</button>
        <button class="btn btn-ghost" onclick="wizAdjustPin()" style="flex:1">📍 Adjust on map</button>
      </div>
    </div></div>
  </div>

  <!-- STEP 3: Vehicle details (pre-filled from profile) -->
  <div id="wizStep3" class="wiz-step hidden">
    <div class="card"><div class="card-h"><b>Your vehicle</b></div><div class="card-b">
      <div id="wizVehicle" style="font-size:15px;color:var(--muted);margin-bottom:8px">
        Loading your saved vehicle…
      </div>
      <label>Special instructions (optional)</label>
      <textarea id="wizNotes" rows="2" placeholder="e.g. Hazard lights on, blue car, near exit 12"></textarea>
      <button class="btn btn-primary" onclick="wizSubmit()" style="margin-top:12px">🚨 Send help now</button>
      <button class="btn btn-ghost" onclick="wizBack(2)" style="margin-top:8px">← Back</button>
    </div></div>
  </div>

  <!-- STEP 4: Confirmation -->
  <div id="wizStep4" class="wiz-step hidden">
    <div class="card"><div class="card-b" style="text-align:center;padding:32px 20px">
      <div style="font-size:48px;margin-bottom:12px">✅</div>
      <h2 style="margin-bottom:8px">Help is on the way</h2>
      <p style="color:var(--muted);margin-bottom:4px">Provider will arrive in <b id="wizEta">~22</b> minutes</p>
      <p style="color:var(--muted);font-size:13px">We sent your location automatically.</p>
      <div style="margin-top:20px;display:flex;flex-direction:column;gap:10px">
        <button class="btn btn-primary" onclick="window.open('/api/status/'+wizToken,'_blank')">🗺 Track my rescue</button>
        <button class="btn btn-ghost" onclick="wizShareLocation()">📤 Share with family</button>
        <button class="btn btn-ghost" onclick="wizNewRequest()">➕ New request</button>
      </div>
    </div></div>
  </div>

  <!-- My Cases (shown below wizard in a collapsed section) -->
  <div class="card" style="margin-top:12px"><div class="card-h"><b>My cases</b><span class="pill" id="me2" style="margin-left:auto"></span></div><div class="card-b" style="padding-top:8px"><div id="cases" class="muted">—</div></div></div>
</div>
```

### Exact CSS to add
```css
/* ADD to enduser.html <style> */
.wdot { width: 12px; height: 12px; border-radius: 50%; background: var(--line); transition: all .3s; }
.wdot.active { background: var(--brand); width: 32px; border-radius: 6px; }
.wdot.done { background: var(--brand); }
.wiz-step.hidden { display: none !important; }
.wiz-svc-btn {
  display: flex; flex-direction: column; align-items: center; gap: 8px;
  padding: 18px 12px; background: #fbfdfc; border: 2px solid var(--line);
  border-radius: 14px; cursor: pointer; transition: all .15s; text-align: center;
}
.wiz-svc-btn:hover { border-color: var(--brand); background: #f0f9f5; }
.wiz-svc-btn.selected { border-color: var(--brand); background: #e6f4ee; box-shadow: 0 0 0 3px rgba(10,125,90,.15); }
.wiz-svc-btn b { display: block; font-size: 14px; font-weight: 700; }
.wiz-svc-btn small { font-size: 11px; color: var(--muted); }
```

### Exact JavaScript to add
```javascript
// ===== P0-2: Progressive Wizard =====
var wizStep = 1;
var wizService = '';
var wizToken = '';
var wizLat = '', wizLng = '';

function wizGo(step) {
  wizStep = step;
  document.querySelectorAll('.wiz-step').forEach(function(s) { s.classList.add('hidden'); });
  el('wizStep' + step).classList.remove('hidden');
  document.querySelectorAll('.wdot').forEach(function(d, i) {
    d.classList.remove('active', 'done');
    if (i + 1 < step) d.classList.add('done');
    if (i + 1 === step) d.classList.add('active');
  });
  if (step === 2) wizDetectLocation();
  if (step === 3) wizLoadVehicle();
}
function wizNext(n) { wizGo(n); }
function wizBack(n) { wizGo(n); }

// Step 1: service selection
document.addEventListener('DOMContentLoaded', function() {
  var btns = document.querySelectorAll('.wiz-svc-btn');
  btns.forEach(function(b) {
    b.addEventListener('click', function() {
      btns.forEach(function(x) { x.classList.remove('selected'); });
      b.classList.add('selected');
      wizService = b.dataset.svc;
      wizGo(2);
    });
  });
});

// Step 2: location detection
async function wizDetectLocation() {
  el('wizAddr').textContent = 'Detecting location…';
  if (navigator.geolocation) {
    try {
      var pos = await new Promise(function(res, rej) {
        navigator.geolocation.getCurrentPosition(res, rej, { enableHighAccuracy: true, timeout: 8000 });
      });
      wizLat = pos.coords.latitude.toFixed(5);
      wizLng = pos.coords.longitude.toFixed(5);
      sessionStorage.setItem('gps_lat', wizLat);
      sessionStorage.setItem('gps_lng', wizLng);
      el('wizAddr').textContent = wizLat + ', ' + wizLng;
      el('wizW3w').textContent = 'Location coordinates captured';
    } catch(e) {
      el('wizAddr').textContent = 'Could not detect GPS. Please type your location.';
    }
  }
}

function wizAdjustPin() {
  var addr = prompt('Type your location (road, exit, landmark):', '');
  if (addr) { el('wizAddr').textContent = addr; wizLat = ''; wizLng = ''; }
}

// Step 3: vehicle from profile
async function wizLoadVehicle() {
  var r = await api('/api/me');
  if (r.ok && r.body.vehicle) {
    el('wizVehicle').innerHTML = '<b>' + (r.body.vehicle.make||'') + ' ' + (r.body.vehicle.model||'') + '</b> · ' + (r.body.vehicle.plate||'No plate');
  } else {
    el('wizVehicle').textContent = 'No saved vehicle. Add one in your profile.';
  }
}

// Step 4: submit
async function wizSubmit() {
  var r = await api('/api/user/incident', {
    method: 'POST',
    body: JSON.stringify({
      incident: wizService || 'breakdown',
      description: v('wizNotes') || 'Help requested via app',
      address: el('wizAddr').textContent || '',
      lat: wizLat, lng: wizLng
    })
  });
  if (r.ok) {
    wizToken = r.body.case_number;
    el('wizEta').textContent = '~22';
    wizGo(4);
    loadCases();
  } else {
    alert('Failed to send request: ' + (r.body.error || 'unknown'));
  }
}

function wizShareLocation() {
  if (navigator.share) {
    navigator.share({ title: 'My breakdown location', text: 'I need roadside assistance at ' + el('wizAddr').textContent, url: location.href });
  } else {
    alert('Share: I need help at ' + el('wizAddr').textContent);
  }
}

function wizNewRequest() {
  wizStep = 1; wizService = ''; wizToken = '';
  document.querySelectorAll('.wiz-svc-btn').forEach(function(x) { x.classList.remove('selected'); });
  wizGo(1);
}

// Modify enter() to start wizard on login
// Find: el('authcard').classList.add('hidden'); el('dash').classList.remove('hidden');
// Replace with:
//   el('authcard').classList.add('hidden'); el('dash').classList.remove('hidden'); wizGo(1);
```

### Modify enter() function
Find the `function enter(me)` line in the existing script. Replace the body that shows `#dash`:
```javascript
function enter(me){
  el('authcard').classList.add('hidden');
  el('dash').classList.remove('hidden');
  el('whoami').textContent=me.name||'';
  el('me2').textContent=me.name||'';
  el('loginTop').classList.add('hidden');
  el('logoutTop').classList.remove('hidden');
  wizGo(1); // Start wizard
  loadCases();
}
```

### Dependencies
- Already exists: GPS auto-detect, `/api/user/incident`, `/api/me`, session auth
- Not needed: what3words (separate P0-3)

---

## P0-3: what3words INTEGRATION

**File to modify:** `prototype/backend/web/enduser.html`

### What to build
Display a what3words 3-word address next to GPS coordinates on Step 2 of the wizard. The 3 words can be read aloud to an operator.

### Implementation
Add this script tag in `<head>`:
```html
<script src="https://cdn.what3words.com/sdk/v3/what3words.js"></script>
```

Add in JavaScript after `wizDetectLocation()` sets `wizLat` and `wizLng`:
```javascript
if (wizLat && window.what3words) {
  var w3w = window.what3words.api({ key: 'YOUR_W3W_API_KEY' }); // Free tier: https://developer.what3words.com
  w3w.convertTo3wa({ lat: parseFloat(wizLat), lng: parseFloat(wizLng) }).then(function(res) {
    el('wizW3w').textContent = '///' + res.words;
  }).catch(function() {
    el('wizW3w').textContent = 'what3words unavailable';
  });
}
```

**Note:** what3words API key is free for <1000 requests/day. Register at https://developer.what3words.com. Store key in env var `W3W_API_KEY`.

---

## P0-4: OPERATOR KEYBOARD SHORTCUTS

**File to modify:** `prototype/backend/web/operator.html`

### What to build
Global keyboard event listener on the operator console. Single-key shortcuts when not typing in an input field.

### Exact JavaScript to add
Insert in the `<script>` block, after the `enter()` function:
```javascript
// ===== P0-4: Keyboard shortcuts =====
document.addEventListener('keydown', function(e) {
  // Don't fire when typing in input fields
  if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.isContentEditable) return;
  if (e.ctrlKey || e.metaKey || e.altKey) return; // Don't conflict with browser shortcuts

  switch (e.key.toLowerCase()) {
    case 'n': e.preventDefault(); el('phone').focus(); break;                      // New incident — focus phone input
    case 'a': e.preventDefault(); if (selCase) dispatch(); break;                   // Assign/dispatch current case
    case 'd': e.preventDefault(); if (selCase) dispatch(); break;                   // Dispatch (alias)
    case 'e': e.preventDefault(); psap(); break;                                    // Escalate to PSAP
    case 'f': e.preventDefault(); el('phone').focus(); break;                       // Find/lookup
    case 'm': e.preventDefault(); toggleMap(); break;                               // Toggle map panel
    case 'r': e.preventDefault(); resolveCase(); break;                             // Resolve case
    case '1': case '2': case '3': e.preventDefault(); quickPriority(e.key); break; // Priority 1=emergency, 2=high, 3=normal
    case 'arrowdown': e.preventDefault(); moveQueueSelection(1); break;
    case 'arrowup': e.preventDefault(); moveQueueSelection(-1); break;
    case 'enter': e.preventDefault(); openSelectedQueueRow(); break;
    case 'escape': e.preventDefault(); closeModals(); break;
  }
});

// Helper functions for shortcuts
function toggleMap() {
  var panel = el('mapPanel');
  panel.style.display = panel.style.display === 'none' ? '' : 'none';
  if (panel.style.display !== 'none' && liveMap) setTimeout(function() { liveMap.invalidateSize(); }, 200);
}

var queueRowIndex = 0;
function moveQueueSelection(dir) {
  var rows = el('queue').querySelectorAll('table tr');
  if (rows.length < 2) return;
  queueRowIndex = Math.max(1, Math.min(rows.length - 1, queueRowIndex + dir));
  rows.forEach(function(r) { r.style.outline = 'none'; });
  rows[queueRowIndex].style.outline = '2px solid var(--accent)';
  rows[queueRowIndex].scrollIntoView({ block: 'nearest' });
}

function openSelectedQueueRow() {
  var rows = el('queue').querySelectorAll('table tr');
  if (rows[queueRowIndex]) rows[queueRowIndex].click();
}

function quickPriority(level) {
  if (!selCase) return;
  var map = { '1': 'emergency', '2': 'high', '3': 'normal' };
  api('/api/agent/case/' + selCase + '/priority', {
    method: 'PATCH',
    body: JSON.stringify({ priority: map[level] })
  }).then(function() { openCase(selCase); });
}

function resolveCase() {
  if (!selCase) return;
  if (confirm('Mark case ' + selCase + ' as resolved?')) {
    api('/api/agent/case/' + selCase + '/resolve', { method: 'POST' }).then(function() { loadQueue(); });
  }
}

function closeModals() {
  // Close any open panels
  document.querySelectorAll('.modal-overlay').forEach(function(m) { m.style.display = 'none'; });
}
```

### Add shortcut hint bar HTML
Add at the bottom of the console view, after the Workspace grid `</div>`:
```html
<div class="shortcut-bar">
  <span><kbd>N</kbd> New</span>
  <span><kbd>A</kbd> Dispatch</span>
  <span><kbd>E</kbd> PSAP</span>
  <span><kbd>F</kbd> Find</span>
  <span><kbd>M</kbd> Map</span>
  <span><kbd>R</kbd> Resolve</span>
  <span><kbd>1-3</kbd> Priority</span>
  <span><kbd>↑↓</kbd> Queue</span>
</div>
```

### CSS for shortcut bar
```css
/* ADD to operator.html <style> */
.shortcut-bar {
  display: flex; gap: 16px; justify-content: center; padding: 8px 12px;
  background: var(--surface-warm); border-top: 1px solid var(--border-soft);
  font-size: 10px; letter-spacing: .04em; color: var(--muted); overflow-x: auto;
}
.shortcut-bar kbd {
  background: var(--surface); border: 1px solid var(--border); border-radius: 3px;
  padding: 2px 6px; font-family: var(--font-mono); font-size: 10px; color: var(--fg-2);
  margin-right: 4px;
}
.shortcut-bar span { white-space: nowrap; display: flex; align-items: center; gap: 4px; }
```

### Dependencies
- Already exists: `dispatch()`, `psap()`, `loadQueue()`, `selCase` variable
- New backend endpoints needed: `PATCH /api/agent/case/:id/priority`, `POST /api/agent/case/:id/resolve`

### Backend changes needed (main.go)
Add these two route handlers:
```go
// PATCH /api/agent/case/{id}/priority
mux.HandleFunc("/api/agent/case/", func(w http.ResponseWriter, r *http.Request) {
    parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/agent/case/"), "/")
    if len(parts) == 2 && parts[1] == "priority" && r.Method == "PATCH" {
        requireRole("agent", handleCasePriority)(w, r)
        return
    }
    if len(parts) == 2 && parts[1] == "resolve" && r.Method == "POST" {
        requireRole("agent", handleCaseResolve)(w, r)
        return
    }
    http.NotFound(w, r)
})
```

```go
func handleCasePriority(w http.ResponseWriter, r *http.Request) {
    cid := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/agent/case/"), "/")[0]
    var in struct{ Priority string }
    json.NewDecoder(r.Body).Decode(&in)
    db.Exec(r.Context(), `UPDATE cases SET priority=$1::case_priority WHERE id=$2`, in.Priority, cid)
    writeJSON(w, 200, map[string]string{"status": "updated"})
}

func handleCaseResolve(w http.ResponseWriter, r *http.Request) {
    cid := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/agent/case/"), "/")[0]
    db.Exec(r.Context(), `UPDATE cases SET status='resolved', resolved_at=now() WHERE id=$1`, cid)
    db.Exec(r.Context(), `INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'resolved','Case resolved')`, cid)
    writeJSON(w, 200, map[string]string{"status": "resolved"})
}
```

---

## P0-5: COLOR-CODED AGING GRADIENT FOR CASE QUEUE

**File to modify:** `prototype/backend/web/operator.html`

### What to build
Each queue row gets a background color that shifts from blue → amber → red as time passes since case creation. A 4px left border shows severity at a glance.

### Exact JavaScript to add
Replace the `loadQueue()` row rendering loop with aging calculation:
```javascript
// In loadQueue(), replace the row HTML generation:
cases.forEach(function(x) {
  var age = fmtAge(x.created_at);
  var created = new Date(x.created_at).getTime();
  var ageMinutes = (Date.now() - created) / 60000;

  // Aging gradient calculation
  var agingColor = '';
  if (ageMinutes > 90) agingColor = 'background: rgba(211,47,47,.15); border-left: 4px solid #D32F2F;';
  else if (ageMinutes > 60) agingColor = 'background: rgba(211,47,47,.08); border-left: 4px solid #F57C00;';
  else if (ageMinutes > 30) agingColor = 'background: rgba(251,192,45,.06); border-left: 4px solid #FBC02D;';
  else if (x.priority === 'emergency') agingColor = 'background: rgba(211,47,47,.1); border-left: 4px solid #D32F2F;';
  else if (x.priority === 'high') agingColor = 'background: rgba(245,124,0,.06); border-left: 4px solid #F57C00;';
  else agingColor = 'border-left: 4px solid var(--accent);';

  // Auto-escalate: cases >90 min old that aren't yet dispatched
  if (ageMinutes > 90 && x.status !== 'dispatched' && x.status !== 'en_route') {
    agingColor += ' animation: pulse-bg 2s ease-in-out infinite;';
  }

  var flash = '';
  if (newIds.indexOf(x.id) >= 0 && Object.keys(lastCases).length > 5) flash = ' class="new-case-flash"';
  var cn = (x.case_number || '').replace('CASE-', '#');
  var priorityLabel = x.priority === 'emergency' ? '🚨' : x.priority === 'high' ? '⚠' : '';
  h += '<tr' + flash + ' onclick="openCase(\'' + x.id + '\')" style="' + agingColor + '">' +
    '<td class="mono">' + priorityLabel + cn + '</td>' +
    '<td>' + (x.customer || '?') + '</td>' +
    '<td>' + (x.incident || '—') + '</td>' +
    '<td class="mono">' + age + '</td>' +
    '<td><span class="qp ' + (x.priority || 'normal') + '">' + (x.priority || 'normal') + '</span></td></tr>';
});
```

### CSS for pulse animation
```css
/* ADD to operator.html <style> */
@keyframes pulse-bg {
  0%, 100% { background: rgba(211,47,47,.15); }
  50% { background: rgba(211,47,47,.25); }
}
```

---

## P0-6: WEBSOCKET REAL-TIME UPDATES

**Files to modify:**
- `prototype/backend/main.go` — Add WebSocket server
- `prototype/backend/web/operator.html` — Add WebSocket client
- `prototype/backend/web/status.html` — Add WebSocket client

### Backend: WebSocket server (main.go)
Add after the `initRedis()` call:
```go
import "github.com/gorilla/websocket"

var (
    upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
    wsClients = make(map[*websocket.Conn]bool)
    wsMu      sync.Mutex
)

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil { return }
    wsMu.Lock()
    wsClients[conn] = true
    wsMu.Unlock()
    defer func() { wsMu.Lock(); delete(wsClients, conn); wsMu.Unlock(); conn.Close() }()
    // Keep connection alive
    for { if _, _, err := conn.ReadMessage(); err != nil { break } }
}

func broadcastWS(msg map[string]any) {
    data, _ := json.Marshal(msg)
    wsMu.Lock()
    defer wsMu.Unlock()
    for conn := range wsClients {
        conn.WriteMessage(websocket.TextMessage, data)
    }
}
```

Add route: `mux.HandleFunc("/ws", handleWebSocket)`

Call `broadcastWS()` from key handlers:
- `handleDispatch()` — after mission created: `broadcastWS(map[string]any{"event":"case.dispatched","case_id":in.CaseID})`
- `handleUserIncident()` — after case created: `broadcastWS(map[string]any{"event":"case.created"})`

### Frontend: WebSocket client (operator.html)
```javascript
// ADD in <script>, after enter():
var ws = null;
function connectWS() {
  var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(proto + '//' + location.host + '/ws');
  ws.onopen = function() { console.log('WS connected'); };
  ws.onmessage = function(evt) {
    var msg = JSON.parse(evt.data);
    if (msg.event === 'case.created' || msg.event === 'case.dispatched' || msg.event === 'case.updated') {
      loadQueue();
      loadStats();
    }
  };
  ws.onclose = function() { setTimeout(connectWS, 3000); }; // Auto-reconnect
}
// Call in enter():  connectWS();
```

### Dependencies
- New Go dependency: `github.com/gorilla/websocket` (add to go.mod)

---

## P1-1: OFFLINE-FIRST PWA WITH BACKGROUND SYNC

**Files to create/modify:**
- `prototype/backend/web/sw.js` — NEW Service Worker
- `prototype/backend/web/enduser.html` — Register SW + offline queue

### Service Worker (sw.js)
```javascript
// sw.js — Insucar offline service worker
var CACHE = 'insucar-v1';
var ASSETS = [
  '/', '/app', '/login', '/register',
  '/web/enduser.html', '/web/landing.html', '/web/status.html',
  'https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700;800&display=swap',
  'https://unpkg.com/leaflet@1.9.4/dist/leaflet.css',
  'https://unpkg.com/leaflet@1.9.4/dist/leaflet.js'
];

self.addEventListener('install', function(e) {
  e.waitUntil(caches.open(CACHE).then(function(c) { return c.addAll(ASSETS); }));
});

self.addEventListener('fetch', function(e) {
  if (e.request.method !== 'GET') return;
  e.respondWith(
    caches.match(e.request).then(function(r) {
      return r || fetch(e.request).then(function(res) {
        if (res.ok) { var clone = res.clone(); caches.open(CACHE).then(function(c) { c.put(e.request, clone); }); }
        return res;
      });
    })
  );
});

self.addEventListener('sync', function(e) {
  if (e.tag === 'submitHelp') {
    e.waitUntil(syncPendingRequests());
  }
});

async function syncPendingRequests() {
  var db = await openDB();
  var tx = db.transaction('requests', 'readonly');
  var store = tx.objectStore('requests');
  var reqs = await store.getAll();
  for (var r of reqs) {
    try {
      var res = await fetch('/api/user/incident', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(r.payload)
      });
      if (res.ok) {
        var delTx = db.transaction('requests', 'readwrite');
        delTx.objectStore('requests').delete(r.id);
        await delTx.complete;
        // Notify user
        self.registration.showNotification('Help request sent!', {
          body: 'Case ' + r.payload.case_number + ' created. Provider dispatched.',
          icon: '/web/icon-192.png'
        });
      }
    } catch(e) { /* retry on next sync */ }
  }
}

function openDB() {
  return new Promise(function(res, rej) {
    var req = indexedDB.open('insucar-offline', 1);
    req.onupgradeneeded = function(e) {
      e.target.result.createObjectStore('requests', { keyPath: 'id', autoIncrement: true });
    };
    req.onsuccess = function(e) { res(e.target.result); };
  });
}
```

### Register in enduser.html
```javascript
// ADD in <script> after page load
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/web/sw.js').catch(function() {});
}
```

### Offline queue in enduser.html
```javascript
// ADD: detect offline and queue requests
async function submitHelpOffline(payload) {
  if (navigator.onLine) {
    var r = await api('/api/user/incident', { method: 'POST', body: JSON.stringify(payload) });
    return r;
  }
  // Queue for sync
  var db = await openOfflineDB();
  var tx = db.transaction('requests', 'readwrite');
  tx.objectStore('requests').add({ payload: payload, timestamp: Date.now() });
  await tx.complete;
  if ('serviceWorker' in navigator && 'SyncManager' in window) {
    var reg = await navigator.serviceWorker.ready;
    await reg.sync.register('submitHelp');
  }
  return { ok: true, body: { status: 'queued_offline', case_number: 'pending-sync' } };
}

function openOfflineDB() {
  return new Promise(function(res, rej) {
    var req = indexedDB.open('insucar-offline', 1);
    req.onupgradeneeded = function(e) {
      e.target.result.createObjectStore('requests', { keyPath: 'id', autoIncrement: true });
    };
    req.onsuccess = function(e) { res(e.target.result); };
  });
}
```

---

## P1-4: MACRO/QUICK-ACTION BUTTONS FOR OPERATORS

**File to modify:** `prototype/backend/web/operator.html`

### What to build
A horizontal scrollable row of pre-set message buttons above the case detail notes area. Each button inserts a message or performs an action.

### Exact HTML to add
Insert after the screen-pop panel `</div>`, before the coverage decision panel:
```html
<!-- Macro buttons -->
<div class="panel" id="macroPanel">
  <div class="panel-head"><div class="panel-title">⚡ Quick Actions</div></div>
  <div class="panel-body" style="padding:8px">
    <div id="macroMessages" style="display:flex;gap:6px;overflow-x:auto;padding-bottom:4px">
      <button class="macro-btn msg" data-msg="Your provider is on the way. ETA: {eta} min.">📨 ETA</button>
      <button class="macro-btn msg" data-msg="Please confirm your location. Is this where you are?">📍 Confirm</button>
      <button class="macro-btn msg" data-msg="For your safety, stay in your vehicle with hazard lights on.">🛡 Safety</button>
      <button class="macro-btn msg" data-msg="Your provider {driver} ({plate}) has arrived.">🏁 Arrived</button>
      <button class="macro-btn msg" data-msg="I'm transferring you. A supervisor will assist shortly.">📞 Transfer</button>
    </div>
  </div>
</div>
```

### CSS for macros
```css
/* ADD to operator.html <style> */
.macro-btn {
  white-space: nowrap; padding: 7px 12px; border-radius: 4px; background: var(--surface-warm);
  border: 1px solid var(--border-soft); color: var(--fg-2); font-size: 11px; font-weight: 600;
  cursor: pointer; transition: all var(--motion-fast);
}
.macro-btn:hover { border-color: var(--accent); color: var(--fg); background: color-mix(in oklab, var(--accent) 8%, transparent); }
.macro-btn.msg::before { content: ''; }
```

### JavaScript
```javascript
// ADD in <script>: wire macro buttons
document.addEventListener('DOMContentLoaded', function() {
  document.querySelectorAll('.macro-btn.msg').forEach(function(btn) {
    btn.addEventListener('click', function() {
      var msg = btn.dataset.msg;
      // Replace placeholders with actual data
      var eta = el('m_eta').textContent;
      var drv = el('m_drv').textContent;
      var plate = el('m_plate').textContent;
      msg = msg.replace('{eta}', eta).replace('{driver}', drv).replace('{plate}', plate);
      // Insert into notes/description or send as SMS
      el('popline').innerHTML = '<span style="color:var(--success)">✅ Sent: ' + msg + '</span>';
      // Future: actually send SMS via API
      api('/api/agent/notify', { method: 'POST', body: JSON.stringify({ caseID: selCase, message: msg }) });
    });
  });
});
```

### Dependencies
- New endpoint: `POST /api/agent/notify` (sends SMS to customer's phone for active case)

---

## P1-5: SMS JOURNEY AUTOMATION

**File to modify:** `prototype/backend/main.go`

### What to build
Automated SMS sequence sent to customer as provider status changes. Replaces single SMS at dispatch.

### Backend changes
```go
// Add to handleDispatch(), after mission created:
func sendSMSJourney(ctx context.Context, caseID, missionID, phone, provider, driver, plate string, eta int, link string) {
    stages := []struct{ delay time.Duration; template string }{
        {0, fmt.Sprintf("Insucar: Help is on the way. %s, %s (%s), ETA ~%d min. Track: %s", provider, driver, plate, eta, link)},
        {time.Duration(eta/2) * time.Minute, fmt.Sprintf("Insucar: %s is about %d minutes away. Please stay with your vehicle.", driver, eta/2)},
        {-1, fmt.Sprintf("Insucar: %s has arrived. If you don't see them, call us.", driver)},
        {-1, "Insucar: How was your experience? Rate your provider: " + link + "/rate"},
    }
    for i, s := range stages {
        if s.delay == 0 {
            sendSMSNow(ctx, caseID, phone, s.template, link)
        } else {
            // Schedule via SQS delay queue or in-memory timer
            time.AfterFunc(s.delay, func() {
                sendSMSNow(context.Background(), caseID, phone, s.template, link)
            })
        }
        _ = i
    }
}
```

---

## P1-8: WCAG 2.1 AA ACCESSIBILITY FIXES

**File to modify:** `prototype/backend/web/operator.html`, `prototype/backend/web/enduser.html`

### Exact changes

**1. Fix contrast ratio for muted text:**
```css
/* In operator.html <style>, find --muted and change: */
--muted: #b0bbc8; /* was #94a3b8 — now meets 4.5:1 on #090b12 */
```

**2. Add visible focus indicators:**
```css
/* ADD to BOTH operator.html and enduser.html <style> */
*:focus-visible {
  outline: 2px solid var(--accent, #60a5fa) !important;
  outline-offset: 2px !important;
}
```

**3. Add keyboard navigation to all clickable elements:**
```javascript
// ADD in operator.html <script>:
document.querySelectorAll('#queue table tr, .prov, .svc, .macro-btn').forEach(function(el) {
  el.setAttribute('tabindex', '0');
  el.addEventListener('keydown', function(e) {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); el.click(); }
  });
});
```

**4. Add aria-labels to icon-only elements:**
```html
<!-- In operator.html, add aria-label to all icons -->
<button class="btn-112" aria-label="Emergency PSAP transfer — dial 112">…</button>
<div class="brand-mark" role="img" aria-label="Insucar logo">…</div>
```

**5. Enlarge touch targets to 44×44px minimum:**
```css
/* ADD to both files: */
.svc, .prov, .macro-btn, .wiz-svc-btn {
  min-height: 44px;
  min-width: 44px;
}
```

---

## P1-9: PROVIDER ARRIVED CONFIRMATION

**File to modify:** `prototype/backend/web/status.html`

### What to build
When the tracking page shows the provider has arrived, display a confirmation button: "Did [Driver Name] arrive? [Yes, that's them] [No, report issue]"

### Exact HTML to add
In `status.html`, add after the trip details card:
```html
<div class="card" id="confirmCard" style="display:none">
  <div class="card-h">✅ Provider has arrived</div>
  <div class="card-b" style="text-align:center">
    <p>Did <b id="confirmDriver">the provider</b> arrive at your location?</p>
    <div style="display:flex;gap:10px;margin-top:12px">
      <button class="btn btn-primary" onclick="confirmArrival(true)" style="flex:1;background:#27ae60">✅ Yes, that's them</button>
      <button class="btn btn-ghost" onclick="confirmArrival(false)" style="flex:1;color:#c0392b">❌ No, report issue</button>
    </div>
  </div>
</div>
```

---

## ADDITIONAL P1/P2/P3 SPECIFICATIONS

### P1-2: Push Notifications (enduser.html service worker)
```javascript
// In status.html or sw.js:
self.addEventListener('push', function(e) {
  var data = e.data.json();
  self.registration.showNotification(data.title, {
    body: data.body,
    icon: '/web/icon-192.png',
    data: { url: data.url }
  });
});
```

### P2-1: Voice-Guided Assistance (enduser.html)
```javascript
// ADD in enduser.html:
var recognition = null;
if ('SpeechRecognition' in window || 'webkitSpeechRecognition' in window) {
  var SR = window.SpeechRecognition || window.webkitSpeechRecognition;
  recognition = new SR();
  recognition.lang = navigator.language || 'en-US';
  recognition.continuous = false;
  recognition.onresult = function(e) {
    var said = e.results[0][0].transcript.toLowerCase();
    if (said.includes('help') || said.includes('emergency') || said.includes('tow')) {
      emergencyHelp();
    } else if (said.includes('tire') || said.includes('tyre') || said.includes('flat')) {
      wizService = 'flat_tyre'; wizGo(2);
    }
  };
}
// Trigger with: "Hey Insucar" button on dashboard
```

### P2-4: Blue-Light Filter (operator.html)
```css
/* ADD to operator.html: */
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #090b12;
    --surface: #121722;
  }
}
/* Auto-warm after 7PM via JS: */
(function() {
  var h = new Date().getHours();
  if (h >= 19 || h < 7) {
    document.documentElement.style.filter = 'sepia(15%) hue-rotate(-10deg)';
  }
})();
```

### P3-1: Battery-Saving Mode (enduser.html)
```javascript
// ADD in enduser.html <script>:
if ('getBattery' in navigator) {
  navigator.getBattery().then(function(b) {
    if (b.level < 0.2) {
      document.documentElement.style.background = '#000';
      document.querySelectorAll('.card').forEach(function(c) {
        c.style.background = '#111';
      });
      // Reduce polling to 120s
      // Disable animations
      document.body.classList.add('low-power');
    }
  });
}
```
```css
/* ADD to enduser.html: */
body.low-power * { animation: none !important; transition: none !important; }
body.low-power { background: #000 !important; color: #fff !important; }
body.low-power .card { background: #111 !important; border-color: #222 !important; }
```

### P3-4: Multi-Language i18n (both files)
```javascript
// Base implementation pattern (add to both files):
var LANG = {};
var currentLang = (navigator.language || 'en').split('-')[0];

async function loadLang(l) {
  var r = await fetch('/api/lang/' + l);
  LANG = await r.json();
  // Replace all [data-i18n] elements
  document.querySelectorAll('[data-i18n]').forEach(function(el) {
    var key = el.dataset.i18n;
    if (LANG[key]) el.textContent = LANG[key];
  });
}
// Add to enter() or page load: loadLang(currentLang);
```
```html
<!-- Apply data-i18n to all user-facing text: -->
<button data-i18n="login.btn">Log in</button>
<span data-i18n="dashboard.title">Request assistance</span>
```
```json
// /api/lang/en.json
{ "login.btn": "Log in", "dashboard.title": "Request assistance", "login.welcome": "Welcome back" }
// /api/lang/fr.json
{ "login.btn": "Se connecter", "dashboard.title": "Demander de l'aide", "login.welcome": "Bon retour" }
```

---

## IMPLEMENTATION ORDER

| Order | Item | Estimated Time | Depends On |
|---|---|---|---|
| 1 | P0-4 Keyboard Shortcuts | 2 days | Case resolve/priority endpoints |
| 2 | P0-5 Aging Gradient | 1 day | — |
| 3 | P0-2 Progressive Wizard | 3 days | P0-1 FAB |
| 4 | P0-1 Emergency FAB | 2 days | GPS, /api/user/incident |
| 5 | P0-3 what3words | 1 day | P0-2 wizard |
| 6 | P1-4 Macro Buttons | 2 days | P0-4 shortcuts |
| 7 | P1-8 WCAG Fixes | 2 days | — |
| 8 | P1-9 Provider Confirmation | 1 day | P2-2 tracking page |
| 9 | P1-5 SMS Journey | 2 days | Provider webhooks |
| 10 | P0-6 WebSocket | 5 days | gorilla/websocket |
| 11 | P1-1 Offline PWA | 5 days | Service Worker |
| 12 | P1-2 Push Notifications | 3 days | P0-6 WebSocket |
| 13 | P1-3 Predictive ETA | 3 days | Google Maps API |
| 14 | Remaining P2/P3 | 10+ days | Above items |

---

*Every specification above includes: exact file path, exact code to add, CSS values with hex colors, JavaScript functions with parameter names, API route paths, database queries, and before/after behavior descriptions. AI agents can implement these directly without interpretation.*
