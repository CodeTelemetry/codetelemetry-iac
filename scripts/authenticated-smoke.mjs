import { readFileSync, writeFileSync, rmSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { smokeConfiguration, assertEmptyBaseline } from './smoke-guards.mjs';
import { consumer, command } from './consumer.mjs';
import { importWithRecovery } from './import-with-recovery.mjs';

async function main() {
  // Refuse all network/CLI work until the isolated principal has been checked.
  const { endpoint, token } = smokeConfiguration(process.env);
  async function get(path) {
    const response = await fetch(endpoint + path, { headers: { Authorization: `Bearer ${token}` }, redirect: 'error', signal: AbortSignal.timeout(30_000) });
    if (!response.ok) throw Error('Authenticated service request failed');
    let size = 0; const chunks = [];
    for await (const chunk of response.body) {
      size += chunk.length;
      if (size > 2 * 1024 * 1024) { await response.body.cancel().catch(() => {}); throw Error('Oversized service response'); }
      chunks.push(chunk);
    }
    return JSON.parse(Buffer.concat(chunks).toString());
  }
  async function empty() { assertEmptyBaseline(await get('/api/alert-rules'), await get('/api/enrichment-config')); }
  await empty();
  const project = consumer(resolve(process.argv[2]), process.argv[3]);
  const env = { ...project.env, CODETELEMETRY_ENDPOINT: endpoint, CODETELEMETRY_TOKEN: token };
  const terraform = (...args) => command('terraform', args[0] === 'state' ? args : [args[0], '-no-color', ...args.slice(1)], project.tf, env);
  const pulumi = (...args) => command(process.env.PULUMI_BINARY || 'pulumi', [...args, '--non-interactive', '--color', 'never'], project.pulumi, env);
  let tfAttempted = false, pulumiAttempted = false;
  const rules = JSON.parse(readFileSync(new URL('../terraform-provider-codetelemetry/examples/rules.json', import.meta.url), 'utf8'));
  rules[0].id = 'ci-artifact-smoke'; rules[0].name = 'CI artifact smoke (disabled)'; rules[0].enabled = false;
  try {
    await empty();
    writeFileSync(join(project.tf, 'rules.json'), JSON.stringify(rules));
    tfAttempted = true;
    terraform('apply', '-auto-approve', '-input=false');
    terraform('plan', '-detailed-exitcode', '-input=false');
    rules[0].tql.threshold = 201; writeFileSync(join(project.tf, 'rules.json'), JSON.stringify(rules));
    terraform('apply', '-auto-approve', '-input=false');
    importWithRecovery(terraform, 'codetelemetry_alarm_rules.main', join(project.tf, 'import-recovery.tfstate'));
    importWithRecovery(terraform, 'codetelemetry_enrichment_config.main', join(project.tf, 'import-recovery.tfstate'));
    terraform('plan', '-detailed-exitcode', '-input=false');
    terraform('destroy', '-auto-approve', '-input=false'); tfAttempted = false;
    await empty();
    writeFileSync(join(project.pulumi, 'index.js'), `const pulumi=require('@pulumi/pulumi');\nconst ct=require('@codetelemetry/pulumi-codetelemetry');\nconst fs=require('node:fs');\nnew ct.AlarmRules('alarms',{rulesJson:pulumi.secret(fs.readFileSync('rules.json','utf8'))});\nnew ct.EnrichmentConfig('enrichment',{mintFloor:0.6,mergeThreshold:0.86,consolidationCadence:'nightly',budgetPct:0.4});\n`);
    writeFileSync(join(project.pulumi, 'rules.json'), JSON.stringify(rules));
    pulumi('stack', 'init', 'smoke', '--secrets-provider', 'passphrase');
    pulumiAttempted = true;
    pulumi('up', '--yes', '--skip-preview', '--stack', 'smoke');
    pulumi('preview', '--expect-no-changes', '--stack', 'smoke');
    rules[0].tql.threshold = 202; writeFileSync(join(project.pulumi, 'rules.json'), JSON.stringify(rules));
    pulumi('up', '--yes', '--skip-preview', '--stack', 'smoke');
    pulumi('refresh', '--yes', '--stack', 'smoke');
    pulumi('preview', '--expect-no-changes', '--stack', 'smoke');
  } finally {
    let cleanupFailed = false;
    if (pulumiAttempted) { try { pulumi('destroy', '--yes', '--stack', 'smoke'); } catch { cleanupFailed = true; } }
    if (tfAttempted) { try { terraform('destroy', '-auto-approve', '-input=false'); } catch { cleanupFailed = true; } }
    try { await empty(); } catch { cleanupFailed = true; }
    rmSync(project.root, { recursive: true, force: true });
    if (cleanupFailed) throw Error('Smoke cleanup failed: dedicated workspace needs maintainer intervention');
  }
  console.log('Authenticated Terraform/Pulumi artifact lifecycle and cleanup passed.');
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });
