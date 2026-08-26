#!/usr/bin/env bash
# Build Promptline, install its executable, and enable the sysadmin Codex plugin.
set -Eeuo pipefail
IFS=$'\n\t'

readonly PLUGIN_NAME='sysadmin'
readonly MARKETPLACE_NAME='promptline'

dry_run=false
temp_root=''

log() {
  printf 'promptline-installer: %s\n' "$*" >&2
}

die() {
  log "error: $*"
  exit 1
}

cleanup() {
  if [[ -n $temp_root && -d $temp_root ]]; then
    rm -rf -- "$temp_root"
  fi
}
trap cleanup EXIT

usage() {
  cat <<'EOF'
Usage: ./install.sh [OPTIONS]

Build Promptline from this checkout, install the executable under
${PROMPTLINE_BIN_DIR:-$HOME/.local/bin}, register the repository as the
`promptline` Codex marketplace, and install and enable `sysadmin`.

Options:
  --bin-dir DIR              Install the promptline executable in DIR.
  --marketplace-source SRC   Marketplace path or Git source (default: checkout).
  --dry-run                  Validate inputs and print the planned actions.
  -h, --help                 Show this help.

Environment:
  GO                         Go executable (default: go).
  CODEX                      Codex executable (default: codex).
  PROMPTLINE_BIN_DIR         Default binary installation directory.
  PROMPTLINE_MARKETPLACE_SOURCE
                             Default marketplace path or Git source.
EOF
}

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
if [[ -n ${PROMPTLINE_BIN_DIR:-} ]]; then
  bin_dir=$PROMPTLINE_BIN_DIR
else
  [[ -n ${HOME:-} ]] || die 'HOME is required unless PROMPTLINE_BIN_DIR is set'
  bin_dir=$HOME/.local/bin
fi
marketplace_source=${PROMPTLINE_MARKETPLACE_SOURCE:-$script_dir}
go_bin=${GO:-go}
codex_bin=${CODEX:-codex}

while (($# > 0)); do
  case $1 in
    --bin-dir)
      (($# >= 2)) || die '--bin-dir requires a value'
      bin_dir=$2
      shift
      ;;
    --marketplace-source)
      (($# >= 2)) || die '--marketplace-source requires a value'
      marketplace_source=$2
      shift
      ;;
    --dry-run) dry_run=true ;;
    -h | --help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
  shift
done

[[ -f $script_dir/go.mod ]] || die "go.mod not found beside installer: $script_dir"
[[ -f $script_dir/.agents/plugins/marketplace.json ]] ||
  die "marketplace manifest not found in checkout: $script_dir"
[[ -n $bin_dir && $bin_dir == /* ]] || die "binary directory must be absolute: $bin_dir"
[[ $bin_dir != / && $bin_dir != "${HOME:-}" ]] ||
  die "refusing unsafe binary directory: $bin_dir"
[[ -n $marketplace_source ]] || die 'marketplace source must not be empty'

command -v "$go_bin" >/dev/null 2>&1 || die "Go executable not found: $go_bin"
command -v "$codex_bin" >/dev/null 2>&1 || die "Codex executable not found: $codex_bin"
command -v grep >/dev/null 2>&1 || die 'grep is required'
command -v install >/dev/null 2>&1 || die 'install is required'
command -v mktemp >/dev/null 2>&1 || die 'mktemp is required'

if "$dry_run"; then
  log "would build $script_dir/cmd/promptline with $go_bin"
  log "would install promptline at $bin_dir/promptline"
  log "would add marketplace source $marketplace_source with $codex_bin"
  log "would install and enable $PLUGIN_NAME@$MARKETPLACE_NAME"
  exit 0
fi

temp_root=$(mktemp -d "${TMPDIR:-/tmp}/promptline-install.XXXXXXXX") ||
  die 'could not create a temporary directory'
built_binary=$temp_root/promptline

log 'building Promptline'
(
  cd -- "$script_dir"
  GOCACHE="$script_dir/.gocache" "$go_bin" build -trimpath \
    -o "$built_binary" ./cmd/promptline
)

mkdir -p -- "$bin_dir"
staged_target=$(mktemp "$bin_dir/.promptline.XXXXXXXX") ||
  die 'could not stage the binary installation'
install -m 0755 "$built_binary" "$staged_target"
mv -f -- "$staged_target" "$bin_dir/promptline"
log "installed Promptline at $bin_dir/promptline"

log "registering marketplace source $marketplace_source"
"$codex_bin" plugin marketplace add "$marketplace_source" --json >/dev/null
log "installing and enabling $PLUGIN_NAME@$MARKETPLACE_NAME"
"$codex_bin" plugin add "$PLUGIN_NAME@$MARKETPLACE_NAME" --json >/dev/null

plugin_state=$("$codex_bin" plugin list --marketplace "$MARKETPLACE_NAME" --json)
grep -Fq "\"pluginId\": \"$PLUGIN_NAME@$MARKETPLACE_NAME\"" <<<"$plugin_state" ||
  die "Codex did not report $PLUGIN_NAME@$MARKETPLACE_NAME as installed"
grep -Eq '"enabled"[[:space:]]*:[[:space:]]*true' <<<"$plugin_state" ||
  die "Codex did not report $PLUGIN_NAME@$MARKETPLACE_NAME as enabled"

case :$PATH: in
  *:"$bin_dir":*) ;;
  *) log "add this directory to PATH before starting Codex: $bin_dir" ;;
esac

log "$PLUGIN_NAME@$MARKETPLACE_NAME is installed and enabled"
log 'start a new Codex conversation to load the plugin'
