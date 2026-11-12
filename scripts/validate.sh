#!/usr/bin/env bash
# Validate the provider: formatting, dependency checks, vet, race and acceptance tests.
set -euo pipefail

repo_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
# --terraform is accepted for compatibility with the former two-integration layout.
if (( $# > 1 )) || { (( $# == 1 )) && [[ "$1" != "--terraform" ]]; }; then
  echo "Usage: bash scripts/validate.sh" >&2
  exit 2
fi
export GO_BINARY="${GO_BINARY:-go}"
terraform_binary="${TF_ACC_TERRAFORM_PATH:-$(command -v terraform || true)}"
terraform_binary="${terraform_binary:-terraform}"
export TF_ACC_TERRAFORM_PATH="$terraform_binary"
for binary in "$GO_BINARY" "$terraform_binary"; do
  command -v "$binary" >/dev/null || { echo "Missing required tool: $binary" >&2; exit 1; }
done

# Go tools and acceptance-test helpers may launch `go` as a child process.
go_bin_dir=$(dirname -- "$(command -v "$GO_BINARY")")
PATH="$go_bin_dir:$PATH"
export PATH

cd "$repo_dir"
gofmt_binary="$("$GO_BINARY" env GOROOT)/bin/gofmt"
unformatted=$("$gofmt_binary" -l main.go internal)
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
