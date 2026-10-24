import { smokeScope } from '../smoke-guards.mjs';
import { smokeConfiguration } from '../smoke-guards.mjs';

export function authenticationConfiguration(env) {
  smokeScope(env);
  const pool = /^([a-z]{2}(?:-[a-z]+)+-\d)_([A-Za-z0-9]+)$/.exec(env.SMOKE_USER_POOL_ID || '');
  if (!pool || env.SMOKE_EXPECTED_ISSUER !== `https://cognito-idp.${pool[1]}.amazonaws.com/${env.SMOKE_USER_POOL_ID}`) throw Error('Smoke Cognito pool must match the configured issuer');
  if (!/^[a-z0-9]{1,128}$/.test(env.SMOKE_CLIENT_ID || '')) throw Error('Smoke Cognito client is not configured');
  if (!env.SMOKE_USERNAME || env.SMOKE_USERNAME.length > 128 || !env.SMOKE_PASSWORD || env.SMOKE_PASSWORD.length > 256) throw Error('Smoke username/password are not configured');
  return { poolId: env.SMOKE_USER_POOL_ID, clientId: env.SMOKE_CLIENT_ID, username: env.SMOKE_USERNAME, password: env.SMOKE_PASSWORD };
}

export async function authenticate(env, sdkLoader = () => import('amazon-cognito-identity-js')) {
  const config = authenticationConfiguration(env);
  const cache = new Map();
  const storage = {
    getItem: key => cache.get(key) ?? null,
    setItem: (key, value) => {
      if (String(value).length > 32768 || (!cache.has(key) && cache.size >= 32)) throw Error('Authentication cache exceeds bound');
      cache.set(key, String(value));
    },
    removeItem: key => cache.delete(key),
    clear: () => cache.clear(),
  };
  try {
    const sdk = await sdkLoader();
    const pool = new sdk.CognitoUserPool({ UserPoolId: config.poolId, ClientId: config.clientId, Storage: storage });
    const user = new sdk.CognitoUser({ Username: config.username, Pool: pool, Storage: storage });
    user.setAuthenticationFlowType('USER_SRP_AUTH');
    const token = await new Promise((resolve, reject) => {
      const fail = () => reject(Error('Dedicated smoke authentication failed'));
      user.authenticateUser(new sdk.AuthenticationDetails({ Username: config.username, Password: config.password }), {
        onSuccess: session => { try { resolve(session.getIdToken().getJwtToken()); } catch { fail(); } },
        onFailure: fail, newPasswordRequired: fail, mfaRequired: fail, totpRequired: fail,
        customChallenge: fail, selectMFAType: fail, mfaSetup: fail,
      });
    });
    smokeConfiguration({ ...env, SMOKE_ID_TOKEN: token });
    const claims = JSON.parse(Buffer.from(token.split('.')[1], 'base64url').toString());
    if (claims.aud !== config.clientId) throw Error('Smoke token audience differs from the configured client');
    return token;
  } catch {
    // SDK/service exceptions can embed auth parameters: never forward them.
    throw Error('Dedicated smoke authentication failed; check the isolated account credentials and client configuration');
  } finally { cache.clear(); }
}
