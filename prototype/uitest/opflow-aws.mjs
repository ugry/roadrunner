import puppeteer from 'puppeteer-core';

const CHROME = '/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';
const BASE = 'http://108.129.149.127:8080';
const OPS = BASE + '/ops-console-7f3a9c';

const browser = await puppeteer.launch({ executablePath: CHROME, headless: 'new',
  args: ['--no-sandbox','--disable-gpu','--window-size=1280,900'] });
const page = await browser.newPage();
await page.setViewport({ width: 1280, height: 900 });

const fail = (m) => { console.error('FAIL: ' + m); process.exitCode = 1; };

await page.goto(OPS, { waitUntil: 'networkidle0' });

// 1) answer the incoming call -> triggers real /api/lookup + /api/cases
await page.click('button[onclick="incoming()"]');
await page.waitForFunction(() => document.getElementById('cust').textContent !== '—', { timeout: 8000 });
const cust = await page.$eval('#cust', el => el.textContent);
const pol  = await page.$eval('#pol',  el => el.textContent);
const veh  = await page.$eval('#veh',  el => el.textContent);
console.log('screen-pop customer :', cust);
console.log('screen-pop policy   :', pol);
console.log('screen-pop vehicle  :', veh);
if (!cust.includes('Claire')) fail('customer not populated');
if (!pol.includes('INS-FR-1001')) fail('policy not populated');

// wait for dispatch button to enable (case created), then dispatch
await page.waitForFunction(() => !document.getElementById('dispatchBtn').disabled, { timeout: 8000 });
await page.click('#dispatchBtn');
await page.waitForFunction(() => document.getElementById('eta').textContent !== '—', { timeout: 8000 });
const prov = await page.$eval('#prov', el => el.textContent);
const eta  = await page.$eval('#eta',  el => el.textContent);
const drv  = await page.$eval('#driver', el => el.textContent);
console.log('dispatch provider   :', prov);
console.log('dispatch driver     :', drv);
console.log('dispatch ETA (min)  :', eta);
if (!prov.includes('AXA')) fail('provider not set');
if (!(parseInt(eta) > 0)) fail('ETA not set');

await page.screenshot({ path: '/tmp/insucar/prototype/operator-live.png', fullPage: true });
console.log('screenshot: operator-live.png');
await browser.close();
console.log(process.exitCode ? 'UI TEST: FAILED' : 'UI TEST: PASSED');
