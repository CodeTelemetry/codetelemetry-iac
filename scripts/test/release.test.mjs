import assert from 'node:assert/strict';
import test from 'node:test';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtempSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { verifyContents } from '../verify-artifacts.mjs';
import { smokeConfiguration, assertEmptyBaseline } from '../smoke-guards.mjs';

test('artifact validation rejects tampering, incomplete sets, duplicate names, traversal and wrong revision', () => {
  const directory = mkdtempSync(join(tmpdir(), 'ct-artifact-test-'));
  try {
    const revision = 'a'.repeat(40), version = '0.0.0-dev.1';
    const files = { 'BUILD.json': JSON.stringify({ repository: 'CodeTelemetry/terraform-provider-codetelemetry', revision, version }),
      [`codetelemetry-pulumi-codetelemetry-${version}.tgz`]: 'sdk' };
    for (const os of ['linux', 'darwin', 'windows']) for (const arch of ['amd64', 'arm64']) files[`terraform-provider-codetelemetry_${version}_${os}_${arch}.zip`] = 'binary';
    let sums = '';
    for (const [name, value] of Object.entries(files)) {
      writeFileSync(join(directory, name), value);
      sums += `${createHash('sha256').update(value).digest('hex')}  ${name}\n`;
    }
    const manifest = join(directory, 'SHA256SUMS');
    const check = () => verifyContents(directory, revision, version);
    writeFileSync(manifest, sums); check();
    assert.throws(() => verifyContents(directory, 'b'.repeat(40), version), /provenance/);
    writeFileSync(manifest, sums + sums.split('\n')[0] + '\n'); assert.throws(check, /duplicate/);
    writeFileSync(manifest, sums.split('\n').slice(1).join('\n')); assert.throws(check, /Missing/);
    writeFileSync(manifest, `${'a'.repeat(64)}  ../outside\n`); assert.throws(check, /Unexpected/);
    writeFileSync(manifest, sums); writeFileSync(join(directory, 'BUILD.json'), '{}'); assert.throws(check, /checksum/);
  } finally { rmSync(directory, { recursive: true, force: true }); }
});

test('live smoke refuses missing credentials, wrong tenant, expiring credentials and nonempty configuration', () => {
  const token = (claims) => 'header.' + Buffer.from(JSON.stringify(claims)).toString('base64url') + '.signature';
  const claims = { iss: 'https://issuer.example/pool', sub: 'isolated-user', token_use: 'id', exp: Date.now() / 1000 + 3600 };
  const env = { SMOKE_DEDICATED_WORKSPACE: 'true', SMOKE_ENDPOINT: 'https://test.example', SMOKE_EXPECTED_SUB: claims.sub, SMOKE_EXPECTED_ISSUER: claims.iss, SMOKE_ID_TOKEN: token(claims) };
  assert.equal(smokeConfiguration(env).endpoint, 'https://test.example');
  for (const patch of [{ SMOKE_ID_TOKEN: '' }, { SMOKE_DEDICATED_WORKSPACE: '' }, { SMOKE_EXPECTED_SUB: 'another-user' }, { SMOKE_ENDPOINT: 'http://test.example' }, { SMOKE_ID_TOKEN: token({ ...claims, exp: 1 }) }]) assert.throws(() => smokeConfiguration({ ...env, ...patch }));
  const defaults = { mintFloor: 0.6, mergeThreshold: 0.86, consolidationCadence: 'nightly', budgetPct: 0.25 };
  assertEmptyBaseline({ rules: [], rev: 0 }, defaults);
  assert.throws(() => assertEmptyBaseline({ rules: [{}], rev: 1 }, defaults));
  assert.throws(() => assertEmptyBaseline({ rules: [], rev: 0 }, { ...defaults, budgetPct: 0.5 }));
});

test('failed consumer commands never echo captured sensitive output', () => {
  const module = new URL('../consumer.mjs', import.meta.url).href;
  const code = `import {command} from ${JSON.stringify(module)}; command(process.execPath, ['-e', 'console.error("sensitive-marker"); process.exit(1)'], process.cwd());`;
  const result = spawnSync(process.execPath, ['--input-type=module', '-e', code], { encoding: 'utf8' });
  assert.notEqual(result.status, 0);
  assert.ok(!result.stdout.includes('sensitive-marker') && !result.stderr.includes('sensitive-marker'));
  assert.match(result.stderr, /output withheld/);
});

test('failed import restores cleanup ownership for both singleton resources', async () => {
  const { importWithRecovery } = await import('../import-with-recovery.mjs');
  const root = mkdtempSync(join(tmpdir(), 'ct-import-test-'));
  try {
    for (const address of ['codetelemetry_alarm_rules.main', 'codetelemetry_enrichment_config.main']) {
      const baseline = { [address]: { id: 'workspace', revision: 7 } };
      let state = structuredClone(baseline);
      const terraform = (...args) => {
        if (args[0] === 'state' && args[1] === 'pull') return JSON.stringify(state);
        if (args[0] === 'state' && args[1] === 'rm') { delete state[args[2]]; return ''; }
        if (args[0] === 'import') throw Error('simulated import-time HTTP failure');
        if (args[0] === 'state' && args[1] === 'push') { state = JSON.parse(readFileSync(args[3], 'utf8')); return ''; }
        throw Error('Unexpected Terraform operation');
      };
      assert.throws(() => importWithRecovery(terraform, address, join(root, 'recovery.tfstate')), /simulated import-time HTTP failure/);
      assert.deepEqual(state, baseline, 'destroy must retain the remote singleton binding');
    }
  } finally { rmSync(root, { recursive: true, force: true }); }
});
