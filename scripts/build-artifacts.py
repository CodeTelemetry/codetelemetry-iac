#!/usr/bin/env python3
"""Build unsigned candidate archives; the trusted CI job signs their manifest."""
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
VERSION = os.environ.get('RELEASE_VERSION', '')
REVISION = os.environ.get('GITHUB_SHA', '')
if not re.fullmatch(r'0\.0\.0-dev\.[1-9][0-9]*', VERSION):
    raise SystemExit('RELEASE_VERSION must be a CI candidate version: 0.0.0-dev.RUN_NUMBER')
if not re.fullmatch(r'[a-f0-9]{40}', REVISION):
    raise SystemExit('GITHUB_SHA must identify the exact source revision')
if subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip() != REVISION:
    raise SystemExit('Checkout does not match source revision')
DIST = ROOT / 'dist'
DIST.mkdir(exist_ok=True)
if any(DIST.iterdir()):
    raise SystemExit('dist must be empty')
if subprocess.run(['git', 'diff', '--quiet', 'HEAD', '--'], cwd=ROOT).returncode:
    raise SystemExit('Tracked source must match the committed revision')
go = os.environ.get('GO_BINARY', 'go')
provider = ROOT / 'terraform-provider-codetelemetry'
# Include upstream module license/notice files in every binary archive.
subprocess.run([go, 'mod', 'download'], cwd=provider, check=True, timeout=180)
modules = subprocess.check_output([go, 'list', '-m', '-json', 'all'], cwd=provider, timeout=120, text=True)
decoder = json.JSONDecoder()
notices = []
while modules.strip():
    module, index = decoder.raw_decode(modules.lstrip())
    modules = modules.lstrip()[index:]
    directory = module.get('Dir')
    if not directory or module.get('Main'):
        continue
    for path in sorted(Path(directory).iterdir()):
        if path.is_file() and re.match(r'(?i)^(license|copying|notice|copyright)(\.|$)', path.name):
            if path.stat().st_size > 1_000_000:
                raise SystemExit('Oversized third-party notice')
            notices.append(f'\n--- {module["Path"]} {module.get("Version", "")} / {path.name} ---\n' + path.read_text(errors='replace'))
goroot = subprocess.check_output([go, 'env', 'GOROOT'], text=True).strip()
notices.append('\n--- Go standard library LICENSE ---\n' + (Path(goroot) / 'LICENSE').read_text())
notices_text = ''.join(notices)
if len(notices_text.encode()) > 10_000_000:
    raise SystemExit('Third-party notices exceed bound')
platforms = [(system, arch) for system in ('linux', 'darwin', 'windows') for arch in ('amd64', 'arm64')]
with tempfile.TemporaryDirectory(prefix='ct-release-') as temp:
    for system, arch in platforms:
        filename = f'terraform-provider-codetelemetry_v{VERSION}' + ('.exe' if system == 'windows' else '')
        binary = Path(temp) / filename
        subprocess.run([go, 'build', '-trimpath', '-buildvcs=true', '-ldflags=' + f'-s -w -X main.version={VERSION}', '-o', str(binary), '.'],
                       cwd=provider, env=dict(os.environ, CGO_ENABLED='0', GOOS=system, GOARCH=arch), check=True, timeout=300)
        archive = DIST / f'terraform-provider-codetelemetry_{VERSION}_{system}_{arch}.zip'
        with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as output:
            info = zipfile.ZipInfo(filename, (2020, 1, 1, 0, 0, 0))
            info.external_attr = 0o100755 << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            output.writestr(info, binary.read_bytes())
            output.writestr('LICENSE', (ROOT / 'LICENSE').read_bytes())
            output.writestr('THIRD_PARTY_NOTICES.txt', notices_text)
# SDK is a local-bridge candidate, not a registry-backed standalone npm release.
sdk = ROOT / 'pulumi-codetelemetry/sdks/codetelemetry'
metadata = json.loads((sdk / 'package.json').read_text())
metadata['version'] = VERSION
(sdk / 'package.json').write_text(json.dumps(metadata, indent=2) + '\n')
subprocess.run(['npm', 'pack', '--ignore-scripts', '--pack-destination', str(DIST)], cwd=sdk, check=True, timeout=120)
(DIST / 'BUILD.json').write_text(json.dumps({
    'repository': 'CodeTelemetry/terraform-provider-codetelemetry', 'revision': REVISION,
    'version': VERSION, 'platforms': platforms,
    'run': os.environ.get('GITHUB_RUN_ID'), 'attempt': os.environ.get('GITHUB_RUN_ATTEMPT'),
    'go': subprocess.check_output([go, 'version'], text=True).strip(),
    'pulumiSdk': 'local-bridge candidate; companion provider binary required in consumer ./bin',
}, indent=2) + '\n')
checksums = ''.join(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n' for path in sorted(DIST.iterdir()))
(DIST / 'SHA256SUMS').write_text(checksums)
