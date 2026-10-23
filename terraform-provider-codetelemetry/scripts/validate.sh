#!/usr/bin/env bash
# Compatibility entrypoint; repository validation covers Terraform and Pulumi.
set -euo pipefail
repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)
exec bash "$repo_dir/scripts/validate.sh" "$@"
