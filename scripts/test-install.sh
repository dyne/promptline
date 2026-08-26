#!/usr/bin/env bash
set -Eeuo pipefail
IFS=$'\n\t'

repository_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/promptline-install-test.XXXXXXXX")

cleanup() {
  rm -rf -- "$test_root"
}
trap cleanup EXIT

fake_bin=$test_root/fake-bin
install_bin=$test_root/install-bin
codex_log=$test_root/codex.log
mkdir -p -- "$fake_bin"

cat > "$fake_bin/go" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
output=''
while (($# > 0)); do
  if [[ $1 == -o && $# -ge 2 ]]; then
    output=$2
    shift
  fi
  shift
done
[[ -n $output ]]
printf '#!/usr/bin/env bash\nprintf "promptline test binary\\n"\n' > "$output"
chmod 0755 "$output"
EOF

cat > "$fake_bin/codex" <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
printf '%s\n' "$*" >> "${PROMPTLINE_TEST_CODEX_LOG:?}"
case $* in
  'plugin list --marketplace promptline --json')
    printf '{"installed":[{"pluginId": "sysadmin@promptline", "enabled": true}]}\n'
    ;;
  *) printf '{}\n' ;;
esac
EOF

chmod 0755 "$fake_bin/go" "$fake_bin/codex"

PATH="$fake_bin:$PATH" \
  GO=go \
  CODEX=codex \
  PROMPTLINE_TEST_CODEX_LOG="$codex_log" \
  "$repository_root/install.sh" --bin-dir "$install_bin"

[[ -x $install_bin/promptline ]]
[[ $("$install_bin/promptline") == 'promptline test binary' ]]
grep -Fq "plugin marketplace add $repository_root --json" "$codex_log"
grep -Fq 'plugin add sysadmin@promptline --json' "$codex_log"
grep -Fq 'plugin list --marketplace promptline --json' "$codex_log"

before_count=$(wc -l < "$codex_log")
PATH="$fake_bin:$PATH" \
  GO=go \
  CODEX=codex \
  PROMPTLINE_TEST_CODEX_LOG="$codex_log" \
  "$repository_root/install.sh" --bin-dir "$install_bin"
after_count=$(wc -l < "$codex_log")
((after_count == before_count + 3))

PATH="$fake_bin:$PATH" \
  GO=go \
  CODEX=codex \
  "$repository_root/install.sh" --bin-dir "$install_bin/dry-run" --dry-run
[[ ! -e $install_bin/dry-run/promptline ]]

if "$repository_root/install.sh" --bin-dir / --dry-run >/dev/null 2>&1; then
  printf 'unsafe binary directory unexpectedly accepted\n' >&2
  exit 1
fi
if "$repository_root/install.sh" --unknown-option >/dev/null 2>&1; then
  printf 'unknown option unexpectedly accepted\n' >&2
  exit 1
fi

printf 'installer contract tests passed\n'
