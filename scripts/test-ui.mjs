#!/usr/bin/env node
// Exercise the shipped single-file workbench in a real, offline browser.
// Browser/native engine parity is checked separately by test-demo.mjs.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { chromium } from 'playwright';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const artifact = path.resolve(process.argv[2] || path.join(root, 'dist/storepath-demo.html'));
const expectedPresets = {
  'telemetry-retention': 'hybrid',
  'transactional-db': 'block',
  'backup-repository': 'object',
  'analytics-range-reads': 'object',
  'hot-object-service': 'hybrid',
  'strict-latency': null,
};
const networkRequests = [];
const pageErrors = [];
const dialogs = [];
let browser;
let completed = 0;
let currentCase = 'browser startup';
let deadline;

async function test(name, action) {
  currentCase = name;
  await action();
  completed++;
  console.log(`PASS ${name}`);
}

async function run() {
  // STOREPATH_BROWSER_CHANNEL=chrome uses an already installed local Chrome.
  // CI leaves it unset and installs Playwright's matching Chromium revision.
  browser = await chromium.launch({
    headless: true,
    timeout: 15000,
    ...(process.env.STOREPATH_BROWSER_CHANNEL ? { channel: process.env.STOREPATH_BROWSER_CHANNEL } : {}),
  });
  const context = await browser.newContext({
    offline: true,
    acceptDownloads: true,
    viewport: { width: 1440, height: 1100 },
    reducedMotion: 'reduce',
  });
  context.on('request', (request) => {
    if (/^(https?|wss?):/i.test(request.url())) networkRequests.push(request.url());
  });
  const page = await context.newPage();
  page.setDefaultTimeout(5000);
  page.on('pageerror', (error) => pageErrors.push(error.message));
  page.on('dialog', async (dialog) => {
    dialogs.push(dialog.message());
    await dialog.dismiss();
  });
  await page.goto(pathToFileURL(artifact).href, { waitUntil: 'load', timeout: 15000 });
  await page.waitForFunction(() => typeof window.storepathEvaluate === 'function'
    && !document.querySelector('#quick-controls').disabled
    && document.querySelector('#result-output').getAttribute('aria-busy') === 'false');

  const output = page.locator('#result-output');
  const exports = ['#export-input', '#export-report'];
  async function exportEnabled(enabled) {
    for (const selector of exports) {
      assert.equal(await page.locator(selector).isEnabled(), enabled, `${selector} enabled=${enabled}`);
    }
  }
  async function decision(mode, status = mode ? 'modeled' : 'benchmark_required') {
    await page.waitForFunction(({ mode, status }) => {
      const result = document.querySelector('#result-output');
      return result.dataset.status === status && result.dataset.recommendation === (mode || '');
    }, { mode, status });
    assert.equal(await page.locator('#result-error').isVisible(), false, 'valid input has no error');
    assert.equal(await output.locator('.candidate').count(), 3, 'all three plans are displayed');
    assert.equal(await output.locator('.candidate.selected').count(), mode ? 1 : 0, 'only a recommended plan is selected');
    if (mode) {
      assert.equal(await output.locator('.candidate.selected').getAttribute('data-mode'), mode);
      assert.match(await output.locator('.recommendation h4').innerText(), new RegExp(mode, 'i'), 'visible recommendation matches selected plan');
    } else {
      assert.equal(await output.locator('.recommendation.needs-evidence').count(), 1, 'missing evidence is visible');
    }
    await exportEnabled(true);
  }
  async function preset(key) {
    await page.locator(`[data-preset="${key}"]`).click();
    assert.equal(await page.locator(`[data-preset="${key}"]`).getAttribute('aria-pressed'), 'true');
    await decision(expectedPresets[key]);
  }
  async function reveal(selector) {
    const control = page.locator(selector);
    const details = control.locator('xpath=ancestor::details[1]');
    if (await details.count() && (await details.getAttribute('open')) === null) {
      await details.locator(':scope > summary').click();
    }
    return control;
  }
  async function readDownload(selector) {
    const downloadPending = page.waitForEvent('download');
    await page.locator(selector).click();
    const download = await downloadPending;
    assert.equal(await download.failure(), null, 'export succeeds');
    assert.match(download.suggestedFilename(), /^blockorbucket-[a-z0-9-]+\.json$/);
    const downloadedPath = await download.path();
    assert.ok(downloadedPath, 'browser provided an actual downloaded file');
    return JSON.parse(await readFile(downloadedPath, 'utf8'));
  }
  async function clearedDecision() {
    assert.equal(await output.locator('.candidate, .recommendation').count(), 0, 'stale decision is removed');
    assert.equal(await output.getAttribute('data-recommendation'), null, 'stale recommendation metadata is removed');
    assert.equal(await output.getAttribute('data-status'), null, 'stale status metadata is removed');
    assert.equal(await page.locator('#result-announcement').textContent(),
      await output.locator('.empty-state').evaluate((element) => element.firstChild.textContent),
      'screen-reader summary matches the current empty state instead of retaining a stale decision');
    await exportEnabled(false);
  }

  await test('offline startup displays telemetry hybrid and engine costs', async () => {
    await decision('hybrid');
    const expected = await page.evaluate(() => JSON.parse(window.storepathEvaluate(
      JSON.stringify(window.STOREPATH_PRESETS['telemetry-retention']),
    )).decision);
    for (const candidate of expected.candidates.filter((item) => item.cost)) {
      const formatted = new Intl.NumberFormat('en-US', {
        style: 'currency', currency: candidate.cost.currency,
      }).format(candidate.cost.monthly_total);
      assert.ok((await output.locator(`[data-mode="${candidate.mode}"] .candidate-price`).innerText()).includes(formatted),
        `${candidate.mode} displays the engine's monthly total`);
    }
    assert.equal(await page.locator('#presets [data-preset]').count(), 6);
  });

  for (const key of Object.keys(expectedPresets)) {
    await test(`preset ${key}`, () => preset(key));
  }

  await test('editing an object rate changes the real recommendation', async () => {
    await preset('backup-repository');
    await (await reveal('[data-path="rates.object_gib_month"]')).fill('1');
    await decision('block');
    const current = JSON.parse(await page.locator('#input-json').inputValue());
    assert.equal(current.rates.object_gib_month, 1, 'edited rate is in the reusable input');
  });

  await test('a zero object price cannot override database semantics', async () => {
    await preset('transactional-db');
    await (await reveal('[data-path="rates.object_gib_month"]')).fill('0');
    await decision('block');
    assert.equal(await output.locator('[data-mode="object"].unavailable').count(), 1);
  });

  await test('a latency target waits for evidence and accepts a passing measurement', async () => {
    await preset('strict-latency');
    await decision(null);
    await (await reveal('#block-p99')).fill('2');
    await decision('block', 'latency_evidence_satisfied');
    assert.match(await output.locator('[data-mode="block"] .candidate-latency').innerText(), /2 ms p99/);
  });

  await test('invalid quick input clears stale results and disables exports', async () => {
    await preset('telemetry-retention');
    await page.locator('#immutable-gib').fill('-1');
    assert.equal(await page.locator('#result-error').isVisible(), true);
    await clearedDecision();
    await page.locator('#immutable-gib').fill('20000');
    await decision('hybrid');
  });

  await test('incomplete numeric latency input cannot silently remove an evidence requirement', async () => {
    for (const selector of ['#target-p99', '#block-p99']) {
      await preset('strict-latency');
      const control = await reveal(selector);
      await control.focus();
      await control.press('ControlOrMeta+A');
      // Real typing matters: browsers retain an incomplete exponent as badInput
      // while exposing an empty value, distinct from intentionally clearing it.
      await control.pressSequentially('e');
      assert.equal(await control.evaluate((element) => element.validity.badInput), true,
        `${selector} represents an incomplete number`);
      assert.equal(await page.locator('#result-error').isVisible(), true);
      await clearedDecision();
      const unchanged = JSON.parse(await page.locator('#input-json').inputValue());
      assert.equal(unchanged.workload.target_p99_ms, 5, 'invalid typing never drops the existing target');
      await control.fill(selector === '#target-p99' ? '5' : '2');
      await decision(selector === '#target-p99' ? null : 'block',
        selector === '#target-p99' ? 'benchmark_required' : 'latency_evidence_satisfied');
    }
  });

  await test('raw JSON duplicate keys reach the strict decoder unchanged', async () => {
    await preset('backup-repository');
    const json = await reveal('#input-json');
    const duplicate = (await json.inputValue()).replace(/"schema_version"\s*:\s*1/, '"schema_version": 1, "schema_version": 1');
    await json.fill(duplicate);
    await page.locator('#apply-json').click();
    assert.equal(await page.locator('#result-error').isVisible(), true);
    assert.match(await page.locator('#result-error').innerText(), /duplicate/i);
    await clearedDecision();
    await page.locator('#reset-input').click();
    await decision('object');
  });

  await test('pending JSON cannot export a decision for earlier assumptions', async () => {
    await preset('backup-repository');
    const json = await reveal('#input-json');
    const edited = JSON.parse(await json.inputValue());
    edited.rates.object_gib_month = 1;
    await json.fill(JSON.stringify(edited, null, 2));
    await clearedDecision();
    assert.equal(await page.locator('#workload-name').isDisabled(), true, 'quick controls cannot overwrite pending JSON');
    await page.locator('#apply-json').click();
    await decision('block');
    assert.equal(await page.locator('#workload-name').isEnabled(), true);
  });

  await test('accepted case-insensitive JSON keys populate canonical controls', async () => {
    await preset('strict-latency');
    const json = await reveal('#input-json');
    const edited = JSON.parse(await json.inputValue());
    edited.workload.TARGET_P99_MS = edited.workload.target_p99_ms;
    delete edited.workload.target_p99_ms;
    await json.fill(JSON.stringify(edited, null, 2));
    await page.locator('#apply-json').click();
    await decision(null);
    assert.equal(await page.locator('#target-p99').inputValue(), '5', 'accepted target remains visible');
    const canonical = JSON.parse(await json.inputValue());
    assert.equal(canonical.workload.target_p99_ms, 5);
    assert.equal(canonical.workload.TARGET_P99_MS, undefined, 'editor uses the canonical decoded key');
    const exported = await readDownload('#export-report');
    assert.equal(exported.input.workload.target_p99_ms, 5, 'export retains the effective target');
    assert.equal(exported.decision.status, 'benchmark_required', 'export preserves the evidence gate');
    // Leave a changed-rate scenario selected for the export regression below.
    await preset('backup-repository');
    await (await reveal('[data-path="rates.object_gib_month"]')).fill('1');
    await decision('block');
  });

  await test('actual exported files retain current input, decision, and assumptions', async () => {
    const expectedInput = JSON.parse(await page.locator('#input-json').inputValue());
    const exportedInput = await readDownload('#export-input');
    assert.deepEqual(exportedInput, expectedInput);
    const report = await readDownload('#export-report');
    assert.equal(report.report_schema_version, 1);
    assert.equal(report.product, 'BlockOrBucket');
    assert.equal(report.version, '0.1.0');
    assert.equal(report.core_engine, 'Storepath');
    assert.equal(report.core_engine_version, '0.2.0');
    assert.deepEqual(report.input, expectedInput);
    assert.equal(report.input.rates.object_gib_month, 1, 'export includes the edited rate');
    assert.equal(report.decision.recommendation, 'block');
    assert.equal(report.decision.status, 'modeled');
    assert.ok(report.decision.assumptions.length > 0);
    assert.ok(report.decision.exclusions.length > 0);
    assert.match(report.report_note, /not realized savings/);
    assert.ok(Number.isFinite(Date.parse(report.generated_at)), 'report timestamp is parseable');
    const expectedDecision = await page.evaluate((raw) => JSON.parse(window.storepathEvaluate(raw)).decision,
      JSON.stringify(expectedInput));
    assert.deepEqual(report.decision, expectedDecision, 'report exports the real engine decision');
  });

  await test('HTML-looking workload labels stay data in the workbench and export', async () => {
    const malicious = '<img src=x onerror="window.__storepath_xss=1"> & workload';
    const json = await reveal('#input-json');
    const input = JSON.parse(await json.inputValue());
    input.workload.name = malicious;
    input.rates.label = malicious;
    await json.fill(JSON.stringify(input, null, 2));
    await page.locator('#apply-json').click();
    await decision('block');
    assert.equal(await page.locator('#workload-name').inputValue(), malicious);
    assert.ok((await page.locator('.rate-meta').innerText()).includes(malicious));
    assert.equal(await page.locator('#workbench img').count(), 0, 'no label becomes an image element');
    assert.equal(await page.evaluate(() => window.__storepath_xss), undefined);
    const report = await readDownload('#export-report');
    assert.equal(report.input.workload.name, malicious);
    assert.equal(report.decision.workload, malicious);
  });

  await test('desktop and 390px mobile layouts avoid horizontal document overflow', async () => {
    await preset('telemetry-retention');
    for (const viewport of [{ width: 1440, height: 1100 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(viewport);
      // Open the long rate fields and JSON editor as well as the cost table.
      await reveal('[data-path="rates.object_gib_month"]');
      await reveal('#input-json');
      const dimensions = await page.evaluate(() => ({
        viewport: document.documentElement.clientWidth,
        document: document.documentElement.scrollWidth,
        body: document.body.scrollWidth,
      }));
      assert.ok(dimensions.document <= dimensions.viewport + 1 && dimensions.body <= dimensions.viewport + 1,
        `no document overflow at ${viewport.width}px: ${JSON.stringify(dimensions)}`);
      assert.equal(await page.locator('#export-report').isEnabled(), true, 'mobile retains export actions');
    }
  });

  await test('the complete interaction session remains offline and error-free', async () => {
    assert.deepEqual(networkRequests, [], 'workbench makes no network requests');
    assert.deepEqual(pageErrors, [], 'browser JavaScript has no uncaught errors');
    assert.deepEqual(dialogs, [], 'input data never executes dialog-producing scripts');
  });
}

try {
  await Promise.race([
    run(),
    new Promise((_, reject) => {
      deadline = setTimeout(() => reject(new Error(`60-second UI test deadline exceeded during: ${currentCase}`)), 60000);
    }),
  ]);
  console.log(`Offline product UI passed: ${completed} cases using ${process.env.STOREPATH_BROWSER_CHANNEL || 'bundled Chromium'}.`);
} catch (error) {
  console.error(`FAIL ${currentCase}: ${error.stack || error}`);
  process.exitCode = 1;
} finally {
  clearTimeout(deadline);
  if (browser) await browser.close();
}
