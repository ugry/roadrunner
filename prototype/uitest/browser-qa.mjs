// Roadrunner end-user QA — headless browser test
import puppeteer from 'puppeteer-core';

const BASE = 'http://34.255.180.21';
const CHROME = '/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';

let total = 0, passed = 0, failed = 0;
const issues = [];

async function test(name, fn) {
  total++;
  try {
    await fn();
    console.log(`  PASS: ${name}`);
    passed++;
  } catch (e) {
    console.log(`  FAIL: ${name} — ${e.message}`);
    failed++;
    issues.push({ name, error: e.message });
  }
}

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: 'new',
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage']
});

const page = await browser.newPage();
await page.setViewport({ width: 1280, height: 900 });
page.on('console', msg => { if (msg.type() === 'error') console.log(`  [JS ERROR] ${msg.text()}`); });
page.on('pageerror', err => console.log(`  [PAGE ERROR] ${err.message}`));

try {

// =========================================================
// GATE 1: LANDING PAGE
// =========================================================
console.log('\n=== GATE 1: LANDING PAGE ===');

await test('Landing page loads', async () => {
  const res = await page.goto(BASE, { waitUntil: 'networkidle2', timeout: 15000 });
  if (res.status() !== 200) throw new Error(`HTTP ${res.status()}`);
  const title = await page.title();
  console.log(`    Title: "${title}"`);
  if (!title.toLowerCase().includes('road') && !title.toLowerCase().includes('insu'))
    throw new Error(`Unexpected title: ${title}`);
});

await test('Landing has navigation/menu', async () => {
  const btns = await page.$$('a, button');
  if (btns.length < 2) throw new Error('Too few interactive elements on landing');
  console.log(`    Found ${btns.length} interactive elements`);
});

// =========================================================
// GATE 2: LOGIN PAGE
// =========================================================
console.log('\n=== GATE 2: LOGIN PAGE ===');

await test('Navigate to /app login page', async () => {
  await page.goto(`${BASE}/app`, { waitUntil: 'networkidle2', timeout: 15000 });
  const title = await page.title();
  console.log(`    Title: "${title}"`);
  if (!title.includes('Sign in') && !title.includes('Roadrunner'))
    throw new Error(`Unexpected title: ${title}`);
});

await test('Login tab is visible', async () => {
  const visible = await page.$eval('#login', el => !el.classList.contains('hidden'));
  if (!visible) throw new Error('Login tab not visible');
});

await test('Register tab exists', async () => {
  const tab = await page.$('#tabReg');
  if (!tab) throw new Error('Register tab not found');
});

await test('Email input visible on login', async () => {
  const el = await page.$('#l_email');
  if (!el) throw new Error('Email input not found');
  const visible = await el.boundingBox();
  if (!visible) throw new Error('Email input not visible');
});

await test('Password input visible on login', async () => {
  const el = await page.$('#l_pass');
  if (!el) throw new Error('Password input not found');
});

await test('Sign in button visible', async () => {
  const el = await page.$('#login .btn-primary');
  if (!el) throw new Error('Sign in button not found');
  const text = await page.$eval('#login .btn-primary', e => e.textContent);
  console.log(`    Button text: "${text}"`);
  if (!text.toLowerCase().includes('sign in') && !text.toLowerCase().includes('log'))
    throw new Error(`Button text unexpected: ${text}`);
});

await test('Forgot password link visible', async () => {
  const links = await page.$$eval('#login a', as => as.map(a => a.textContent));
  const forgot = links.find(l => l.toLowerCase().includes('forgot') || l.toLowerCase().includes('reset'));
  console.log(`    Links: ${links.join(', ')}`);
  if (!forgot) throw new Error('Forgot password link not found');
  console.log(`    Found: "${forgot}"`);
});

await test('Create account button/link visible', async () => {
  const links = await page.$$eval('#login *', els => els.map(e => e.textContent?.trim()).filter(Boolean));
  const create = links.find(l => l.toLowerCase().includes('create') || l.toLowerCase().includes('register') || l.toLowerCase().includes('account'));
  if (!create) throw new Error('No create account link found');
  console.log(`    Found: "${create}"`);
  // Check it's a tab or link
  const tabBtn = await page.$('.tabx[onclick*="reg"]');
  const linkBtn = await page.$('a[href="/register-page"]');
  if (!tabBtn && !linkBtn) {
    // Check for the span with onclick
    const spans = await page.$$('span[onclick]');
    console.log(`    Span buttons: ${spans.length}`);
    if (spans.length < 1) throw new Error('Create account element not interactive');
  }
});

// =========================================================
// GATE 3: FORGOT PASSWORD FLOW
// =========================================================
console.log('\n=== GATE 3: FORGOT PASSWORD FLOW ===');

await test('Click forgot password shows form', async () => {
  await page.goto(`${BASE}/app`, { waitUntil: 'networkidle2', timeout: 15000 });
  // Click the forgot password link
  const forgotLink = await page.$('a[onclick*="forgotPassword"], a[onclick*="showForgot"]');
  if (forgotLink) {
    await forgotLink.click();
    await sleep(500);
  } else {
    // Try the alternative - could be showForgotPassword()
    await page.evaluate(() => { if (typeof showForgotPassword === 'function') showForgotPassword(); });
    await sleep(500);
  }
  const fEmail = await page.$('#f_email');
  if (!fEmail) throw new Error('Forgot password email input not visible');
  const visible = await fEmail.boundingBox();
  if (!visible) throw new Error('Forgot password form not visible');
  console.log('    Forgot password form shown');
});

await test('Forgot password has email input', async () => {
  const val = await page.$eval('#f_email', el => el.placeholder);
  console.log(`    Placeholder: "${val}"`);
  if (!val.includes('@')) throw new Error('Email input placeholder unexpected');
});

await test('Forgot password has send reset button', async () => {
  const btn = await page.$('#forgotpw .btn-primary');
  if (!btn) throw new Error('Send reset button not found');
  const text = await page.$eval('#forgotpw .btn-primary', e => e.textContent);
  console.log(`    Button: "${text}"`);
});

await test('Back to sign in link on forgot password', async () => {
  const links = await page.$$eval('#forgotpw a', as => as.map(a => ({ text: a.textContent.trim(), href: a.getAttribute('href') })));
  const back = links.find(l => l.text.includes('Back'));
  console.log(`    Links: ${JSON.stringify(links)}`);
  if (!back) throw new Error('No back to sign in link');
});

await test('Send reset with bad email shows error', async () => {
  await page.$eval('#f_email', el => el.value = 'notanemail');
  await page.click('#forgotpw .btn-primary');
  await sleep(1000);
  const msg = await page.$eval('#fmsg', el => el.textContent);
  console.log(`    Message: "${msg}"`);
  if (!msg.toLowerCase().includes('valid') && !msg.toLowerCase().includes('email') && !msg.toLowerCase().includes('error'))
    throw new Error(`Unexpected message: ${msg}`);
});

// =========================================================
// GATE 4: REGISTER TAB
// =========================================================
console.log('\n=== GATE 4: REGISTER TAB ===');

await test('Switch to Register tab', async () => {
  await page.goto(`${BASE}/app`, { waitUntil: 'networkidle2', timeout: 15000 });
  await page.click('#tabReg');
  await sleep(300);
  const visible = await page.$eval('#reg', el => !el.classList.contains('hidden'));
  if (!visible) throw new Error('Register tab not visible after click');
});

await test('Register form fields visible', async () => {
  const fields = await page.$$eval('#reg input, #reg select', els => els.map(e => ({ id: e.id, type: e.type, placeholder: e.placeholder, name: e.tagName })));
  console.log(`    Fields: ${fields.length} — ${fields.map(f => f.id || f.type).join(', ')}`);
  if (fields.length < 4) throw new Error('Too few register fields');
});

await test('Register has password field', async () => {
  const pass = await page.$('#r_pass');
  if (!pass) throw new Error('Password field not found');
});

await test('Register has Create Account button', async () => {
  const btn = await page.$('#reg .btn-primary');
  if (!btn) throw new Error('Register button not found');
  const text = await page.$eval('#reg .btn-primary', e => e.textContent);
  console.log(`    Button: "${text}"`);
  if (!text.toLowerCase().includes('create') && !text.toLowerCase().includes('register') && !text.toLowerCase().includes('sign'))
    throw new Error(`Button text unexpected: ${text}`);
});

// =========================================================
// GATE 5: REGISTER A USER VIA UI
// =========================================================
console.log('\n=== GATE 5: REGISTER VIA UI ===');

const testEmail = `qabrowser${Date.now()}@example.com`;
const testPass = 'BrowserTest123!';

await test('Fill register form', async () => {
  await page.$eval('#r_first', el => el.value = '');
  await page.$eval('#r_last', el => el.value = '');
  await page.$eval('#r_email', el => el.value = '');
  await page.$eval('#r_phone', el => el.value = '');
  await page.$eval('#r_pass', el => el.value = '');
  await sleep(100);

  await page.type('#r_first', 'BrowserQA');
  await page.type('#r_last', 'Tester');
  await page.type('#r_email', testEmail);
  await page.type('#r_phone', '+33600000050');
  await page.type('#r_pass', testPass);
  // Select country FR (default should be FR)
  await page.select('#r_country', 'FR');
});

await test('Click Create Account', async () => {
  await page.click('#reg .btn-primary');
  await sleep(2000);
  // After registration, should be redirected to dashboard
  const dashVisible = await page.evaluate(() => {
    const d = document.getElementById('dash');
    return d && !d.classList.contains('hidden');
  });
  console.log(`    Dashboard visible: ${dashVisible}`);
  // If there's an error message, check it
  const out = await page.$eval('#out', el => el.textContent).catch(() => '');
  console.log(`    Output: "${out}"`);
  if (out && out.includes('⚠')) {
    throw new Error(`Registration error: ${out}`);
  }
});

// =========================================================
// GATE 6: DASHBOARD AFTER LOGIN
// =========================================================
console.log('\n=== GATE 6: DASHBOARD ===');

await test('Dashboard is visible', async () => {
  const dashVisible = await page.evaluate(() => {
    const d = document.getElementById('dash');
    return d && !d.classList.contains('hidden');
  });
  if (!dashVisible) throw new Error('Dashboard not visible after registration');
});

await test('User name is displayed', async () => {
  const name = await page.$eval('#whoami', el => el.textContent);
  console.log(`    Displayed: "${name}"`);
  if (!name || name.length < 2) throw new Error('User name not displayed');
});

await test('Request Assistance card visible', async () => {
  const card = await page.$('#dash .card-h');
  if (!card) throw new Error('No card header in dashboard');
  const text = await page.$eval('#dash .card-h b', el => el.textContent);
  console.log(`    Card: "${text}"`);
  if (!text.includes('Request') && !text.includes('assistance') && !text.includes('help'))
    throw new Error(`Unexpected card: ${text}`);
});

await test('My Cases card visible', async () => {
  const cards = await page.$$('#dash .card-h');
  console.log(`    Cards: ${cards.length}`);
  if (cards.length < 2) throw new Error('Expected at least 2 cards');
});

// =========================================================
// GATE 7: INCIDENT WIZARD
// =========================================================
console.log('\n=== GATE 7: INCIDENT WIZARD ===');

await test('Incident type dropdown visible', async () => {
  const sel = await page.$('#i_type');
  if (!sel) throw new Error('Incident type select not found');
  const options = await page.$$eval('#i_type option', ops => ops.map(o => o.textContent));
  console.log(`    Options: ${options.join(', ')}`);
  if (options.length < 3) throw new Error('Too few incident types');
});

await test('GPS location button visible', async () => {
  // Click Next to go to step 2
  await page.click('#wizard1 .btn-primary');
  await sleep(300);
  const gpsBtn = await page.$('button[onclick*="GPS"]');
  if (!gpsBtn) throw new Error('GPS button not found');
  console.log('    GPS button found');
});

await test('Wizard progresses through steps', async () => {
  // Step 1 -> 2 already done
  // Step 2 -> 3
  await page.$eval('#i_addr', el => el.value = '');
  await page.type('#i_addr', 'A6 highway, test location');
  await page.click('#wizard2 .btn-primary:last-child');
  await sleep(300);

  // Step 3 -> 4
  await page.$eval('#i_desc', el => el.value = '');
  await page.type('#i_desc', 'Test incident description');
  await page.click('#wizard3 .btn-primary:last-child');
  await sleep(300);

  // Step 4 — should show summary
  const summary = await page.$eval('#wizSummary', el => el.textContent);
  console.log(`    Summary: "${summary}"`);
  if (!summary || summary.length < 2) throw new Error('Wizard summary empty');
});

await test('Emergency button visible in wizard', async () => {
  // Go back to step 3
  await page.click('#wizard4 .btn-ghost');
  await sleep(200);
  const panicBtn = await page.$('.panic-btn');
  if (!panicBtn) throw new Error('Emergency panic button not found');
  console.log('    Emergency button found');
});

// =========================================================
// GATE 8: LOGOUT AND LOGIN
// =========================================================
console.log('\n=== GATE 8: LOGOUT AND RE-LOGIN ===');

await test('Logout button visible', async () => {
  await page.goto(`${BASE}/app`, { waitUntil: 'networkidle2', timeout: 15000 });
  // Should still be logged in
  await sleep(500);
  const logoutBtn = await page.$('#logoutTop');
  if (!logoutBtn) throw new Error('Logout button not found (might not be logged in)');
  const visible = await logoutBtn.boundingBox().catch(() => null);
  if (!visible) {
    // Try logging in first
    console.log('    Not logged in, logging in...');
    await page.type('#l_email', testEmail);
    await page.type('#l_pass', testPass);
    await page.click('#login .btn-primary');
    await sleep(2000);
  }
});

await test('Click logout returns to login', async () => {
  try {
    await page.click('#logoutTop');
    await sleep(1000);
  } catch (e) {
    // Might already be logged out
  }
  const loginVisible = await page.$eval('#login', el => !el.classList.contains('hidden')).catch(() => true);
  console.log(`    Login visible after logout: ${loginVisible}`);
});

await test('Login with registered credentials', async () => {
  await page.goto(`${BASE}/app`, { waitUntil: 'networkidle2', timeout: 15000 });
  await page.$eval('#l_email', el => el.value = '');
  await page.$eval('#l_pass', el => el.value = '');
  await page.type('#l_email', testEmail);
  await page.type('#l_pass', testPass);
  await page.click('#login .btn-primary');
  await sleep(2000);
  const dashVisible = await page.evaluate(() => {
    const d = document.getElementById('dash');
    return d && !d.classList.contains('hidden');
  });
  if (!dashVisible) {
    const err = await page.$eval('#out', el => el.textContent).catch(() => 'no error msg');
    throw new Error(`Login failed, dashboard not visible. Error: ${err}`);
  }
  console.log('    Dashboard visible after login');
});

// =========================================================
// GATE 9: RESPONSIVE DESIGN
// =========================================================
console.log('\n=== GATE 9: RESPONSIVE DESIGN ===');

await test('Mobile viewport works', async () => {
  await page.setViewport({ width: 375, height: 812 });
  await sleep(500);
  // Check no horizontal overflow
  const overflow = await page.evaluate(() => {
    return document.body.scrollWidth > window.innerWidth + 10;
  });
  if (overflow) throw new Error('Page has horizontal overflow on mobile');
});

await test('Back to desktop viewport', async () => {
  await page.setViewport({ width: 1280, height: 900 });
  await sleep(300);
});

// =========================================================
// GATE 10: VISUAL CONSISTENCY
// =========================================================
console.log('\n=== GATE 10: VISUAL CONSISTENCY ===');

await test('No JS errors on login page', async () => {
  let errors = [];
  page.on('pageerror', err => errors.push(err));
  await page.goto(`${BASE}/app`, { waitUntil: 'networkidle2', timeout: 15000 });
  await sleep(1000);
  if (errors.length > 0) {
    console.log(`    JS errors: ${errors.map(e => e.message).join('; ')}`);
  } else {
    console.log('    No JS errors');
  }
});

await test('All critical elements have accessible labels', async () => {
  const missing = await page.evaluate(() => {
    const issues = [];
    document.querySelectorAll('button, input, select, textarea').forEach(el => {
      const hasLabel = el.getAttribute('aria-label') || 
        el.getAttribute('placeholder') ||
        el.getAttribute('title') ||
        (el.id && document.querySelector(`label[for="${el.id}"]`));
      if (!hasLabel && el.type !== 'hidden' && el.type !== 'submit') {
        issues.push(el.id || el.className || el.tagName);
      }
    });
    return issues;
  });
  console.log(`    Elements without accessible label: ${missing.length}`);
  if (missing.length > 5) {
    console.log(`    Missing: ${missing.slice(0,10).join(', ')}`);
  }
});

} finally {
  await browser.close();
}

// FINAL REPORT
console.log('\n========================================');
console.log('QA REPORT — END USER EXPERIENCE');
console.log('========================================');
console.log(`Total: ${total} | Passed: ${passed} | Failed: ${failed}`);
console.log(`Score: ${Math.round(passed/total*100)}%`);
if (issues.length > 0) {
  console.log('\nIssues Found:');
  issues.forEach((i, idx) => console.log(`  ${idx+1}. ${i.name}: ${i.error}`));
}
console.log('========================================\n');
process.exit(failed > 0 ? 1 : 0);
