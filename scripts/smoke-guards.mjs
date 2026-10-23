export function smokeConfiguration(env) {
  if (env.SMOKE_DEDICATED_WORKSPACE !== 'true') throw Error('Smoke requires explicit dedicated non-production workspace configuration');
  const url = new URL(env.SMOKE_ENDPOINT);
  if (url.protocol !== 'https:' || url.username || url.password || url.pathname !== '/' || url.search || url.hash) throw Error('Smoke endpoint must be an HTTPS origin');
  if (!env.SMOKE_EXPECTED_SUB || !env.SMOKE_EXPECTED_ISSUER) throw Error('Expected smoke principal must be configured');
  const token = env.SMOKE_ID_TOKEN;
  if (!token || token.length > 16384 || token.split('.').length !== 3) throw Error('Missing or malformed smoke ID token');
  let claims;
  try { claims = JSON.parse(Buffer.from(token.split('.')[1], 'base64url').toString()); }
  catch { throw Error('Malformed smoke claims'); }
  // Configuration guard only. The service must authenticate this token.
  if (claims.sub !== env.SMOKE_EXPECTED_SUB || claims.iss !== env.SMOKE_EXPECTED_ISSUER || claims.token_use !== 'id') throw Error('Smoke principal does not match configured isolated workspace');
  if (!Number.isFinite(claims.exp) || claims.exp * 1000 < Date.now() + 15 * 60_000) throw Error('Smoke ID token must remain valid for at least 15 minutes');
  return { endpoint: url.origin, token };
}
export function assertEmptyBaseline(alarms, enrichment) {
  if (!Array.isArray(alarms.rules) || alarms.rules.length || !Number.isSafeInteger(alarms.rev) || alarms.rev < 0) throw Error('Smoke workspace must have an empty alarm list');
  for (const [key, value] of Object.entries({ mintFloor: 0.6, mergeThreshold: 0.86, consolidationCadence: 'nightly', budgetPct: 0.25 })) {
    if (enrichment[key] !== value) throw Error('Smoke workspace must have default enrichment settings');
  }
}
