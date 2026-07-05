# QA Visual Test Directives — End-User Experience Testing

**Created after:** Critical visual bug missed twice by headless browser tests  
**Bug:** Registration page "Account created!" success card was always visible alongside the form card  
**Root cause:** `.hidden { display: none }` CSS rule was missing from `register.html`  
**Why missed:** QA tests checked DOM element existence, not visual rendering state  

---

## THE FUNDAMENTAL MISTAKE

We tested what robots see (DOM elements), not what humans see (rendered pixels).

A stranded motorist on the A6 at 8 PM doesn't inspect the DOM. They look at their screen. If they see both "Create Your Account" AND "Account created! Welcome to Insucar" simultaneously, the experience is broken — regardless of whether the API works, the database works, or the elements exist in the DOM.

### Wrong test pattern (what we did):

```javascript
// BAD: checks only DOM existence
const first = await page.$('#r_first');
if (!first) throw new Error('Missing field');
// This passes even if the element is display:none, opacity:0,
// behind another element, off-screen, or zero-sized.
```

### Correct test pattern (what we must do):

```javascript
// GOOD: checks visual rendering state
const isVisible = await page.$eval('#r_first', el => {
  const style = window.getComputedStyle(el);
  return style.display !== 'none' 
    && style.visibility !== 'hidden' 
    && style.opacity !== '0'
    && el.offsetWidth > 0 
    && el.offsetHeight > 0;
});
if (!isVisible) throw new Error('Field exists in DOM but is NOT visible');
```

---

## MANDATORY VISUAL TESTS — EVERY QA CYCLE

These tests must run on every page, every deployment. No exceptions.

### 1. STATE ISOLATION TEST

**Purpose:** Verify that only the CURRENT state's UI is visible. Previous/next states must be hidden.

**Pattern:** For every page with multiple states (login vs logged-in, form vs success, loading vs loaded), verify exactly ONE state is visible.

```javascript
async function testStateIsolation(page, states) {
  // states = { form: '#formCard', success: '#successCard', error: '#errorCard' }
  let visibleCount = 0;
  for (const [name, selector] of Object.entries(states)) {
    const el = await page.$(selector);
    if (!el) continue;
    const visible = await el.evaluate(el => {
      const s = window.getComputedStyle(el);
      return s.display !== 'none' && s.visibility !== 'hidden' && el.offsetHeight > 0;
    });
    if (visible) {
      visibleCount++;
      console.log(`  VISIBLE: ${name} (${selector})`);
    }
  }
  if (visibleCount !== 1) {
    throw new Error(`STATE ISOLATION FAILED: ${visibleCount} states visible, expected 1`);
  }
}
```

**Pages requiring this test:**

| Page | States | Selectors |
|---|---|---|
| `/app` (enduser.html) | auth form, dashboard, tracking card | `#authcard`, `#dash`, `#trackingCard` |
| `/register-page` | form, success | `#formCard`, `#successCard` |
| Operator login | login overlay, console | `#loginOverlay`, `#console` |
| `/api/status/:token` | loading, tracking, not-found | `.loading` text, `#app` cards, error state |

### 2. Z-INDEX / OVERLAP TEST

**Purpose:** Verify no two interactive elements overlap. A user must be able to click/tap every button without hitting a different element.

**Pattern:** Check for overlapping bounding boxes on all clickable elements.

```javascript
async function testNoOverlaps(page) {
  const overlaps = await page.evaluate(() => {
    const buttons = document.querySelectorAll('button, a, input, select, [onclick]');
    const rects = [];
    const issues = [];
    for (const b of buttons) {
      const r = b.getBoundingClientRect();
      if (r.width === 0 || r.height === 0) continue;
      for (const prev of rects) {
        if (r.left < prev.right && r.right > prev.left && 
            r.top < prev.bottom && r.bottom > prev.top) {
          issues.push(`${b.tagName} "${b.textContent?.slice(0,30)}" overlaps ${prev.tag}`);
        }
      }
      rects.push({left:r.left, right:r.right, top:r.top, bottom:r.bottom, tag:b.tagName});
    }
    return issues;
  });
  if (overlaps.length > 0) throw new Error(`OVERLAP: ${overlaps.join('; ')}`);
}
```

### 3. CONTENT LEAK TEST

**Purpose:** Verify that data/UI from one state doesn't "leak" into another state. A logged-out user must never see logged-in content. A pre-registration page must never show post-registration content.

**Pattern:** Check for forbidden strings in each state.

```javascript
async function testContentLeak(page, state, forbiddenStrings) {
  const text = await page.evaluate(() => document.body.innerText);
  for (const s of forbiddenStrings) {
    if (text.includes(s)) {
      throw new Error(`CONTENT LEAK: "${s}" visible in ${state} state`);
    }
  }
}

// Usage:
await testContentLeak(page, 'login', ['Account created!', 'Welcome back,', 'My Cases']);
await testContentLeak(page, 'logged-out', ['Sign out', 'Request assistance']);
await testContentLeak(page, 'registration-form', ['Account created!', 'Welcome']);
```

### 4. CSS CLASS EFFECTIVENESS TEST

**Purpose:** Verify that CSS classes used for show/hide actually have CSS rules that make them work.

**Pattern:** Check that `.hidden`, `.visible`, `.active`, `.show` classes are defined in CSS.

```javascript
async function testCssClassExists(page, className, expectedProperty, expectedValue) {
  const found = await page.evaluate((cls, prop, val) => {
    for (const sheet of document.styleSheets) {
      try {
        for (const rule of sheet.cssRules) {
          if (rule.selectorText?.includes('.' + cls)) {
            const actual = rule.style.getPropertyValue(prop);
            if (val && !actual.includes(val)) continue;
            return true;
          }
        }
      } catch (e) { /* cross-origin sheet */ }
    }
    // Also check inline styles
    const testEl = document.createElement('div');
    testEl.className = cls;
    document.body.appendChild(testEl);
    const computed = window.getComputedStyle(testEl);
    const result = computed.getPropertyValue(prop);
    document.body.removeChild(testEl);
    return val ? result.includes(val) : result !== '';
  }, className, expectedProperty, expectedValue);
  
  if (!found) {
    throw new Error(`CSS CLASS "${className}" has no rule setting ${expectedProperty}`);
  }
}

// Usage:
await testCssClassExists(page, 'hidden', 'display', 'none');
```

### 5. RESPONSIVE BREAKPOINT TEST

**Purpose:** Verify the page is usable at mobile, tablet, and desktop widths.

**Pattern:** Test at 3 viewport sizes.

```javascript
const BREAKPOINTS = [
  { name: 'mobile', width: 375, height: 812 },   // iPhone
  { name: 'tablet', width: 768, height: 1024 },   // iPad
  { name: 'desktop', width: 1440, height: 900 },
];

for (const bp of BREAKPOINTS) {
  await page.setViewport(bp);
  // Verify: no horizontal scrollbar
  const hasHScroll = await page.evaluate(() => 
    document.documentElement.scrollWidth > document.documentElement.clientWidth + 5
  );
  if (hasHScroll) throw new Error(`Horizontal scroll at ${bp.name}`);
  
  // Verify: all buttons are tappable (min 44x44px per WCAG)
  const smallTargets = await page.evaluate(() => {
    return [...document.querySelectorAll('button, a.btn, [role="button"]')]
      .filter(el => {
        const r = el.getBoundingClientRect();
        return r.width > 0 && r.height > 0 && (r.width < 44 || r.height < 44);
      })
      .map(el => el.textContent?.slice(0,20));
  });
  if (smallTargets.length > 0) {
    console.warn(`  Small touch targets at ${bp.name}: ${smallTargets.join(', ')}`);
  }
}
```

### 6. SCREENSHOT DIFF TEST

**Purpose:** Catch visual regressions that code tests miss — layout shifts, color changes, font issues.

**Pattern:** Compare screenshots against baseline.

```javascript
import { readFileSync } from 'fs';

async function testVisualRegressions(page, name) {
  const screenshot = await page.screenshot({ fullPage: true });
  const baselinePath = `/tmp/qa-baselines/${name}.png`;
  
  try {
    const baseline = readFileSync(baselinePath);
    // Compare using pixelmatch or similar
    // For now: at minimum, take the screenshot and log dimensions
    console.log(`  Screenshot: ${screenshot.length} bytes`);
    if (!baseline) {
      console.log(`  No baseline yet — saving as new baseline`);
      // writeFileSync(baselinePath, screenshot);
    }
  } catch (e) {
    console.log(`  No baseline for ${name} — first run`);
  }
}
```

---

## UPDATED QA CHECKLIST FOR EVERY PAGE

For every page in the application, these checks must pass:

### Pre-login pages (`/`, `/app`, `/register-page`)

| # | Check | Method |
|---|---|---|
| V1 | Only ONE state card visible | State Isolation Test |
| V2 | No "Account created!" text on registration form | Content Leak Test |
| V3 | No "Welcome back" on landing page | Content Leak Test |
| V4 | `.hidden` CSS class has `display:none` rule | CSS Class Effectiveness Test |
| V5 | All buttons are at least 44×44px | Responsive Breakpoint Test |
| V6 | No horizontal scroll at 375px width | Responsive Breakpoint Test |
| V7 | Login form shows BEFORE clicking any tab | State Isolation Test |
| V8 | Password field type="password" (masked) | DOM check |

### Post-login pages (dashboard, tracking)

| # | Check | Method |
|---|---|---|
| V9 | Auth card is hidden after login | State Isolation Test |
| V10 | Dashboard is visible after login | State Isolation Test |
| V11 | No elements from auth card leak into dashboard | Content Leak Test |
| V12 | Tracking card replaces form after incident submission | State Isolation Test |
| V13 | Incident form is hidden when tracking card shows | State Isolation Test |
| V14 | Logout restores login page (all states reset) | State Isolation Test |

### Operator console

| # | Check | Method |
|---|---|---|
| V15 | Login overlay covers entire console | Z-Index/Overlap Test |
| V16 | Console elements NOT clickable behind login overlay | Z-Index/Overlap Test |
| V17 | Login overlay hidden after successful login | State Isolation Test |
| V18 | No operator data leaks into the login overlay | Content Leak Test |
| V19 | Map panel renders without errors | Screenshot comparison |
| V20 | Queue table renders without overflow | Responsive Breakpoint Test |

---

## LESSONS LEARNED

### 1. DOM existence is not visual rendering
`page.$('#element')` only tells you the element exists in the DOM. It does NOT tell you:
- Is it visible? (`display:none`, `visibility:hidden`, `opacity:0`)
- Is it the right size? (`width:0`, `height:0`, `transform:scale(0)`)
- Is it behind another element? (z-index issues)
- Is it on-screen? (positioned off-viewport)
- Is text readable? (overflow:hidden, font-size:0, color matching background)

### 2. Every state transition must be tested visually
Just because clicking "Submit" triggers the right API call doesn't mean the success state looks correct. The transition from form → success must be verified with visual tests, not just API response checks.

### 3. CSS classes without CSS rules are invisible bugs
When HTML uses `class="hidden"` but no `.hidden { display: none }` exists in the stylesheet, the class has zero effect. The browser silently ignores unknown classes. There is no error, no warning, no console message. The only way to catch this is a visual test.

### 4. The headless browser test must simulate a human
A human sees the page. If two contradictory states are visible simultaneously, the human knows something is wrong immediately. The automated test must replicate this — checking computed styles, not just DOM selectors.

### 5. Minimum viable visual test: just ONE
If you can't implement all the tests above, implement this single test that would have caught the bug:

```javascript
// CATCHES: any CSS class that should hide something but doesn't
async function testHiddenClassWorks(page) {
  const works = await page.evaluate(() => {
    const div = document.createElement('div');
    div.className = 'hidden';
    div.textContent = 'TEST';
    document.body.appendChild(div);
    const isHidden = window.getComputedStyle(div).display === 'none';
    document.body.removeChild(div);
    return isHidden;
  });
  if (!works) {
    throw new Error('FATAL: .hidden CSS class has no display:none rule. Every page that uses class="hidden" is broken.');
  }
}
```

---

## IMPLEMENTATION

Add this to the QA test suite (`qa-full.mjs`) as the **first test** that runs, before any page-specific tests:

```javascript
// PRE-FLIGHT: Verify .hidden class actually hides elements
await test('CORE: .hidden CSS class works', async () => {
  const works = await page.evaluate(() => {
    const d = document.createElement('div');
    d.className = 'hidden'; d.textContent = 'test';
    document.body.appendChild(d);
    const hidden = window.getComputedStyle(d).display === 'none';
    document.body.removeChild(d);
    return hidden;
  });
  if (!works) throw new Error('.hidden class missing — ALL state transitions broken');
});
```

This 5-line test would have caught the registration page bug on the very first run.
