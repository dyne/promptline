#!/usr/bin/env bash
set -Eeuo pipefail

go_mod=${GO_MOD:-go.mod}
go_directive=$(awk '$1 == "go" { print $2; exit }' "$go_mod")
toolchain_directive=$(awk '$1 == "toolchain" { print $2; exit }' "$go_mod")
selected_toolchain=$(tr -d '[:space:]' < .go-version)

[[ $go_directive == 1.24.0 ]] || {
  printf 'go.mod must preserve Go 1.24.0 language semantics; found %s\n' "$go_directive" >&2
  exit 1
}
[[ $toolchain_directive =~ ^go1\.25\.[0-9]+$ ]] || {
  printf 'toolchain must be a supported Go 1.25 patch release; found %s\n' "$toolchain_directive" >&2
  exit 1
}
[[ $selected_toolchain == "${toolchain_directive#go}" ]] || {
  printf '.go-version (%s) must match go.mod toolchain (%s)\n' "$selected_toolchain" "$toolchain_directive" >&2
  exit 1
}

# The go directive is Go's compiler-enforced language version. Go 1.25 made
# no language changes, so no honest Go 1.25-only syntax fixture exists. Guard
# the actual boundary instead: an attempted Go 1.25 language directive must
# be rejected by this same policy checker.
if [[ ${GO_POLICY_SELF_TEST:-} != 1 ]]; then
  fixture=$(mktemp)
  trap 'rm -f "$fixture"' EXIT
  sed 's/^go 1\.24\.0$/go 1.25.0/' "$go_mod" > "$fixture"
  if GO_MOD="$fixture" GO_POLICY_SELF_TEST=1 "$0" >/dev/null 2>&1; then
    printf 'policy accepted a Go 1.25 language directive\n' >&2
    exit 1
  fi
fi
