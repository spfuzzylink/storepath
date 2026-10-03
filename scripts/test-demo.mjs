#!/usr/bin/env node
// Test the actual release HTML's embedded Go engine against the native CLI.
// This intentionally needs no browser packages, server, cloud, or network.
import assert from 'node:assert/strict';
import { webcrypto } from 'node:crypto';
import { readFileSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawnSync } from 'node:child_process';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const artifact = path.resolve(process.argv[2] || path.join(root, 'dist/storepath-demo.html'));
const html = readFileSync(artifact, 'utf8');
assert.doesNotMatch(html, /<script\b[^>]*\bsrc\s*=/i, 'release HTML needs no external scripts');
assert.doesNotMatch(html, /<link\b[^>]*\brel\s*=\s*["']stylesheet["']/i,
  'release HTML embeds its stylesheet');
function script(id) {
  const matches = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script\s*>/gi)]
    .filter((match) => new RegExp(`\\bid=["']${id}["']`).test(match[1]));
  assert.equal(matches.length, 1, `one embedded ${id} script`);
  return matches[0][2];
}

globalThis.crypto ??= webcrypto;
globalThis.window = globalThis;
vm.runInThisContext(script('storepath-config'), { filename: 'embedded-config.js' });
vm.runInThisContext(script('storepath-runtime'), { filename: 'embedded-go-runtime.js' });
assert.equal(typeof Go, 'function', 'matching Go runtime is embedded');
const engine = Buffer.from(script('storepath-wasm').trim(), 'base64');
const go = new Go();
const { instance } = await WebAssembly.instantiate(engine, go.importObject);
const engineRun = go.run(instance);
assert.equal(typeof storepathEvaluate, 'function', 'real Go bridge is available');
const temp = mkdtempSync(path.join(tmpdir(), 'storepath-parity-'));
const binary = path.join(temp, process.platform === 'win32' ? 'storepath.exe' : 'storepath');
const deadline = setTimeout(() => {
  console.error('Browser engine parity test timed out');
  process.exit(1);
}, 30000);

function browser(raw) {
  const reply = JSON.parse(storepathEvaluate(raw));
  if (reply.error) throw new Error(reply.error);
  assert.ok(reply.decision, 'browser bridge supplies a decision');
  assert.ok(reply.input, 'browser bridge supplies the canonical input that was evaluated');
  return reply.decision;
}
function parity(input) {
  const raw = typeof input === 'string' ? input : JSON.stringify(input);
  const decision = browser(raw);
  const native = JSON.parse(execFileSync(binary, ['evaluate', '-'], { input: raw, encoding: 'utf8' }));
  assert.deepEqual(decision, native, 'browser decision matches native engine exactly');
  return decision;
}
function invalid(raw) {
  const reply = JSON.parse(storepathEvaluate(raw));
  assert.ok(reply.error, 'invalid browser input returns an error');
  assert.equal(reply.decision, undefined, 'invalid input never returns a stale decision');
  assert.equal(reply.input, undefined, 'invalid input never returns canonical data');
  const native = spawnSync(binary, ['evaluate', '-'], { input: raw, encoding: 'utf8' });
  assert.equal(native.status, 2, 'native decoder rejects the same input');
}

try {
  execFileSync('go', ['build', '-trimpath', '-o', binary, './cmd/storepath'], { cwd: root });
  const expected = {
    'transactional-db': 'block', 'telemetry-retention': 'hybrid', 'backup-repository': 'object',
    'analytics-range-reads': 'object', 'hot-object-service': 'hybrid', 'strict-latency': null,
  };
  assert.deepEqual(Object.keys(STOREPATH_PRESETS).sort(), Object.keys(expected).sort());
  for (const [name, recommendation] of Object.entries(expected)) {
    const fixtureText = readFileSync(path.join(root, 'examples', `${name}.json`), 'utf8');
    const fixture = JSON.parse(fixtureText);
    assert.deepEqual(STOREPATH_PRESETS[name], fixture, `${name} embeds the canonical fixture`);
    const decision = parity(fixtureText);
    assert.equal(decision.recommendation ?? null, recommendation, `${name} expected outcome`);
  }

  const strict = structuredClone(STOREPATH_PRESETS['strict-latency']);
  assert.equal(parity(strict).status, 'benchmark_required');
  const alternateCase = structuredClone(strict);
  alternateCase.workload.TARGET_P99_MS = alternateCase.workload.target_p99_ms;
  delete alternateCase.workload.target_p99_ms;
  const normalized = JSON.parse(storepathEvaluate(JSON.stringify(alternateCase)));
  assert.equal(normalized.input.workload.target_p99_ms, 5,
    'case-insensitive Go decoding returns the canonical optional constraint to the UI');
  assert.equal(normalized.input.workload.TARGET_P99_MS, undefined);
  assert.equal(parity(alternateCase).status, 'benchmark_required');
  assert.deepEqual(parity(normalized.input), normalized.decision,
    'the canonical exported input recreates the evaluated decision');
  strict.workload.measured_p99_ms = { block: 2, object: 7 };
  const measured = parity(strict);
  assert.equal(measured.status, 'latency_evidence_satisfied');
  assert.equal(measured.recommendation, 'block');
  strict.workload.measured_p99_ms = { block: 9, object: 7 };
  assert.equal(parity(strict).status, 'no_feasible_plan');

  const backup = structuredClone(STOREPATH_PRESETS['backup-repository']);
  assert.equal(parity(backup).recommendation, 'object');
  backup.rates.object_gib_month = 1;
  assert.equal(parity(backup).recommendation, 'block', 'a rate edit changes the computed decision');

  const db = structuredClone(STOREPATH_PRESETS['transactional-db']);
  db.rates.object_gib_month = 0;
  assert.equal(parity(db).recommendation, 'block', 'free object storage cannot bypass semantics');

  invalid('{');
  invalid('{"schema_version":1,"schema_version":1}');
  invalid(JSON.stringify({ ...backup, private_field: true }));
  invalid(JSON.stringify({ ...backup, workload: null }));
  invalid(' '.repeat(2 * 1024 * 1024 + 1));
  for (const args of [[], [null], [42], ['{}', '{}']]) {
    const reply = JSON.parse(storepathEvaluate(...args));
    assert.ok(reply.error, 'bridge rejects missing or incorrectly typed arguments');
    assert.equal(reply.decision, undefined);
    assert.equal(reply.input, undefined);
  }

  // A data string may look like HTML; the engine must preserve it as data.
  backup.workload.name = '</script><img src=x onerror=alert(1)> & workload';
  assert.equal(parity(backup).workload, backup.workload.name);
  console.log('Browser release parity passed: six fixtures, editable costs, semantic gates, latency evidence, and invalid-input boundaries.');
} finally {
  clearTimeout(deadline);
  rmSync(temp, { recursive: true, force: true });
}
// The browser bridge intentionally waits for future calls. Finish this test
// process after checking the exact embedded module, without changing the engine.
void engineRun;
process.exit(0);
