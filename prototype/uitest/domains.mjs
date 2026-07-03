import puppeteer from 'puppeteer-core';
const CHROME='/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';
const b=await puppeteer.launch({executablePath:CHROME,headless:'new',args:['--no-sandbox','--disable-gpu']});
const fail=(m)=>{console.error('FAIL: '+m);process.exitCode=1;};
// USER on unysolar.com
let p=await b.newPage();await p.setViewport({width:1100,height:900});
await p.goto('http://unysolar.com/',{waitUntil:'networkidle0',timeout:30000});
await p.click('#login button, button[onclick="login()"]');
await p.waitForFunction(()=>document.getElementById('dash')&&document.getElementById('dash').style.display!=='none',{timeout:10000});
console.log('USER signed in:', await p.$eval('#whoami',e=>e.textContent));
await p.screenshot({path:'/tmp/insucar/design/live-user-unysolar.png',fullPage:true});
// OPERATOR on op.unysolar.com
let o=await b.newPage();await o.setViewport({width:1600,height:1000});
await o.goto('http://op.unysolar.com/',{waitUntil:'networkidle0',timeout:30000});
await o.click('button[onclick="alogin()"]');
await o.waitForFunction(()=>document.getElementById('loginOverlay').style.display==='none',{timeout:10000});
console.log('OPERATOR signed in:', await o.$eval('#a_name',e=>e.textContent));
await o.waitForFunction(()=>document.querySelector('#queue table'),{timeout:10000});
await o.click('#queue table tr:nth-child(2)');
await o.waitForFunction(()=>document.getElementById('c_no').textContent!=='no case selected',{timeout:10000});
console.log('OPERATOR opened case:', await o.$eval('#c_no',e=>e.textContent),'| cust', await o.$eval('#c_cust',e=>e.textContent));
await o.click('#dispatchBtn');
await o.waitForFunction(()=>document.getElementById('m_eta').textContent!=='--',{timeout:10000});
console.log('OPERATOR dispatched ETA:', await o.$eval('#m_eta',e=>e.textContent),'provider', await o.$eval('#m_prov',e=>e.textContent));
await o.screenshot({path:'/tmp/insucar/design/live-operator-op.png',fullPage:true});
await b.close();
console.log(process.exitCode?'DOMAIN E2E: FAILED':'DOMAIN E2E: PASSED');
