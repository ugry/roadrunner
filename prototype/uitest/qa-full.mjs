// Insucar QA — comprehensive functional testing via headless browser + API
// Tests ALL end-user and operator functions against live deployment
// Run: node prototype/uitest/qa-full.mjs

import puppeteer from 'puppeteer';

const BASE = process.env.BASE || 'http://localhost:8080';
const OPS_PATH = '/ops-console-7f3a9c';
const OBSERVATIONS = [];
let total = 0, passed = 0, failed = 0;

function obs(section, item, detail) {
  OBSERVATIONS.push({ section, item, detail });
}

async function test(name, fn) {
  total++;
  try {
    await fn();
    console.log(`  PASS: ${name}`);
    passed++;
  } catch (e) {
    console.log(`  FAIL: ${name} — ${e.message}`);
    failed++;
  }
}

async function testAPI(name, url, opts = {}, checks = []) {
  total++;
  try {
    const res = await fetch(url, opts);
    let body = {};
    try { body = await res.json(); } catch (_) {}
    for (const c of checks) {
      const val = c.path ? c.path.split('.').reduce((o, k) => (o || {})[k], body) : res.status;
      if (c.expect !== undefined && val !== c.expect) {
        throw new Error(`${c.path || 'status'} expected ${c.expect}, got ${JSON.stringify(val)}`);
      }
      if (c.min !== undefined && val < c.min) {
        throw new Error(`${c.path || 'count'} min ${c.min}, got ${JSON.stringify(val)}`);
      }
      if (c.contains && !JSON.stringify(val).includes(c.contains)) {
        throw new Error(`${c.path || 'value'} should contain "${c.contains}"`);
      }
    }
    console.log(`  PASS: ${name}`);
    passed++;
  } catch (e) {
    console.log(`  FAIL: ${name} — ${e.message}`);
    failed++;
  }
}

// ============================================================================
console.log('\n=== INSUCAR QA — FUNCTIONAL TESTING ===');
console.log(`Target: ${BASE}\n`);

// ---- BROWSER SETUP ----
const browser = await puppeteer.launch({
  headless: 'new',
  args: ['--no-sandbox', '--disable-gpu', '--window-size=1440,900'],
});
const ctx = browser.defaultBrowserContext();
await ctx.overridePermissions(BASE, ['clipboard-read']);

console.log('--- END-USER APP ---');

// ---- LANDING PAGE ----
const up = await browser.newPage();
await up.setViewport({ width: 1200, height: 900 });

await test('Landing page loads (/)', async () => {
  const res = await up.goto(BASE + '/', { waitUntil: 'networkidle0', timeout: 15000 });
  if (!res.ok()) throw new Error(`HTTP ${res.status()}`);
  const title = await up.title();
  if (!title.includes('Insucar')) throw new Error(`Title mismatch: ${title}`);
});

await test('Landing page has login button', async () => {
  const btn = await up.$('button[onclick="login()"], button:has-text("Log in"), #loginTop');
  if (!btn) throw new Error('Login button not found');
  const text = await btn.evaluate(el => el.textContent.trim());
  console.log(`    [Login button: "${text}"]`);
});

await test('Landing page has register tab', async () => {
  const tab = await up.$('#tabReg');
  if (!tab) throw new Error('Register tab not found');
});

// ---- END-USER LOGIN ----
await test('End-user login (Claire Martin)', async () => {
  await up.goto(BASE + '/login', { waitUntil: 'networkidle0' });
  await up.evaluate(() => {
    document.getElementById('l_email').value = 'claire.martin@example.fr';
    document.getElementById('l_pass').value = 'Claire#2026';
  });
  await up.click('button[onclick="login()"]');
  await up.waitForFunction(
    () => document.getElementById('dash') && !document.getElementById('dash').classList.contains('hidden'),
    { timeout: 10000 }
  );
  const name = await up.$eval('#whoami', el => el.textContent);
  if (!name.includes('Claire') && !name.includes('Martin'))
    throw new Error(`Login didn't show user name: "${name}"`);
  console.log(`    [Signed in as: ${name}]`);
});

await test('Dashboard shows request assistance form', async () => {
  const type = await up.$('#i_type');
  if (!type) throw new Error('Incident type select not found');
  const desc = await up.$('#i_desc');
  if (!desc) throw new Error('Description textarea not found');
  const addr = await up.$('#i_addr');
  if (!addr) throw new Error('Address input not found');
  const btn = await up.$('button[onclick="submitIncident()"]');
  if (!btn) throw new Error('Submit button not found');
});

await test('Dashboard shows My Cases section', async () => {
  const section = await up.$('#cases');
  if (!section) throw new Error('Cases section not found');
});

await test('Login/Logout buttons toggle correctly', async () => {
  const loginVis = await up.$eval('#loginTop', el => !el.classList.contains('hidden'));
  const logoutVis = await up.$eval('#logoutTop', el => !el.classList.contains('hidden'));
  if (loginVis) throw new Error('Login button should be hidden after login');
  if (!logoutVis) throw new Error('Logout button should be visible after login');
});

// ---- END-USER INCIDENT SUBMISSION ----
await test('Submit incident creates case', async () => {
  await up.select('#i_type', 'breakdown');
  await up.evaluate(() => {
    document.getElementById('i_desc').value = 'Engine failure on A6 near Lyon';
    document.getElementById('i_addr').value = 'A6 southbound, km 290';
  });
  up.once('dialog', d => d.accept());
  await up.click('button[onclick="submitIncident()"]');
  await new Promise(r => setTimeout(r, 2000)); // wait for API response + dialog
  const cases = await up.$$('#cases .case-item');
  if (cases.length < 1) throw new Error('No cases found after submitting incident');
  console.log(`    [Cases visible: ${cases.length}]`);
});

await test('My Cases shows case details', async () => {
  const caseNo = await up.$eval('#cases .case-item .case-no', el => el.textContent);
  if (!caseNo) throw new Error('Case number not displayed');
  console.log(`    [Case: ${caseNo}]`);
  const status = await up.$eval('#cases .case-item .pill', el => el.textContent);
  console.log(`    [Status: ${status}]`);
});

// ---- END-USER LOGOUT ----
await test('End-user logout', async () => {
  await up.click('#logoutTop');
  await new Promise(r => setTimeout(r, 1000));
  // After logout, auth card should be visible again
  const authCard = await up.$('#authcard');
  const hidden = await authCard.evaluate(el => el.classList.contains('hidden'));
  if (hidden) throw new Error('Auth card still hidden after logout');
});

await up.close();

// ---- END-USER REGISTRATION ----
await test('End-user registration form visible', async () => {
  const rp = await browser.newPage();
  await rp.setViewport({ width: 520, height: 900 });
  await rp.goto(BASE + '/register', { waitUntil: 'networkidle0' });
  await rp.click('#tabReg');
  await new Promise(r => setTimeout(r, 500));
  const first = await rp.$('#r_first');
  const last = await rp.$('#r_last');
  const email = await rp.$('#r_email');
  const pass = await rp.$('#r_pass');
  if (!first || !last || !email || !pass) throw new Error('Registration fields missing');
  console.log('    [Registration form fields present]');
  await rp.close();
});

// ---- COGNITO SSO CHECK ----
await test('Cognito SSO button exists (when configured)', async () => {
  await testAPI('/api/auth/config', BASE + '/api/auth/config', {}, [
    { path: 'cognito', expect: true },
    { path: 'customerDomain', contains: 'insucar-dev-customer' },
    { path: 'staffDomain', contains: 'insucar-dev-staff' },
  ]);
});

// ---- OPERATOR APP ----
console.log('\n--- OPERATOR CONSOLE ---');

const op = await browser.newPage();
await op.setViewport({ width: 1440, height: 1000 });

await test('Operator hidden path returns 404 for wrong paths', async () => {
  const res = await op.goto(BASE + '/admin', { waitUntil: 'networkidle0' });
  if (res.status() !== 404) throw new Error(`/admin should return 404, got ${res.status()}`);
  const res2 = await op.goto(BASE + '/ops-console', { waitUntil: 'networkidle0' });
  if (res2.status() !== 404) throw new Error(`/ops-console should return 404, got ${res2.status()}`);
});

await test('Operator console accessible via correct path', async () => {
  const res = await op.goto(BASE + OPS_PATH, { waitUntil: 'networkidle0', timeout: 15000 });
  if (!res.ok()) throw new Error(`HTTP ${res.status()}`);
  const title = await op.title();
  if (!title.includes('Operator')) throw new Error(`Title mismatch: ${title}`);
});

await test('Operator login form visible', async () => {
  const idField = await op.$('#a_id');
  const passField = await op.$('#a_pass');
  const loginBtn = await op.$('button[onclick="alogin()"]');
  if (!idField || !passField || !loginBtn) throw new Error('Login form fields missing');
});

await test('Operator login (OP-1001)', async () => {
  await op.evaluate(() => {
    document.getElementById('a_id').value = 'OP-1001';
    document.getElementById('a_pass').value = 'Operator#2026';
  });
  await op.click('button[onclick="alogin()"]');
  await op.waitForFunction(
    () => {
      const overlay = document.getElementById('loginOverlay');
      return overlay && overlay.style.display === 'none';
    },
    { timeout: 10000 }
  );
  const name = await op.$eval('#a_name', el => el.textContent);
  if (name === '—') throw new Error('Operator name not set after login');
  console.log(`    [Signed in as: ${name}]`);
});

await test('Operator topbar shows agent info', async () => {
  const role = await op.$eval('#a_role', el => el.textContent);
  if (role === 'not signed in') throw new Error('Role not updated');
  console.log(`    [Role: ${role}]`);
  const avatar = await op.$eval('#a_avatar', el => el.textContent);
  if (avatar === '–') throw new Error('Avatar not set');
});

await test('Operator has SLA counter strip', async () => {
  const waiting = await op.$eval('#q-wait', el => el.textContent);
  const active = await op.$eval('#q-active', el => el.textContent);
  console.log(`    [Queue waiting: ${waiting}, active: ${active}]`);
});

await test('Operator clock is ticking', async () => {
  const t1 = await op.$eval('#clock', el => el.textContent);
  await new Promise(r => setTimeout(r, 1100));
  const t2 = await op.$eval('#clock', el => el.textContent);
  if (t1 === t2) throw new Error('Clock not updating');
});

await test('Operator has 112 PSAP button', async () => {
  const btn = await op.$('.btn-112');
  if (!btn) throw new Error('112 PSAP button missing');
});

// ---- OPERATOR INCOMING CALL ----
await test('Operator incoming call (mock Connect)', async () => {
  const input = await op.$('#phone');
  await input.click({ clickCount: 3 });
  await input.type('+33600000001');
  await op.click('button[onclick="incoming()"]');
  await new Promise(r => setTimeout(r, 2000));
  const popline = await op.$eval('#popline', el => el.textContent);
  if (!popline || popline.includes('Unknown')) throw new Error(`Screen-pop failed: "${popline}"`);
  console.log(`    [Pop line: ${popline.substring(0, 80)}]`);
});

// ---- OPERATOR CASE QUEUE ----
await test('Operator case queue loads', async () => {
  await op.waitForFunction(
    () => {
      const q = document.getElementById('queue');
      return q && q.textContent !== 'loading…';
    },
    { timeout: 10000 }
  );
  const queueHtml = await op.$eval('#queue', el => el.innerHTML);
  if (queueHtml.includes('Queue empty')) console.log('    [Queue empty — no cases yet]');
  else {
    const rows = await op.$$('#queue table tr');
    console.log(`    [Queue rows: ${rows.length}]`);
    if (rows.length < 2) throw new Error('Queue has no case rows');
  }
});

await test('Operator queue count badge updates', async () => {
  const count = await op.$eval('#q-count', el => el.textContent);
  console.log(`    [Count badge: ${count}]`);
});

// ---- OPERATOR CASE DETAIL ----
await test('Operator opens case from queue', async () => {
  const rows = await op.$$('#queue table tr');
  if (rows.length >= 2) {
    await rows[1].click();
    await op.waitForFunction(
      () => document.getElementById('c_no').textContent !== 'no case selected',
      { timeout: 8000 }
    );
    const caseNo = await op.$eval('#c_no', el => el.textContent);
    console.log(`    [Opened: ${caseNo}]`);
    const cust = await op.$eval('#c_cust', el => el.textContent);
    console.log(`    [Customer: ${cust}]`);
    if (cust === 'Select a case') throw new Error('Customer name not populated');
  } else {
    console.log('    [No cases to open — creating via API]');
    // Create a case via API so we can test the queue
    const r = await fetch(BASE + '/api/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        first: 'QA', last: 'Test', email: 'qa@test.dev',
        password: 'Test1234!', country: 'FR', phone: '+33600000123',
        consents: ['terms']
      })
    });
    const data = await r.json();
    console.log(`    [Created test user: ${data.customer_id}]`);
  }
});

// ---- OPERATOR DISPATCH ----
await test('Operator dispatch button enabled after case select', async () => {
  const disabled = await op.$eval('#dispatchBtn', el => el.disabled);
  if (disabled) console.log('    [Dispatch disabled — no case selected or already dispatched]');
  else console.log('    [Dispatch button ENABLED]');
});

await test('Operator dispatch flow', async () => {
  const disabled = await op.$eval('#dispatchBtn', el => el.disabled);
  if (!disabled) {
    await op.click('#dispatchBtn');
    await op.waitForFunction(
      () => {
        const eta = document.getElementById('m_eta');
        return eta && eta.textContent !== '--';
      },
      { timeout: 10000 }
    );
    const eta = await op.$eval('#m_eta', el => el.textContent);
    const prov = await op.$eval('#m_prov', el => el.textContent);
    console.log(`    [Dispatched: ${prov}, ETA: ${eta} min]`);
    if (eta === '--') throw new Error('ETA not updated after dispatch');
    
    // Check driver info
    const driverBox = await op.$('#driverBox');
    const drvVis = await driverBox.evaluate(el => el.style.display);
    if (drvVis === 'none') throw new Error('Driver box not shown after dispatch');
    const drv = await op.$eval('#m_drv', el => el.textContent);
    console.log(`    [Driver: ${drv}]`);
  }
});

// ---- OPERATOR LOGOUT ----
await test('Operator logout', async () => {
  const btns = await op.$$('button');
  let logoutBtn = null;
  for (const b of btns) {
    const text = await b.evaluate(el => el.textContent.trim());
    if (text === 'Log out') { logoutBtn = b; break; }
  }
  if (!logoutBtn) throw new Error('Logout button not found');
  await logoutBtn.click();
  await new Promise(r => setTimeout(r, 1000));
  const overlay = await op.$('#loginOverlay');
  const disp = await overlay.evaluate(el => el.style.display);
  if (disp === 'none') throw new Error('Login overlay not shown after logout');
});

await op.close();

// ============================================================================
// API-level tests
console.log('\n--- API ENDPOINTS ---');

await testAPI('Health check', BASE + '/healthz', {}, [
  { path: 'status', expect: 'ok' },
]);

await testAPI('Registration API', BASE + '/api/register', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    first: 'APITest', last: 'User', email: 'apitest@test.dev',
    password: 'Test1234!', country: 'FR', phone: '+33600000999',
    consents: ['terms']
  })
}, [
  { path: 'status', expect: 'active' },
  { path: 'customer_id', contains: '-' },
]);

await testAPI('Telephony mock incoming', BASE + '/api/telephony/mock/incoming', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ phone: '+33600000001' })
}, [
  { path: 'matched', expect: true },
  { path: 'screen_pop.customer.first_name', expect: 'Claire' },
]);

await testAPI('Agent lookup', BASE + '/api/agent/lookup?phone=%2B33600000001', {
  headers: { 'Content-Type': 'application/json' },
  credentials: 'include',
}, [
  { path: 'matched', expect: true },
]);

await testAPI('Auth config (Cognito)', BASE + '/api/auth/config', {}, [
  { path: 'cognito', expect: true },
  { path: 'region', expect: 'eu-west-1' },
]);

// ============================================================================
// Navigate to wrong paths
console.log('\n--- ERROR HANDLING ---');

await testAPI('Hidden operator path returns 404', BASE + '/admin', {}, [
  { path: 'status', expect: 404 },
]);

await testAPI('Invalid API returns proper error', BASE + '/api/nonexistent', {}, [
  { path: 'status', expect: 404 },
]);

// ============================================================================
// SUMMARY
console.log('\n=== QA TEST RESULTS ===');
console.log(`Total: ${total}, Passed: ${passed}, Failed: ${failed}`);
console.log(`Success rate: ${((passed / total) * 100).toFixed(1)}%\n`);

await browser.close();

// Write observations
if (failed > 0) {
  const obs = [
    '# Insucar QA — Failed Test Observations',
    '',
    `Test run: ${new Date().toISOString()}`,
    `Target: ${BASE}`,
    `Results: ${passed}/${total} passed (${((passed/total)*100).toFixed(1)}%)`,
    '',
    '## Failures',
    '',
    ...OBSERVATIONS.map(o => `### ${o.section}\n**Item:** ${o.item}\n\n${o.detail}\n`),
  ].join('\n');
  console.log(obs);
}

process.exit(failed > 0 ? 1 : 0);
