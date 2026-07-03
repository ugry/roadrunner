import puppeteer from 'puppeteer-core';
const CHROME='/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';
const B=process.env.BASE;
const b=await puppeteer.launch({executablePath:CHROME,headless:'new',args:['--no-sandbox','--disable-gpu','--window-size=1280,900']});
const fail=(m)=>{console.error('FAIL: '+m);process.exitCode=1;};

// ---- USER app ----
let p=await b.newPage(); await p.setViewport({width:520,height:900});
await p.goto(B+'/login',{waitUntil:'networkidle0'});
await p.click('button[onclick="login()"]');
await p.waitForFunction(()=>document.getElementById('dash') && !document.getElementById('dash').classList.contains('hidden'),{timeout:8000});
console.log('user signed in :', await p.$eval('#whoami',e=>e.textContent));
await p.select('#i_type','flat_tyre');
await p.type('#i_desc','Front-left tyre blown on the ring road');
await p.type('#i_addr','Paris périphérique, exit 12');
p.once('dialog',d=>d.accept());
await p.click('button[onclick="submitIncident()"]');
await p.waitForFunction(()=>document.querySelector('#cases table'),{timeout:8000});
const rows=await p.$$eval('#cases table tr',r=>r.length);
console.log('user cases rows (incl header):', rows);
if(rows<2) fail('user cases not listed');
await p.screenshot({path:'/tmp/insucar/prototype/enduser-auth.png',fullPage:true});

// ---- OPERATOR app ----
let o=await b.newPage(); await o.setViewport({width:1280,height:900});
await o.goto(B+'/ops-console-7f3a9c',{waitUntil:'networkidle0'});
await o.click('button[onclick="alogin()"]');
await o.waitForFunction(()=>document.getElementById('console') && !document.getElementById('console').classList.contains('hidden'),{timeout:8000});
console.log('agent signed in :', await o.$eval('#agent',e=>e.textContent));
await o.waitForFunction(()=>document.querySelector('#queue table'),{timeout:8000});
const q=await o.$$eval('#queue table tr',r=>r.length);
console.log('queue rows (incl header):', q);
if(q<2) fail('queue empty');
// open first case row
await o.click('#queue table tr:nth-child(2)');
await o.waitForFunction(()=>document.getElementById('c_no').textContent!=='—',{timeout:8000});
console.log('opened case :', await o.$eval('#c_no',e=>e.textContent), '| vehicle', await o.$eval('#c_veh',e=>e.textContent));
await o.click('#dispatchBtn');
await o.waitForFunction(()=>document.getElementById('eta').textContent!=='—',{timeout:8000});
console.log('dispatched provider', await o.$eval('#d_prov',e=>e.textContent),'ETA', await o.$eval('#eta',e=>e.textContent));
await o.screenshot({path:'/tmp/insucar/prototype/operator-auth.png',fullPage:true});

await b.close();
console.log(process.exitCode?'UI AUTH TEST: FAILED':'UI AUTH TEST: PASSED');
