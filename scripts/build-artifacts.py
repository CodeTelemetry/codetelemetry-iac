#!/usr/bin/env python3
"""Build unsigned provider archives and their checksum manifest.

A candidate version (0.0.0-dev.RUN_NUMBER) produces the main-branch candidate
set that the trusted CI job signs with Cosign. A stable version (X.Y.Z)
produces Terraform Registry release assets that the release job signs with GPG.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
NAME = 'terraform-provider-codetelemetry'
VERSION = os.environ.get('RELEASE_VERSION', '')
REVISION = os.environ.get('GITHUB_SHA', '')
CANDIDATE = bool(re.fullmatch(r'0\.0\.0-dev\.[1-9][0-9]*', VERSION))
STABLE = bool(re.fullmatch(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)', VERSION)) and VERSION != '0.0.0'
if not CANDIDATE and not STABLE:
    raise SystemExit('RELEASE_VERSION must be a candidate (0.0.0-dev.RUN_NUMBER) or stable (X.Y.Z) version')
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
# Include upstream module license/notice files in every binary archive.
subprocess.run([go, 'mod', 'download'], cwd=ROOT, check=True, timeout=180)
modules = subprocess.check_output([go, 'list', '-m', '-json', 'all'], cwd=ROOT, timeout=120, text=True)
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


def add(output, name, data, mode):
    # Fixed entry times keep archives independent of the build clock.
    info = zipfile.ZipInfo(name, (2020, 1, 1, 0, 0, 0))
    info.external_attr = mode << 16
    info.compress_type = zipfile.ZIP_DEFLATED
    output.writestr(info, data)


platforms = [(system, arch) for system in ('linux', 'darwin', 'windows') for arch in ('amd64', 'arm64')]
with tempfile.TemporaryDirectory(prefix='ct-release-') as temp:
    for system, arch in platforms:
        filename = f'{NAME}_v{VERSION}' + ('.exe' if system == 'windows' else '')
        binary = Path(temp) / filename
        subprocess.run([go, 'build', '-trimpath', '-buildvcs=true', '-ldflags=' + f'-s -w -X main.version={VERSION}', '-o', str(binary), '.'],
                       cwd=ROOT, env=dict(os.environ, CGO_ENABLED='0', GOOS=system, GOARCH=arch), check=True, timeout=300)
        with zipfile.ZipFile(DIST / f'{NAME}_{VERSION}_{system}_{arch}.zip', 'w') as output:
            add(output, filename, binary.read_bytes(), 0o100755)
            add(output, 'LICENSE', (ROOT / 'LICENSE').read_bytes(), 0o100644)
            add(output, 'THIRD_PARTY_NOTICES.txt', notices_text, 0o100644)
if CANDIDATE:
    (DIST / 'BUILD.json').write_text(json.dumps({
        'repository': 'CodeTelemetry/terraform-provider-codetelemetry', 'revision': REVISION,
        'version': VERSION, 'platforms': platforms,
        'run': os.environ.get('GITHUB_RUN_ID'), 'attempt': os.environ.get('GITHUB_RUN_ATTEMPT'),
        'go': subprocess.check_output([go, 'version'], text=True).strip(),
    }, indent=2) + '\n')
    sums = DIST / 'SHA256SUMS'
else:
    # The Registry reads protocol versions from this asset and requires these names.
    shutil.copyfile(ROOT / 'terraform-registry-manifest.json', DIST / f'{NAME}_{VERSION}_manifest.json')
    sums = DIST / f'{NAME}_{VERSION}_SHA256SUMS'
sums.write_text(''.join(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n' for path in sorted(DIST.iterdir())))
