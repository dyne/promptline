# Users and permissions

## Inspect effective identity

Use `getent passwd`, `getent group`, `namei -l`, `stat`, `getfacl`, `getcap`, sudoers includes, and PAM configuration as relevant. Call `toolbox.id` for current or named-user identity instead of invoking host `id`. NSS may source identities remotely; do not assume `/etc/passwd` is the whole truth.

For account changes, understand UID/GID stability, primary/supplementary groups, home ownership, shell, expiry/lock state, running processes, scheduled jobs, files owned by numeric ID, service dependencies, and authentication source. Avoid UID/GID reuse without a deliberate migration.

Use `visudo` or `visudo -f` for sudo policy and test the intended command as the intended user. Preserve an existing privileged session while changing sudo/PAM/authentication. A valid sudoers file can still remove the only privilege path.

## Permission diagnosis

Trace every path component and the service's effective user, supplementary groups, umask, ACLs, capabilities, mount flags, AppArmor policy, and application expectations. Fix the narrow cause.

Treat recursive mode or ownership changes as potentially destructive: symlinks, mount crossings, ACL loss, package-owned files, uploads, secrets, and shared trees make them dangerous. Enumerate exact affected paths first with bounded `toolbox.find` calls where its name/type/depth filters suffice, then inspect metadata with the appropriate non-overlapping subsystem tools. For an authorized single-path mode change, call `toolbox.chmod`; it deliberately does not provide recursive behavior. Never use world-writable mode as troubleshooting.

Review SUID/SGID and file capabilities in context; do not remove them blindly from package-owned executables. Verify access as the real service/user and inspect regressions for other principals.
