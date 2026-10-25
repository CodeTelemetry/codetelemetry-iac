import { authenticate } from './authenticate.mjs';
// Invoked only through a bounded private pipe by run-authenticated-smoke.mjs.
// Never run this entry point directly in an Actions run step or tee its stdout.
try { process.stdout.write(await authenticate(process.env)); }
catch { console.error('Dedicated smoke authentication failed'); process.exitCode = 1; }
