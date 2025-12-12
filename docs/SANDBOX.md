# Sandbox Security Model

This documents the Linux namespace sandbox used by Promptline, the isolation goals, and operational requirements.

## Goals
- Confine AI tool execution to the project workdir and a tmp area.
- Prevent access to sensitive host paths and symlink escapes.
- Provide deterministic runtime environment (binaries/libs) without host writes.
- Run as non-root inside the container to reduce impact if compromised.

## Model
- **Container runtime:** `github.com/criyle/go-sandbox` preforked container using user/mount/pid/uts/cgroup namespaces and tmpfs root.
- **Mount layout:**
  - tmpfs root; workdir bind mounted RW at `/w`.
  - tmpfs `/tmp`.
  - RO binds for `/bin`, `/usr`, `/lib`, `/lib64` (or configured `read_only_paths`).
  - Masked paths inherit defaults (`/proc/*` dangerous entries, `/sys/firmware`, `/usr/lib/wsl`, etc.) plus config overrides.
  - Root remounted RO after pivot.
- **Credentials:** default non-root via user namespace mapping (container uid/gid 1000 -> host user). Capabilities dropped; `no_new_privs` set.
- **Execution:** tools call `ExecInSandbox` (via manager) using `sh -c` for shell and direct syscalls for file ops. `MS_NOSYMFOLLOW` on binds where supported.
- **Path policy:** file tools validate paths, deny dangerous prefixes, and require targets to stay under the configured workdir.

## Requirements
- **Kernel:** Linux with unprivileged user namespaces and mount namespaces enabled; `MS_NOSYMFOLLOW` available (>=5.10) for symlink hardening; cgroup v2 optional.
- **File layout:** workdir must be writable by the mapped uid/gid; RO paths must exist on host to be bound.
- **Network:** loopback disabled unless explicitly enabled via `InitCommand` (currently not used).
- **Fallback:** if sandbox init fails or sandbox disabled in config, tools fall back to host execution (logged).

## Config keys
- `sandbox.enabled` (bool, default true) – turn sandbox on/off.
- `sandbox.workdir` (string) – host path bound RW to `/w`; defaults to process CWD.
- `sandbox.read_only_paths` ([]string) – additional host RO binds (defaults: /bin,/usr,/lib,/lib64).
- `sandbox.masked_paths` ([]string) – extra paths to bind-mask/null.
- `sandbox.non_root_user` (bool, default true) – use user-namespace uid/gid mapping instead of mapped root.

## Known limitations
- Non-Linux platforms stub out sandbox; tools run on host.
- If user namespaces are disabled by the host, sandbox creation will fail and fallback is used.
- No seccomp policy yet; rely on mount isolation and uid/gid mapping. Future work: add seccomp filters and cgroup accounting.
