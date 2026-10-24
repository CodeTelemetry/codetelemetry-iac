import assert from 'node:assert/strict';
import test from 'node:test';
import { spawnSync } from 'node:child_process';
import { authenticate, authenticationConfiguration } from '../auth/authenticate.mjs';

const base = {
  SMOKE_DEDICATED_WORKSPACE: 'true', SMOKE_ENDPOINT: 'https://smoke.example',
  SMOKE_USER_POOL_ID: 'us-east-1_pool', SMOKE_CLIENT_ID: 'client123',
  SMOKE_EXPECTED_ISSUER: 'https://cognito-idp.us-east-1.amazonaws.com/us-east-1_pool',
  SMOKE_EXPECTED_SUB: 'smoke-user', SMOKE_USERNAME: 'ci@example.test', SMOKE_PASSWORD: 'private-password-marker',
};
function idToken(patch = {}) {
  return 'header.' + Buffer.from(JSON.stringify({ iss: base.SMOKE_EXPECTED_ISSUER, sub: base.SMOKE_EXPECTED_SUB,
    aud: base.SMOKE_CLIENT_ID, token_use: 'id', exp: Date.now() / 1000 + 3600, ...patch })).toString('base64url') + '.signature';
}
function sdk(callback) {
  return {
    CognitoUserPool: class { constructor(config) { this.config = config; } },
    AuthenticationDetails: class { constructor(config) { this.config = config; } },
    CognitoUser: class {
      constructor(config) { this.config = config; }
      setAuthenticationFlowType(flow) { assert.equal(flow, 'USER_SRP_AUTH'); }
      authenticateUser(details, callbacks) { callback(this.config, details.config, callbacks); }
    },
  };
}

test('SRP returns a fresh token only for the configured principal/client and clears cached secrets', async () => {
  let storage;
  const token = idToken();
  const actual = await authenticate(base, async () => sdk((config, credentials, callbacks) => {
    storage = config.Storage; storage.setItem('transient', credentials.Password);
    assert.equal(credentials.Username, base.SMOKE_USERNAME);
    callbacks.onSuccess({ getIdToken: () => ({ getJwtToken: () => token }) });
  }));
  assert.equal(actual, token);
  assert.equal(storage.getItem('transient'), null);
  for (const patch of [{ sub: 'another-tenant' }, { aud: 'another-client' }, { exp: 1 }, { token_use: 'access' }]) {
    await assert.rejects(authenticate(base, async () => sdk((_config, _credentials, callbacks) => {
      callbacks.onSuccess({ getIdToken: () => ({ getJwtToken: () => idToken(patch) }) });
    })), /Dedicated smoke authentication failed/);
  }
});

test('misconfigured credentials never load the auth client or send a password', async () => {
  let loaded = false;
  for (const patch of [{ SMOKE_EXPECTED_ISSUER: 'https://attacker.example/pool' }, { SMOKE_USER_POOL_ID: 'bad' },
    { SMOKE_PASSWORD: '' }, { SMOKE_DEDICATED_WORKSPACE: '' }, { SMOKE_CLIENT_ID: 'invalid/client' }]) {
    await assert.rejects(authenticate({ ...base, ...patch }, async () => { loaded = true; }));
  }
  assert.equal(loaded, false);
  assert.equal(authenticationConfiguration(base).poolId, base.SMOKE_USER_POOL_ID);
});

test('service errors and interactive challenges fail without exposing credentials', async () => {
  for (const event of ['onFailure', 'newPasswordRequired', 'mfaRequired', 'totpRequired', 'customChallenge', 'selectMFAType', 'mfaSetup']) {
    await assert.rejects(authenticate(base, async () => sdk((_config, credentials, callbacks) => {
      callbacks[event](Error(credentials.Password));
    })), error => !error.message.includes(base.SMOKE_PASSWORD) && /Dedicated smoke authentication failed/.test(error.message));
  }
  const result = spawnSync(process.execPath, ['scripts/run-authenticated-smoke.mjs', 'dist', '0.0.0-dev.1'], {
    env: { ...process.env, ...base, SMOKE_EXPECTED_ISSUER: 'https://attacker.example' }, encoding: 'utf8',
  });
  assert.notEqual(result.status, 0);
  assert.ok(!result.stdout.includes(base.SMOKE_PASSWORD) && !result.stderr.includes(base.SMOKE_PASSWORD));
});
