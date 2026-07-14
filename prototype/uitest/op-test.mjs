// Operator E2E test - see all case details
import puppeteer from 'puppeteer-core';
const CHROME = '/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';
const DOMAIN = 'https://unysolar.com';
const OPS = DOMAIN + '/ops-console-7f3a9c';
const sleep = ms => new Promise(r=>setTimeout(r,ms));

const b = await puppeteer.launch({
  executablePath: CHROME, headless: false,
  args: ['--no-sandbox','--disable-setuid-sandbox','--disable-dev-shm-usage','--window-size=1400,900','--ignore-certificate-errors']
});
const p = await b.newPage();
await p.setViewport({width:1400,height:900});
let s = 1;
async function snap(n) { await p.screenshot({path:'/tmp/op-'+(s++)+'-'+n+'.png'}); }

// LOGIN
console.log('=== 1. Operator Login ===');
await p.goto(OPS, {waitUntil:'networkidle2',timeout:15000});
await sleep(1500);

// Login via API first (gets session cookie), then reload
await p.evaluate(async () => {
  await fetch('/api/agent/login', {
    method: 'POST',
    headers: {'Content-Type':'application/json'},
    body: JSON.stringify({agent_id:'OP-1001',password:'Operator#2026'}),
    credentials: 'include'
  });
});
await sleep(500);
await p.goto(OPS, {waitUntil:'networkidle2',timeout:15000});
await sleep(2000);
await snap('op-console');
console.log('Title:', await p.title());

// Check visible state
const state = await p.evaluate(() => {
  const overlay = document.getElementById('loginOverlay');
  const queue = document.getElementById('queue');
  return {
    overlayHidden: overlay ? overlay.classList.contains('hidden') : 'no-overlay',
    queueItems: document.querySelectorAll('.case-item, .queue-row, [data-case-id]').length,
    queueText: queue ? queue.innerText?.substring(0,500) : 'none'
  };
});
console.log('State:', JSON.stringify(state).substring(0,300));

// Dismiss login overlay if still visible
await p.evaluate(() => {
  const overlay = document.getElementById('loginOverlay');
  if (overlay && !overlay.classList.contains('hidden')) {
    overlay.classList.add('hidden');
  }
  const console = document.getElementById('console');
  if (console) console.classList.remove('hidden');
});
await sleep(500);

// ===== TRY TO CLICK A CASE =====
console.log('\n=== 2. Click on case in queue ===');
// Find and click the first case row
const clicked = await p.evaluate(() => {
  const rows = document.querySelectorAll('.case-item, .queue-row, [data-case-id], [onclick*="case"], tr[onclick]');
  if (rows.length > 0) {
    rows[0].click();
    return 'clicked ' + rows.length + ' rows found, clicked first';
  }
  // Try clicking any element with case number text
  const all = document.querySelectorAll('*');
  for (const el of all) {
    if (el.textContent?.includes('CASE-17840')) {
      el.click();
      return 'clicked case text element';
    }
  }
  return 'NO CASE ELEMENT FOUND - text: ' + document.body.innerText.substring(0,500);
});
console.log('Click result:', clicked);

await sleep(1500);
await snap('case-selected');

// ===== READ ALL DETAILS =====
console.log('\n=== 3. Check case detail panel ===');
const details = await p.evaluate(() => {
  const body = document.body.innerText;
  // Find all visible text
  const panels = document.querySelectorAll('.detail-panel, #caseDetail, .screen-pop, .case-detail, [class*="detail"], [class*="panel"], [class*="info"]');
  let panelText = '';
  panels.forEach(p => { panelText += p.innerText + '\n---\n'; });
  
  // Also get specific elements
  const map = document.querySelector('#map, .map-container, .leaflet-container');
  
  return {
    hasMap: !!map,
    mapSize: map ? map.offsetWidth + 'x' + map.offsetHeight : 'none',
    fullBody: body.substring(0, 1000),
    panels: panelText.substring(0, 800)
  };
});
console.log('Has map:', details.hasMap, details.mapSize);
console.log('Body text:', details.fullBody.substring(0, 400));
console.log('Panels:', details.panels.substring(0, 400));

// ===== SCREEN POP via incoming call =====
console.log('\n=== 4. Mock incoming call ===');
await p.evaluate(async () => {
  // Find and click answer call button
  const btns = document.querySelectorAll('button');
  for (const b of btns) {
    if (b.textContent?.includes('Answer') || b.textContent?.includes('incoming')) {
      b.click();
      break;
    }
  }
});
await sleep(1000);
await snap('incoming-call');

// Final screenshot
await snap('final-state');

console.log('\nDONE - browser stays open');
