# OpenSSH

## Diagnose client and server

Use `ssh -vvv` with secrets redacted for client negotiation. Inspect client precedence (`ssh -G HOST`), proxy/bastion settings, host-key errors, address family, DNS, route, and server reachability separately.

On the server:

```sh
systemctl status ssh --no-pager
sshd -t
sshd -T
sshd -T -C user=USER,host=HOST,addr=CLIENT_IP
journalctl -u ssh -b --since 'TIME'
```

On Debian the service is commonly `ssh.service`; discover rather than guessing. Read `/etc/ssh/sshd_config` and included files in effective precedence order. `sshd -T` shows effective configuration; use `-C` for `Match` conditions. `sshd -t` validates keys/config syntax but not successful authentication.

Check directory/file ownership and modes along the complete `authorized_keys` path, the target user's shell/account state, PAM where enabled, key algorithm/policy, host keys, forwarding constraints, and `known_hosts` provenance. Never paste private keys into logs or reports.

## Access-affecting change

1. Inventory every currently working authentication/privilege path and the exact desired users/addresses.
2. Install and test the replacement mechanism in a fresh independent session before disabling the old one.
3. Make a minimal config/drop-in edit; validate with `sshd -t` and inspect effective values with `sshd -T -C ...`.
4. Preserve the current session and recovery path. Reload when sufficient; do not close the original session.
5. Establish a fresh connection, authenticate, elevate if required, and test bastion/automation paths before declaring success.

Do not disable password authentication when it is the only proven access method. Consider console, break-glass accounts, automation keys, `Match` blocks, keyboard-interactive/PAM behavior, and root-login policy. A syntax-valid lockout remains a lockout.
