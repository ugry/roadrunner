// Full E2E test with visible browser + screenshots
import puppeteer from 'puppeteer-core';

const CHROME = '/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';
const DOMAIN = 'https://unysolar.com';
const OPS = DOMAIN + '/ops-console-7f3a9c';

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: false,
  args: ['--no-sandbox','--disable-setuid-sandbox','--disable-dev-shm-usage','--window-size=1400,900','--ignore-certificate-errors']
});

const page = await browser.newPage();
await page.setViewport({width:1400,height:900});
const sleep = ms => new Promise(r=>setTimeout(r,ms));
let step = 1;

async function snap(name) {
  const f = '/tmp/e2e-' + (step++) + '-' + name + '.png';
  await page.screenshot({path:f, fullPage:false});
  console.log('  ' + f);
}

// ===== STEP 1: LOAD LOGIN PAGE =====
console.log('\n=== STEP 1: Open login page ===');
await page.goto(DOMAIN + '/app', {waitUntil:'networkidle2',timeout:15000});
await sleep(1000);
await snap('login-page');
console.log('Title:', await page.title());

// ===== STEP 2: LOGIN =====
console.log('\n=== STEP 2: Login as ugur.yardimci@unygms.com ===');
await page.type('#l_email', 'ugur.yardimci@unygms.com');
await page.type('#l_pass', 'UgurTest2026!');
await snap('login-filled');
await page.click('#login .btn-primary');
await sleep(2000);

// Check if dashboard appeared
const dashVisible = await page.evaluate(() => {
  const d = document.getElementById('dash');
  return d && !d.classList.contains('hidden');
});
console.log('Dashboard visible:', dashVisible);
if (!dashVisible) {
  const err = await page.$eval('#out', e=>e.textContent).catch(()=>'');
  console.log('Login error:', err);
}

// ===== STEP 3: FILL INCIDENT WIZARD =====
console.log('\n=== STEP 3: Fill incident wizard ===');
await snap('dashboard');

// Step 1: Select incident type
await page.select('#i_type', 'breakdown');
await snap('wizard-step1');
await page.click('#wizard1 .btn-primary');
await sleep(400);

// Step 2: Enter address
await page.type('#i_addr', 'A6 highway, km 25, near Paris');
await snap('wizard-step2');
await page.click('#wizard2 .btn-primary:last-child');
await sleep(400);

// Step 3: Describe
await page.type('#i_desc', 'Engine stopped on highway, car wont restart. I am on the emergency lane.');
await snap('wizard-step3');
await page.click('#wizard3 .btn-primary:last-child');
await sleep(400);

// Step 4: Confirm - show summary
const summary = await page.$eval('#wizSummary', e=>e.textContent);
console.log('Summary:', summary);
await snap('wizard-step4-summary');

// ===== STEP 4: SUBMIT INCIDENT (press and hold) =====
console.log('\n=== STEP 4: Submit incident (press & hold) ===');
// Click and hold the button
const holdBtn = await page.$('#holdBtn');
const box = await holdBtn.boundingBox();
await page.mouse.move(box.x + box.width/2, box.y + box.height/2);
await page.mouse.down();
// Hold for 3 seconds
for (let i = 0; i < 6; i++) {
  await sleep(500);
  const width = await page.$eval('#holdProgress', e=>e.style.width);
  console.log('  Hold progress:', width);
}
await page.mouse.up();
await sleep(1500);

// Check for success/error banner
const banner = await page.evaluate(() => {
  const b = document.querySelector('#out');
  return b ? b.textContent : '';
});
console.log('Result banner:', banner);

// Check if case appeared in My Cases
await sleep(1000);
const casesVisible = await page.evaluate(() => {
  const c = document.getElementById('cases');
  return c ? c.innerHTML.substring(0,200) : '';
});
console.log('Cases section:', casesVisible);
await snap('incident-result');

// ===== STEP 5: VERIFY CASE IN DB =====
console.log('\n=== STEP 5: Check DB ===');
// We'll check via curl separately

// ===== STEP 6: OPERATOR LOGIN =====
console.log('\n=== STEP 6: Open operator console ===');
const opPage = await browser.newPage();
await opPage.setViewport({width:1400,height:900});
await opPage.goto(OPS, {waitUntil:'networkidle2',timeout:15000});
await sleep(1000);
await snap('operator-login-page');
console.log('Op title:', await opPage.title());

// Login as operator
await opPage.type('#l_agent', 'OP-1001');
await opPage.type('#l_pass', 'Operator#2026');
await snap('operator-login-filled');
await opPage.click('#login .btn-primary');
await sleep(2000);

// Check queue
const queue = await opPage.evaluate(() => {
  const q = document.getElementById('queue');
  return q ? q.innerHTML.substring(0,300) : 'no queue';
});
console.log('Operator queue:', queue);
await snap('operator-queue');

// Clean up
await sleep(2000);
console.log('\n=== DONE ===');
