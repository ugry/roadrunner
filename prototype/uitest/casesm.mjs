import puppeteer from 'puppeteer-core';
const CHROME='/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';
const b=await puppeteer.launch({executablePath:CHROME,headless:'new',args:['--no-sandbox','--disable-gpu']});
const p=await b.newPage();await p.setViewport({width:390,height:820,isMobile:true});
await p.goto('https://unysolar.com/app',{waitUntil:'networkidle0',timeout:30000});
const dashHidden=await p.$eval('#dash',e=>e.classList.contains('hidden')).catch(()=>true);
if(dashHidden){await p.click('button[onclick="login()"]');await p.waitForFunction(()=>!document.getElementById('dash').classList.contains('hidden'),{timeout:10000});}
await p.waitForFunction(()=>document.querySelector('#cases .case-item'),{timeout:8000});
const n=await p.$$eval('#cases .case-item',e=>e.length);
const ov=await p.$eval('#cases',e=>e.scrollWidth>e.clientWidth+2);
const bodyOv=await p.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth+2);
console.log(`mobile 390: cards=${n} casesOverflow=${ov} pageOverflow=${bodyOv}`);
await p.screenshot({path:'/tmp/insucar/design/cases-mobile.png',fullPage:true});
await b.close();
