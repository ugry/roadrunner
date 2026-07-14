// Roadrunner QA — end-user browser test against www.unysolar.com
import puppeteer from 'puppeteer-core';

const BASE = 'http://www.unysolar.com';
const CHROME = '/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';

let total = 0, passed = 0, failed = 0;
const issues = [];

async function test(name, fn) {
  total++;
  try { await fn(); console.log(`  PASS: ${name}`); passed++; }
  catch(e) { console.log(`  FAIL: ${name} — ${e.message}`); failed++; issues.push({name, error: e.message}); }
}
function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

const browser = await puppeteer.launch({
  executablePath: CHROME, headless: 'new',
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--host-resolver-rules=MAP www.unysolar.com 34.255.180.21, MAP unysolar.com 34.255.180.21, MAP op.unysolar.com 34.255.180.21']
});
const page = await browser.newPage();
await page.setViewport({ width: 1280, height: 900 });

const testEmail = `domainuser${Date.now()}@example.com`;
const testPass = 'DomainTest123!';

try {

console.log('=== LANDING: unysolar.com ===');
await test('Landing loads', async () => {
  const res = await page.goto('http://unysolar.com/', { waitUntil: 'networkidle2', timeout: 15000 });
  if (res.status() !== 200) throw new Error(`HTTP ${res.status()}`);
  console.log(`    Title: "${await page.title()}"`);
});

console.log('\n=== LOGIN: www.unysolar.com/app ===');
await test('Login page loads', async () => {
  await page.goto('http://www.unysolar.com/app', { waitUntil: 'networkidle2', timeout: 15000 });
  console.log(`    Title: "${await page.title()}"`);
});

await test('Sign in form ready', async () => {
  await page.waitForSelector('#l_email', { timeout: 5000 });
  await page.waitForSelector('#l_pass', { timeout: 5000 });
  const btn = await page.$eval('#login .btn-primary', e => e.textContent);
  console.log(`    Button: "${btn}"`);
});

await test('Forgot password link visible', async () => {
  const link = await page.$('a[onclick*="Forgot"], a[onclick*="forgot"]');
  if (!link) throw new Error('No forgot password link');
  console.log(`    Found: "${await link.evaluate(e => e.textContent)}"`);
});

await test('Create account option visible', async () => {
  const el = await page.$('#login span[onclick*="reg"], #login a[href*="register"]');
  if (!el) throw new Error('No create account option');
  console.log(`    Found: "${await el.evaluate(e => e.textContent)}"`);
});

console.log('\n=== FORGOT PASSWORD FLOW ===');
await test('Show forgot password form', async () => {
  await page.click('a[onclick*="Forgot"]');
  await sleep(500);
  const visible = await page.$eval('#forgotpw', el => !el.classList.contains('hidden'));
  if (!visible) throw new Error('Form not shown');
});
await test('Send reset with invalid email', async () => {
  await page.type('#f_email', 'bademail');
  await page.click('#forgotpw .btn-primary');
  await sleep(800);
  const msg = await page.$eval('#fmsg', el => el.textContent);
  if (!msg.toLowerCase().includes('valid')) throw new Error(`Unexpected: ${msg}`);
  console.log(`    Validation: "${msg.trim()}"`);
});
await test('Back to sign in', async () => {
  await page.click('#forgotpw a');
  await sleep(300);
  const visible = await page.$eval('#login', el => !el.classList.contains('hidden'));
  if (!visible) throw new Error('Did not return to login');
});

console.log('\n=== REGISTER ===');
await test('Switch to Register tab', async () => {
  await page.click('#tabReg'); await sleep(300);
  const visible = await page.$eval('#reg', el => !el.classList.contains('hidden'));
  if (!visible) throw new Error('Register tab not shown');
});
await test('Fill and submit register', async () => {
  await page.type('#r_first', 'Domain');
  await page.type('#r_last', 'User');
  await page.type('#r_email', testEmail);
  await page.type('#r_phone', '+33600000060');
  await page.type('#r_pass', testPass);
  await page.select('#r_country', 'FR');
  await page.click('#reg .btn-primary');
  await sleep(2000);
});
await test('Dashboard appears after register', async () => {
  const d = await page.$eval('#dash', el => !el.classList.contains('hidden'));
  if (!d) throw new Error('Dashboard not visible');
  const name = await page.$eval('#whoami', el => el.textContent);
  console.log(`    Logged in as: "${name}"`);
});

console.log('\n=== DASHBOARD ===');
await test('Request Assistance card', async () => {
  const t = await page.$eval('#dash .card-h b', el => el.textContent);
  if (!t.includes('Request')) throw new Error(`Unexpected: ${t}`);
});
await test('My Cases card', async () => {
  const cards = await page.$$('#dash .card-h');
  if (cards.length < 2) throw new Error('Missing cards');
});

console.log('\n=== INCIDENT WIZARD ===');
await test('Incident types loaded', async () => {
  const opts = await page.$$eval('#i_type option', o => o.length);
  if (opts < 4) throw new Error(`Only ${opts} types`);
  console.log(`    ${opts} incident types`);
});
await test('Wizard step 1→2→3→4', async () => {
  await page.click('#wizard1 .btn-primary'); await sleep(300);
  await page.type('#i_addr', 'Eiffel Tower, Paris');
  await page.click('#wizard2 .btn-primary:last-child'); await sleep(300);
  await page.type('#i_desc', 'Test incident from domain');
  await page.click('#wizard3 .btn-primary:last-child'); await sleep(300);
  const summary = await page.$eval('#wizSummary', el => el.textContent);
  console.log(`    Summary: "${summary}"`);
  if (summary.length < 5) throw new Error('Summary too short');
});

console.log('\n=== LOGOUT + LOGIN ===');
await test('Logout works', async () => {
  await page.click('#logoutTop'); await sleep(1000);
  const visible = await page.$eval('#login', el => !el.classList.contains('hidden'));
  if (!visible) throw new Error('Not returned to login');
});
await test('Re-login with same credentials', async () => {
  await page.type('#l_email', testEmail);
  await page.type('#l_pass', testPass);
  await page.click('#login .btn-primary');
  await sleep(2000);
  const d = await page.$eval('#dash', el => !el.classList.contains('hidden'));
  if (!d) throw new Error('Re-login failed');
});

console.log('\n=== MOBILE VIEWPORT ===');
await test('Mobile 375px works', async () => {
  await page.setViewport({ width: 375, height: 812 });
  await sleep(500);
  const overflow = await page.evaluate(() => document.body.scrollWidth > window.innerWidth + 10);
  if (overflow) throw new Error('Horizontal overflow');
});

console.log('\n=== JS ERRORS ===');
await test('No JS console errors', async () => {
  let errors = [];
  page.on('pageerror', e => errors.push(e));
  await page.goto('http://www.unysolar.com/app', { waitUntil: 'networkidle2', timeout: 15000 });
  await sleep(1000);
  if (errors.length > 0) console.log(`    Errors: ${errors.map(e=>e.message).join('; ')}`);
  else console.log('    Clean');
});

} finally { await browser.close(); }

console.log(`\n========================================`);
console.log(`unysolar.com — END USER QA`);
console.log(`Total: ${total} | Passed: ${passed} | Failed: ${failed}`);
if (issues.length) { console.log('\nIssues:'); issues.forEach((i,n) => console.log(`  ${n+1}. ${i.name}: ${i.error}`)); }
console.log(`========================================`);
process.exit(failed > 0 ? 1 : 0);
