# Networking failure

1. Preserve the remote session. Identify namespace, link, address, selected route/rules, neighbor, management path and persistent configuration owner.
2. Test layer by layer: local bind/link → gateway/route → destination IP → destination port → application protocol → name resolution. Inspect nftables and provider/container layers.
3. Compare `ip route get` and return-path expectations; check MTU, policy routing, VPN, bridge/VLAN/bond, and recent manager/cloud changes as evidence dictates.
4. For remote address/route/firewall changes, prepare timed rollback or console recovery, preserve old access, and apply one bounded delta.
5. Verify a fresh independent SSH connection plus required bidirectional/application behavior and persistence before cancelling rollback.
