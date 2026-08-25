#!/usr/bin/env bash
set -euo pipefail

go_bin="${GO:-/usr/local/go/bin/go}"
cache_dir="${GOCACHE:-$PWD/.gocache}"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/promptline-coverage.XXXXXX")"
trap 'rm -rf "$work_dir"' EXIT

check_package() {
  package="$1"
  floor="$2"
  profile="$work_dir/$(basename "$package").out"

  GOCACHE="$cache_dir" "$go_bin" test -coverprofile="$profile" "$package"
  coverage="$("$go_bin" tool cover -func="$profile" | awk '/^total:/ { sub(/%$/, "", $3); print $3 }')"
  if [ -z "$coverage" ]; then
    echo "coverage: no total reported for $package" >&2
    exit 1
  fi
  if ! awk -v coverage="$coverage" -v floor="$floor" 'BEGIN { exit !(coverage + 0 >= floor + 0) }'; then
    echo "coverage: $package is ${coverage}%, below its ${floor}% floor" >&2
    exit 1
  fi
  printf 'coverage: %s %s%% (floor %s%%)\n' "$package" "$coverage" "$floor"
}

# Floors cover behavioral boundaries only. Console adapters and command wiring
# remain intentionally outside this gate; see docs/TESTING.md.
check_package ./internal/appserver 87
check_package ./internal/governance 84
check_package ./internal/instance 76
check_package ./internal/mcp 81
check_package ./internal/paths 84
check_package ./internal/runtime 74
check_package ./internal/tools 67
