# nftables and netfilter

## Inspect ownership and live state

```sh
systemctl status nftables --no-pager
nft list ruleset
nft list ruleset -a
dpkg-query -W nftables iptables 2>/dev/null
```

Determine whether rules are owned by `/etc/nftables.conf` and `nftables.service`, a framework, configuration management, Docker/Podman, libvirt, a cloud agent, or compatibility iptables tooling. Do not edit generated chains or assume the saved file equals the live ruleset. UFW is optional software, not the Debian firewall model.

## Safe change procedure

1. Map current SSH connection, management address/interface, allowed source ranges, input hook priorities and established-flow handling.
2. Save a deliberate rollback artifact and arrange timed rollback or independent console access for a remote change.
3. Build a complete candidate ruleset or narrow transaction and inspect the diff. Validate syntax with `nft --check --file FILE`; understand that syntax validation cannot prove connectivity.
4. Preserve container-runtime and other dynamic rules. Avoid flushing the entire live ruleset to add one port.
5. Apply only after authorization, then verify rules, counters, required new flow, current/fresh SSH access, and unintended exposure. Cancel rollback only after independent success.

For port 443, first verify the service is expected, bound to the intended addresses, and that another firewall layer (provider/security group/router) is not the real blocker. Add the narrowest family/interface/source/destination rule consistent with the policy; opening a port without a listener does not provide a service.

Debian's nftables package convention commonly loads `/etc/nftables.conf` through `nftables.service`, but inspect the installed unit and local configuration on the actual release. Persistence is a separate verification step.
