import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { authenticationConfiguration } from './auth/authenticate.mjs';
import { runAuthenticatedSmoke } from './authenticated-smoke.mjs';

try {
  authenticationConfiguration(process.env);
  let token;
  try {
    token = execFileSync(process.execPath, [fileURLToPath(new URL('./auth/login.mjs', import.meta.url))], {
      env: process.env, stdio: 'pipe', encoding: 'utf8', timeout: 60_000, maxBuffer: 32768,
    }).trim();
  } catch { throw Error('Dedicated smoke sign-in failed; auth output withheld'); }
  // Password and token never enter command arguments, GITHUB_ENV, or log output.
  const env = { ...process.env, SMOKE_ID_TOKEN: token };
  delete env.SMOKE_PASSWORD;
  delete process.env.SMOKE_PASSWORD;
  await runAuthenticatedSmoke(env, process.argv[2], process.argv[3]);
} catch (error) { console.error(error.message); process.exitCode = 1; }
