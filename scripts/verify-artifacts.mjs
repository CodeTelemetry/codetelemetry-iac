import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync, lstatSync } from 'node:fs';
import { resolve, join } from 'node:path';
import { fileURLToPath } from 'node:url';

export function verifyContents(directory, revision, version) {
  if (!/^[a-f0-9]{40}$/.test(revision) || !/^0\.0\.0-dev\.[1-9][0-9]*$/.test(version)) throw Error('Invalid expected source/version');
  const expected = ['BUILD.json', `codetelemetry-pulumi-codetelemetry-${version}.tgz`];
  for (const os of ['linux', 'darwin', 'windows']) for (const arch of ['amd64', 'arm64']) {
    expected.push(`terraform-provider-codetelemetry_${version}_${os}_${arch}.zip`);
  }
  const sumsFile = join(directory, 'SHA256SUMS');
  if (lstatSync(sumsFile).size > 8192) throw Error('Oversized checksum manifest');
  const lines = readFileSync(sumsFile, 'utf8').trimEnd().split('\n');
  const found = new Set();
  for (const line of lines) {
    const match = /^([a-f0-9]{64})  ([A-Za-z0-9_.-]+)$/.exec(line);
    if (!match || !expected.includes(match[2]) || found.has(match[2])) throw Error('Unexpected or duplicate artifact');
    found.add(match[2]);
    const path = join(directory, match[2]);
    const stat = lstatSync(path);
    if (!stat.isFile() || stat.isSymbolicLink() || stat.size > 150_000_000) throw Error('Invalid artifact file');
    if (createHash('sha256').update(readFileSync(path)).digest('hex') !== match[1]) throw Error('Artifact checksum mismatch');
  }
  if (found.size !== expected.length) throw Error('Missing artifacts');
  const info = JSON.parse(readFileSync(join(directory, 'BUILD.json'), 'utf8'));
  if (info.revision !== revision || info.version !== version || info.repository !== 'CodeTelemetry/codetelemetry-iac') throw Error('Source provenance mismatch');
  return info;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const [directory, revision, version] = process.argv.slice(2);
  if (!directory || !revision || !version) throw Error('Usage: node scripts/verify-artifacts.mjs DIST EXPECTED_SHA EXPECTED_VERSION');
  execFileSync('cosign', ['verify-blob', join(directory, 'SHA256SUMS'),
    '--bundle', join(directory, 'SHA256SUMS.sigstore.json'),
    '--certificate-identity', 'https://github.com/CodeTelemetry/codetelemetry-iac/.github/workflows/codetelemetry-providers.yml@refs/heads/main',
    '--certificate-oidc-issuer', 'https://token.actions.githubusercontent.com',
    '--certificate-github-workflow-sha', revision,
  ], { stdio: 'inherit', timeout: 180_000 });
  verifyContents(directory, revision, version);
  console.log('Signature, exact source revision, and artifact checksums verified.');
}
