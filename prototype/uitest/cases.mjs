import puppeteer from 'puppeteer-core';
const CHROME='/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';
const b=await puppeteer.launch({executablePath:CHROME,headless:'new',args:['--no-sandbox','--disable-gpu']});
async function shot(w,h,path){
  const p=await b.newPage();await p.setViewport({width:w,height:h});
  await p.goto('https://unysolar.com/app',{waitUntil:'networkidle0',timeout:30000});
  // login if needed
  const needLogin=await p.$('#login');
  if(needLogin){await p.click('button[onclick="login()"]');await p.waitForFunction(()=>document.getElementById('dash')&&!document.getElementById('dash').classList.contains('hidden'),{timeout:10000});}
  await p.waitForFunction(()=>document.querySelector('#cases .case-item')||/(No cases)/.test(document.getElementById('cases').textContent),{timeout:8000});
  const n=await p.$$eval('#cases .case-item',e=>e.length).catch(()=>0);
  // check overflow: does #cases scrollWidth exceed clientWidth?
  const ov=await p.$eval('#cases',e=>e.scrollWidth>e.clientWidth+2);
  console.log(`viewport ${w}x${h}: case cards=${n} horizontalOverflow=${ov}`);
  await p.screenshot({path});await p.close();
}
await shot(1200,850,'/tmp/insucar/design/cases-desktop.png');
await shot(390,800,'/tmp/insucar/design/cases-mobile.png');
await b.close();
