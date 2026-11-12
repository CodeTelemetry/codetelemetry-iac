import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, copyFileSync, chmodSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

export function command(tool, args, cwd, env = process.env) {
  try { return execFileSync(tool, args, { cwd, env, encoding: 'utf8', stdio: 'pipe', timeout: 120_000, maxBuffer: 4 * 1024 * 1024 }); }
  catch { throw Error(`${tool} ${args[0]} failed; tool output withheld to protect credentials and state`); }
}
export function consumer(directory, version) {
  if (process.platform !== 'linux' || process.arch !== 'x64') throw Error('Consumer checks require the Linux amd64 CI runner');
  if (!/^0\.0\.0-dev\.[1-9][0-9]*$/.test(version)) throw Error('Invalid candidate version');
  const root = mkdtempSync(join(tmpdir(), 'ct-consumer-')); chmodSync(root, 0o700);
  try {
  const tf = join(root, 'terraform'), mirror = join(root, 'mirror');
  for (const path of [tf, mirror]) mkdirSync(path, { recursive: true });
  const archive = `terraform-provider-codetelemetry_${version}_linux_amd64.zip`;
  const destination = join(mirror, 'registry.terraform.io/codetelemetry/codetelemetry');
  mkdirSync(destination, { recursive: true });
  copyFileSync(join(directory, archive), join(destination, archive));
  writeFileSync(join(root, 'terraform.rc'), `provider_installation {\n filesystem_mirror { path = ${JSON.stringify(mirror)} }\n}\n`);
  writeFileSync(join(tf, 'main.tf'), `terraform {\n required_providers {\n codetelemetry = { source = "CodeTelemetry/codetelemetry", version = "${version}" }\n }\n}\nprovider "codetelemetry" {}\nresource "codetelemetry_alarm_rules" "main" { rules_json = file("\u0024{path.module}/rules.json") }\nresource "codetelemetry_enrichment_config" "main" {\n mint_floor = 0.6\n merge_threshold = 0.86\n consolidation_cadence = "nightly"\n budget_pct = 0.4\n}\n`);
  const env = { ...process.env, TF_CLI_CONFIG_FILE: join(root, 'terraform.rc'), TF_IN_AUTOMATION: 'true', CHECKPOINT_DISABLE: '1' };
  for (const key of ['CODETELEMETRY_TOKEN', 'CODETELEMETRY_ENDPOINT', 'SMOKE_ID_TOKEN', 'SMOKE_PASSWORD', 'TF_LOG', 'TF_LOG_PATH']) delete env[key];
  command('terraform', ['init', '-input=false', '-no-color'], tf, env);
  command('terraform', ['providers', 'schema', '-json'], tf, env);
  return { root, tf, env };
  } catch (error) {
    rmSync(root, { recursive: true, force: true });
    throw error;
  }
}
