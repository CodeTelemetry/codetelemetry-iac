#!/usr/bin/env bash
# Validate both integrations by default; select one explicitly for CI steps.
set -euo pipefail

repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
provider_dir="$repo_dir/terraform-provider-codetelemetry"
pulumi_dir="$repo_dir/pulumi-codetelemetry"
selection="${1:-all}"
case "$selection" in
  all|--terraform|--pulumi) ;;
  *) echo "Usage: bash scripts/validate.sh [--terraform|--pulumi]" >&2; exit 2 ;;
esac
if (( $# > 1 )); then
  echo "Usage: bash scripts/validate.sh [--terraform|--pulumi]" >&2
  exit 2
fi
export GO_BINARY="${GO_BINARY:-go}"
export PULUMI_BINARY="${PULUMI_BINARY:-pulumi}"
required_tools=("$GO_BINARY")
if [[ "$selection" != "--pulumi" ]]; then
  terraform_binary="${TF_ACC_TERRAFORM_PATH:-$(command -v terraform || true)}"
  terraform_binary="${terraform_binary:-terraform}"
  export TF_ACC_TERRAFORM_PATH="$terraform_binary"
  required_tools+=("$terraform_binary")
fi
if [[ "$selection" != "--terraform" ]]; then
  required_tools+=("$PULUMI_BINARY" node npm)
fi
for binary in "${required_tools[@]}"; do
  command -v "$binary" >/dev/null || { echo "Missing required tool: $binary" >&2; exit 1; }
done

# Go tools and acceptance-test helpers may launch `go` as a child process.
go_bin_dir=$(dirname -- "$(command -v "$GO_BINARY")")
PATH="$go_bin_dir:$PATH"
export PATH

if [[ "$selection" != "--pulumi" ]]; then
  echo "=== Terraform: formatting, dependency checks, vet, race and acceptance tests ==="
  cd "$provider_dir"
  gofmt_binary="$("$GO_BINARY" env GOROOT)/bin/gofmt"
  unformatted=$("$gofmt_binary" -l .)
  if [[ -n "$unformatted" ]]; then
    echo "Go files need formatting:" >&2
    echo "$unformatted" >&2
    exit 1
  fi
  "$GO_BINARY" mod tidy -diff
  "$GO_BINARY" vet ./...
  "$terraform_binary" fmt -check -recursive examples
  TF_ACC=1 "$GO_BINARY" test -race -count=1 -timeout=5m -cover ./...
  "$GO_BINARY" build ./...
fi

if [[ "$selection" != "--terraform" ]]; then
  echo "=== Pulumi: provider build, SDK generation, locked install, typecheck and lifecycle tests ==="
  cd "$pulumi_dir"
  metadata_dir=$(mktemp -d)
  trap 'rm -rf -- "$metadata_dir"' EXIT
  for file in package.json package-lock.json Pulumi.yaml; do
    cp "$file" "$metadata_dir/$file"
  done
  npm run setup
  npm test
  for file in package.json package-lock.json Pulumi.yaml; do
    if ! cmp -s "$file" "$metadata_dir/$file"; then
      echo "Setup changed $file; regenerate and commit consistent metadata." >&2
      exit 1
    fi
  done
fi
