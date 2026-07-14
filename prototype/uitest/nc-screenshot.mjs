// Take screenshot of Namecheap domain NS settings
import puppeteer from 'puppeteer-core';

const CHROME = '/home/semyaza/.cache/puppeteer/chrome/linux-131.0.6778.264/chrome-linux64/chrome';

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: 'new',
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--window-size=1400,900']
});

try {
  const page = await browser.newPage();
  await page.setViewport({ width: 1400, height: 900 });
  page.setDefaultTimeout(15000);

  console.log('1. Going to Namecheap login...');
  await page.goto('https://www.namecheap.com/myaccount/login/', { waitUntil: 'domcontentloaded', timeout: 30000 });
  await new Promise(r => setTimeout(r, 2000));
  await page.screenshot({ path: '/tmp/nc1-login.png', fullPage: false });
  console.log('   Screenshot: /tmp/nc1-login.png');

  // Fill credentials
  const inputs = await page.$$('input');
  console.log(`   Found ${inputs.length} inputs`);
  for (const inp of inputs) {
    const name = await inp.evaluate(el => el.name || el.id || el.placeholder || el.type);
    console.log(`   Input: ${name}`);
  }

  // Try to fill login form
  const userField = await page.$('input[name="LoginUserName"]');
  if (!userField) {
    // Try alternative selectors
    const allInputs = await page.$$('input[type="text"], input[type="email"], input:not([type])');
    for (const inp of allInputs) {
      const val = await inp.evaluate(el => el.name + '|' + el.id + '|' + el.placeholder);
      console.log(`   Alt input: ${val}`);
    }
  }

  if (userField) {
    await userField.type('uguryardimci82');
    console.log('   Username filled');
  }

  const passField = await page.$('input[name="LoginPassword"]');
  if (passField) {
    await passField.type('m@ssw0rd1982!');
    console.log('   Password filled');
  }

  // Try submit
  const submitBtn = await page.$('button[type="submit"], input[type="submit"]');
  if (submitBtn) {
    await submitBtn.click();
    console.log('   Submit clicked');
    await new Promise(r => setTimeout(r, 5000));
    await page.screenshot({ path: '/tmp/nc2-after-login.png', fullPage: false });
    console.log('   Screenshot: /tmp/nc2-after-login.png');
  }

  // Check current URL
  const url = page.url();
  console.log(`   Current URL: ${url}`);

  // If logged in, go to domain management
  if (url.includes('account') || url.includes('dashboard') || url.includes('domain')) {
    console.log('2. Navigating to domain list...');
    await page.goto('https://ap.www.namecheap.com/Domains/DomainList', { waitUntil: 'domcontentloaded', timeout: 30000 });
    await new Promise(r => setTimeout(r, 3000));
    await page.screenshot({ path: '/tmp/nc3-domain-list.png', fullPage: true });
    console.log('   Screenshot: /tmp/nc3-domain-list.png');

    // Look for unysolar.com
    const pageText = await page.$eval('body', el => el.textContent?.substring(0, 1000) || '');
    console.log(`   Page contains unysolar: ${pageText.includes('unysolar')}`);
  }

  // Final screenshot
  await page.screenshot({ path: '/tmp/nc-final.png', fullPage: true });
  console.log('Done. Check /tmp/nc-*.png');

} catch (e) {
  console.log(`Error: ${e.message}`);
  await page.screenshot({ path: '/tmp/nc-error.png', fullPage: true });
} finally {
  await browser.close();
}
