#!/usr/bin/env bash
set -Eeuo pipefail

temporary_dir=$(mktemp -d)
trap 'rm -rf "$temporary_dir"' EXIT
version=$(git describe --tags --always --dirty 2>/dev/null || printf dev)
go_command=${GO:-go}
for target in linux/amd64 darwin/amd64 windows/amd64; do
  os=${target%/*}
  arch=${target#*/}
  suffix=
  [[ $os == windows ]] && suffix=.exe
  binary="$temporary_dir/promptline-${os}-${arch}${suffix}"
  GOCACHE="${GOCACHE:-$(pwd)/.gocache}" GOOS="$os" GOARCH="$arch" "$go_command" build -trimpath -ldflags "-s -w -X main.Version=$version" -o "$binary" ./cmd/promptline
  ./scripts/release-metadata.sh "$temporary_dir" "$binary" "$version"
  (cd "$temporary_dir" && shasum -a 256 --check "$(basename "$binary").sha256")
done
