# Design

## Objective

Produce one operational Debian administration skill, not a catalog of commands or a bundle of competing routers. The skill should change an agent's decisions under uncertainty and privilege: inspect before assuming, isolate the responsible subsystem, make one reversible delta, preserve remote access, and verify function.

## Structure

`SKILL.md` contains the activation boundary, common operating loop, risk/authority rules, invariants, routing and stop conditions. Subsystem references are loaded only when their decisions apply. Playbooks describe incident sequences and link back to the authoritative subsystem rules. This keeps activation unambiguous while avoiding a monolithic entrypoint.

No remediation helpers are bundled. System administration commands depend too heavily on live state, authority and access topology for a generic “safe fix” wrapper. The scripts are validation-only: one audits repository structure and instruction safety, one checks the Skill Creator frontmatter contract, and one compares the documented toolbox catalog with a live isolated Promptline MCP server.

## Opinionated choices

- Debian stable/oldstable first; testing must be explicit. Ubuntu conventions are not defaults.
- Detect active owners for networking, DNS, firewall, scheduled work and generated configuration.
- Treat APT/dpkg as a subsystem with provenance, conffiles, pins, holds, triggers and recovery, not just install/remove commands.
- Prefer nftables reasoning while respecting dynamic netfilter owners such as container runtimes.
- Prefer Promptline's structured `toolbox` MCP for every supported direct Unix operation, but only when the toolbox runs on the managed host; never substitute controller-side evidence for remote-target state.
- Treat remote survivability and rollback as invariants, not closing checkboxes.
- Require confirmation at the final target-resolved boundary for destructive storage and irreversible identity changes.
- Prefer effective state plus application tests over exit codes.

## Authority boundary

The skill may autonomously gather scoped read-only evidence. Mutation requires the user's task to authorize mutation, and riskier actions add more immediate validation/confirmation. The risk table guides judgment but does not convert a diagnostic request into permission to repair.

## Authoritative checks

Questionable upstream claims were checked against installed manuals for `sources.list(5)`, `apt-key(8)`, `apt-get(8)`, `dpkg(1)`, `interfaces(5)`, `resolv.conf(5)`, `sshd(8)`, `sshd_config(5)`, and nftables tooling/package layout. The resulting rules include deb822 source support, scoped `Signed-By` keyrings, no `apt-key`, ifupdown as a real Debian mechanism, generated resolver awareness, `sshd -t`/`-T`, and inspection of `/etc/nftables.conf` plus the installed service rather than assumed persistence.

Systemd guidance follows upstream systemd semantics: local administrator units/drop-ins in `/etc/systemd/system`, vendor units in the vendor load path (`/usr/lib/systemd/system`, with Debian compatibility layouts possible), and clear separation of daemon reload, service reload/restart, enablement and masking. Always inspect the target release and installed unit paths.

## Known boundary

The toolbox catalog check exercises Promptline's MCP protocol but does not execute administration procedures against a live Debian host. Release-specific package and service behavior still must be verified on the actual host. Application-specific recovery, database consistency, cloud firewall layers, Secure Boot signing, exotic storage, and formal incident response require authoritative specialist material when they become the primary task.
