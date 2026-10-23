import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, copyFileSync, chmodSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { randomBytes } from 'node:crypto';

export function command(tool, args, cwd, env = process.env) {
  try { return execFileSync(tool, args, { cwd, env, encoding: 'utf8', timeout: 120_000, maxBuffer: 4 * 1024 * 1024 }); }
  catch { throw Error(`${tool} ${args[0]} failed; tool output withheld to protect credentials and state`); }
}
export function consumer(directory, version) {
  if (process.platform !== 'linux' || process.arch !== 'x64') throw Error('Consumer checks require the Linux amd64 CI runner');
  if (!/^0\.0\.0-dev\.[1-9][0-9]*$/.test(version)) throw Error('Invalid candidate version');
  const root = mkdtempSync(join(tmpdir(), 'ct-consumer-')); chmodSync(root, 0o700);
  const tf = join(root, 'terraform'), pulumi = join(root, 'pulumi'), mirror = join(root, 'mirror');
  for (const path of [tf, pulumi, mirror, join(pulumi, 'bin'), join(root, 'backend')]) mkdirSync(path, { recursive: true });
  const archive = `terraform-provider-codetelemetry_${version}_linux_amd64.zip`;
  const destination = join(mirror, 'registry.terraform.io/codetelemetry/codetelemetry');
  mkdirSync(destination, { recursive: true });
  copyFileSync(join(directory, archive), join(destination, archive));
  command('unzip', ['-q', join(directory, archive), '-d', join(pulumi, 'bin')], root);
  const binary = join(pulumi, 'bin', `terraform-provider-codetelemetry_v${version}`);
  chmodSync(binary, 0o755); copyFileSync(binary, join(pulumi, 'bin/terraform-provider-codetelemetry'));
  chmodSync(join(pulumi, 'bin/terraform-provider-codetelemetry'), 0o755);
  writeFileSync(join(root, 'terraform.rc'), `provider_installation {\n filesystem_mirror { path = ${JSON.stringify(mirror)} }\n}\n`);
  writeFileSync(join(tf, 'main.tf'), `terraform {\n required_providers {\n codetelemetry = { source = "CodeTelemetry/codetelemetry", version = "${version}" }\n }\n}\nprovider "codetelemetry" {}\nresource "codetelemetry_alarm_rules" "main" { rules_json = file("\u0024{path.module}/rules.json") }\nresource "codetelemetry_enrichment_config" "main" {\n mint_floor = 0.6\n merge_threshold = 0.86\n consolidation_cadence = "nightly"\n budget_pct = 0.4\n}\n`);
  writeFileSync(join(pulumi, 'Pulumi.yaml'), 'name: artifact-smoke\nruntime: nodejs\n');
  writeFileSync(join(pulumi, 'package.json'), JSON.stringify({ name: 'artifact-smoke', private: true,
    dependencies: { '@codetelemetry/pulumi-codetelemetry': `file:${join(directory, `codetelemetry-pulumi-codetelemetry-${version}.tgz`)}`, '@pulumi/pulumi': '3.268.0' } }));
  const env = { ...process.env, TF_CLI_CONFIG_FILE: join(root, 'terraform.rc'), TF_IN_AUTOMATION: 'true', CHECKPOINT_DISABLE: '1',
    PULUMI_BACKEND_URL: pathToFileURL(join(root, 'backend')).href, PULUMI_CONFIG_PASSPHRASE: randomBytes(32).toString('hex'), PULUMI_SKIP_UPDATE_CHECK: 'true' };
  for (const key of ['CODETELEMETRY_TOKEN', 'CODETELEMETRY_ENDPOINT', 'SMOKE_ID_TOKEN', 'TF_LOG', 'TF_LOG_PATH', 'PULUMI_ACCESS_TOKEN', 'PULUMI_CONFIG', 'PULUMI_STACK']) delete env[key];
  command('terraform', ['init', '-input=false', '-no-color'], tf, env);
  command('terraform', ['providers', 'schema', '-json'], tf, env);
  command('npm', ['install', '--ignore-scripts', '--no-audit', '--no-fund'], pulumi, env);
  command(process.env.PULUMI_BINARY || 'pulumi', ['plugin', 'install', 'resource', 'terraform-provider', '1.5.0'], pulumi, env);
  command('node', ['-e', 'const sdk=require("@codetelemetry/pulumi-codetelemetry"); if(!sdk.AlarmRules||!sdk.EnrichmentConfig) process.exit(1)'], pulumi, env);
  return { root, tf, pulumi, env };
}
