# Networking

## Identify ownership

Debian does not imply Netplan or NetworkManager. Inspect active units, packages, links, and configuration for ifupdown (`/etc/network/interfaces` and `interfaces.d`), systemd-networkd, NetworkManager, cloud-init/provider agents, container runtimes, and orchestration. Do not configure the same interface in two managers.

```sh
ip -details -statistics link show
ip -brief address show
ip route show table all
ip rule show
ip neigh show
ss -lntup
networkctl status 2>/dev/null
nmcli general status 2>/dev/null
```

Trace failures layer by layer: link/carrier → address/prefix → local route and policy rules → neighbor/gateway → path/MTU → firewall → listener → application protocol → DNS only when names differ from direct-address tests. Use `ip route get DEST` to see the selected path. Check namespaces, VRFs, bridges, VLANs, bonds, WireGuard, and container networks when visible.

## Remote address/route changes

Classify as DISRUPTIVE. Identify the current SSH source/destination, ingress interface, selected return route, management network, persistent config owner, and console. Preserve the old address/route during transition when valid; schedule an independently recoverable rollback and test a new SSH connection through the new path before cancelling it. Do not use a blind `ifdown`/`ifup`, manager restart, or route flush remotely.

For MTU, compare interface, tunnel and path behavior using bounded probes; do not lower MTU globally on speculation. For connectivity, distinguish local listener binding, remote rejection, timeout, asymmetric routing, and filtering.

## Verification

Verify link/address/route/rule state, chosen path both directions where possible, neighbor resolution, required DNS, required port/application behavior, firewall counters, and persistence after the owning manager re-reads config. Reboot testing requires explicit authority and a recovery path.
