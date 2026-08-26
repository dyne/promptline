# Upstream review

## Method and license handling

The four cloned repositories were inventoried at the commits recorded in [PROVENANCE.md](PROVENANCE.md). Relevant `SKILL.md` files, linked specialist references, examples, licenses and unsafe-pattern matches were reviewed. “Incorporated” means the operational concept was independently rewritten and reconciled with Debian/tool semantics; it does not mean text was copied.

License codes below: **NL** = no license file found in the cloned `linux-skills` snapshot; **MIT-DB** = MIT, Diego Bodart; **MIT-GH** = MIT, GitHub, Inc.; **MIT-TM** = MIT, Toby Miller. NL material contributed only general facts/ideas and path-level provenance.

## linux-skills

Repository-wide overlap is high: every specialist repeats contracts, distro matrices, helper scripts and routing. The strongest shared ideas are read-only diagnosis by default, explicit inputs/authority, recovery/rollback evidence, distro detection, and acceptance criteria beyond exit status. The weakest shared material is generic dual-distro boilerplate and reliance on repository-specific `sk-*` tools. Debian correctness is uneven because Debian and Ubuntu are repeatedly combined.

| Path | License | Purpose | Strong ideas retained | Weak/questionable; Debian correctness and overlap | Incorporated / provenance |
|---|---|---|---|---|---|
| `linux-sysadmin/SKILL.md` | NL | Routing hub | Scope routing; diagnosis is not repair; verify outcomes | Numeric menu, external control-plane/Kaizen dependencies, duplicated contracts; distro-generic | Yes: compact router and evidence boundary, rewritten in `SKILL.md` |
| `01-provisioning-and-bootstrap/linux-package-management/SKILL.md` and `references/apt-reference.md` | NL | Package lifecycle | Policy/origin, holds, file ownership, dependency recovery, unattended-upgrade evidence | Groups Debian with Ubuntu; calls PPAs “extra repos”; snap/AppImage emphasis; routine maintenance/autoremove needs more caution | Yes: `references/apt-dpkg.md`; PPA/snap material rejected |
| `02-users-access-and-secrets/linux-access-control/SKILL.md` | NL | Accounts, keys, permissions | Discover identity; key permissions; confirmation for lockout-prone changes | Shallow PAM/sudo/ACL/capability treatment; overlaps hardening | Yes: `users-permissions.md`, expanded and Debian-neutral |
| `03-networking-and-dns/linux-network-admin/SKILL.md` and diagnostic reference | NL | Host networking | Layered diagnosis, modern `ip`/`ss`, timed rollback idea | Incorrectly says Debian/Ubuntu use Netplan; overweights RHEL NetworkManager; Debian may use ifupdown/networkd/NM/cloud owners | Yes: diagnostic tree; Netplan default explicitly rejected |
| `03-networking-and-dns/linux-dns-server/SKILL.md` | NL | Authoritative DNS | Validate zones before reload; query/serial evidence | Primarily BIND server administration, not local resolver diagnosis; overlaps only partially | Partial: validation pattern; server setup omitted |
| `05-services-and-virtualization/linux-service-management/SKILL.md` and references | NL | systemd services/scheduling | Status+journal+dependencies, config validation, timers, cgroup controls, restart verification | Product-specific Node examples and optional helper tools; direct details broader than needed | Yes: `systemd.md` and service playbook |
| `06-storage-and-filesystems/linux-disk-storage/SKILL.md` and references | NL | Usage/mount diagnosis | Bytes vs inodes, deleted-open files, safe cleanup order, stable mapping | Network-share/product examples; insufficient final target confirmation for mkfs/LVM/RAID/LUKS | Yes: diagnosis retained; destructive gate strengthened |
| `07-security-and-hardening/linux-firewall-ssl/SKILL.md` and `nftables-and-iptables.md` | NL | Firewall/TLS | Inspect live rules, nftables awareness, persistence concern | UFW receives first-class/default treatment; mixed SSL scope; container interactions incomplete | Partial: nftables reasoning only; UFW default rejected |
| `07-security-and-hardening/linux-security-analysis/SKILL.md` | NL | Security audit | Evidence/severity/report structure; distinguish analysis from hardening | Generic layer checklist, web-stack bias, overlaps monitor/hardening | Partial: practical evidence categories, no fixed “10 layers” |
| `07-security-and-hardening/linux-server-hardening/SKILL.md` | NL | Host hardening | Audit before change; verify after hardening | UFW-first, generic sysctl list, web-stack assumptions; risk of benchmark cargo cult | Partial: workload-aware principle; prescriptive recipes rejected |
| `08-observability-and-logging/linux-log-management/SKILL.md` | NL | Journald/logrotate/log review | Boot/unit/time filters; bounded evidence; rotation awareness | Nginx/fail2ban bias; monitoring overlap | Yes: diagnostic log workflow, specialized product recipes omitted |
| `08-observability-and-logging/linux-observability/SKILL.md` | NL | Telemetry deployment | Define evidence/acceptance and rollback | Direct GitHub binary installation labeled canonical, third-party agent sprawl, web/PHP examples | No substantial material; supply-chain pattern rejected |
| `08-observability-and-logging/linux-system-monitoring/SKILL.md` | NL | Health checks | CPU/memory/I/O/network/backup evidence categories | Checklist is broad and duplicates diagnose/performance; no deep pressure semantics | Partial: bounded baseline and backup evidence distinction |
| `09-troubleshooting-and-recovery/linux-troubleshooting/SKILL.md` and diagnosis reference | NL | Incident triage | Symptom tree, time correlation, narrow hypothesis, packet capture bounds | Broad overlap with all specialists; optional helper dependence | Yes: central operating loop and playbooks |
| `09-troubleshooting-and-recovery/linux-disaster-recovery/SKILL.md` and references | NL | Restore workflow | Stabilize, RPO/RTO, qualify backup, isolated restore, preserve target, app validation | Some examples are web/database specific; a running shell is over-assumed | Yes: backup/recovery principles with explicit console/rescue limits |
| `10-automation-and-scripting/linux-bash-scripting/SKILL.md` and references | NL | Defensive shell | Quoting, validation, temp files, atomic writes, confirmation, cleanup | Repository-specific `common.sh` contract and flags; strict-mode advice needs contextual nuance | Yes: `shell-safety.md`; helper framework omitted |
| `13-backup-and-archiving/linux-rsync-sync/SKILL.md` | NL | rsync transfer | Dry-run/change review, metadata and restore validation | `rsync -a` is not backup design; deletion/mirror semantics need stronger target safeguards | Partial: verification/metadata concepts only |
| `13-backup-and-archiving/linux-archive-integrity/SKILL.md` | NL | Archive integrity | Checksums, ACL/xattr/numeric-owner awareness, test extraction | Archive creation can be mistaken for a backup; GPG/key lifecycle shallow | Partial: restore metadata/integrity checks |
| `13-backup-and-archiving/linux-filesystem-snapshots/SKILL.md` | NL | Snapshot lifecycle | Stack discovery, capacity and rollback considerations | Snapshot is not off-host backup; filesystem-specific operations too broad | Partial: failure-domain distinction only |
| `14-performance-and-kernel/linux-perf-profiling/SKILL.md` and reference | NL | Bottleneck profiling | `vmstat` classification, drill-down, bounded profiler overhead | Tool availability/overhead varies; no application context by default | Yes: `performance.md` and high-load playbook |
| `14-performance-and-kernel/linux-sysctl-tuning/SKILL.md` and reference | NL | Kernel tuning | Change only a measured key; comparable before/after; rollback failed experiments | Large generic tuning catalog risks cargo cult despite safeguards | Partial: experiment contract retained; recipes rejected |

Related router specialists for databases, web/mail, containers, provisioning, virtualization, compliance and RHEL-specific behavior were inventoried but excluded because they do not materially improve the Debian host-administration core; application specialists should remain separate.

## linux-sysadmin-skills

| Path | License | Purpose | Strong ideas retained | Weak/questionable; Debian correctness and overlap | Incorporated / provenance |
|---|---|---|---|---|---|
| `skills/sysadmin-diagnose/SKILL.md` | MIT-DB | General diagnosis | Establish baseline, correlate services/logs/network/storage | Checklist is shallow and overlaps monitor; limited rollback/risk | Yes: evidence loop, rewritten and deepened |
| `skills/sysadmin-monitor/SKILL.md` | MIT-DB | Routine monitoring | Resource/service/network/backup sweep | Checklist duplicates diagnose/performance; “healthy” criteria underdefined | Partial: bounded observations and backup proof |
| `skills/sysadmin-performance/SKILL.md` | MIT-DB | Performance analysis | Measure before tuning; name bottleneck; before/after comparison | Generic optimization/sysctl suggestions risk action without workload evidence | Yes: classification retained, tuning recipes rejected |
| `skills/sysadmin-security/SKILL.md` | MIT-DB | Security audit | Access, exposure, patch, logs and restore checklist | Distribution-neutral, benchmark-like, shallow AppArmor/repository provenance | Partial: evidence categories only |
| `skills/sysadmin-maintain/SKILL.md` | MIT-DB | Routine maintenance | Inspect upgrades, logs, disk and backups; ask before removal | `apt autoremove`/cache cleaning normalized; package impact and service verification weak | Partial: maintenance evidence; routine autoremove rejected |
| `EXAMPLES.md` | MIT-DB | Worked scenarios | Inspect logs before service repair; confirmation and before/after evidence | Maintenance example proceeds from upgrade to autoremove too readily; command success sometimes substitutes for function | Partial: scenario shape retained, unsafe normalization corrected |

## awesome-copilot

The `skills/` tree was searched for Debian, Linux, shell, systemd, SSH, firewall, networking, DNS, backup, Unix, security and performance terms. Arch/CentOS/Fedora triage and unrelated application/security-review matches were not imported.

| Path | License | Purpose | Strong ideas retained | Weak/questionable; Debian correctness and overlap | Incorporated / provenance |
|---|---|---|---|---|---|
| `skills/debian-linux-triage/SKILL.md` | MIT-GH | Short Debian incident router | Confirm release; apt/dpkg/systemd; verification and rollback; AppArmor/firewall awareness | Too short for privileged operation; asks questions where shell discovery is possible; no remote/storage/package depth | Yes: activation vocabulary and concise routing, substantially expanded |

## DevOps-Security-Agent-Skills

| Path | License | Purpose | Strong ideas retained | Weak/questionable; Debian correctness and overlap | Incorporated / provenance |
|---|---|---|---|---|---|
| `infrastructure/servers/linux-administration/SKILL.md` | MIT-TM | Generic host administration | Broad subsystem inventory, remove vs purge, timers | Uses Ubuntu package versions/repository URL under Debian heading; `apt ... -y`, routine `autoremove -y`, global examples; shallow rollback | Partial: scope inventory only; commands rejected/rewritten |
| `infrastructure/servers/systemd-services/SKILL.md` | MIT-TM | Unit authoring/operation | Dependencies/order, drop-ins, restart limits, sandboxing, `visudo` validation | Large “complete unit” invites wholesale copying; application assumptions; hardening can break workloads | Yes: semantic concepts, minimal-edit policy added |
| `infrastructure/servers/ssh-configuration/SKILL.md` | MIT-TM | Client/server SSH | `sshd -t`, keys, bastions, forwarding, config patterns | Hardening recipes can disable access; remote survivability and `sshd -T -C` insufficiently central | Yes: expanded access-safe procedure |
| `infrastructure/servers/user-management/SKILL.md` | MIT-TM | Users/groups/sudo/PAM | `visudo`, account lifecycle, permissions | Generic password policy; recursive ownership examples need target analysis; NSS/UID migration shallow | Partial: targeted identity/permission method |
| `infrastructure/servers/performance-tuning/SKILL.md` | MIT-TM | Performance/sysctl recipes | Measurement tools and bottleneck categories | Extensive blanket sysctl/limits/THP/network recipes; risks unmeasured tuning | Partial: observation only; recipes rejected |
| `security/hardening/linux-hardening/SKILL.md` | MIT-TM | Prescriptive hardening | Patch/exposure/SSH/firewall/permissions checklist | Ubuntu/UFW bias, benchmark-like settings, potential workload/access disruption | Partial: audit categories, not commands |
| `security/network/firewall-config/SKILL.md` | MIT-TM | iptables/nftables/cloud firewalls | Established-flow rule, management-source scoping, counters/audit intent | Starts by flushing tables and setting DROP; remote lockout risk; flawed DNS source-port rules; simplistic anti-DDoS; advises removing iptables packages/turning off Docker management; UFW emphasis | Only high-level intent; example procedures explicitly rejected |
| `infrastructure/networking/dns-management/SKILL.md` | MIT-TM | Provider/authoritative DNS | Record/TTL/propagation validation concepts | Cloud/API focused, not Debian local resolver; overlaps little | Minimal: distinguish authoritative from local resolver |
| `infrastructure/storage/backup-recovery/SKILL.md` | MIT-TM | File backup/restore | Retention, encryption, verification and restore testing | Tool/cloud examples can imply configured equals safe; consistency/key custody limited | Yes: four backup-state model and restore gates |
| `infrastructure/databases/database-backups/SKILL.md` | MIT-TM | Database backups | Engine-native tools, PITR awareness, automated isolated restore tests, RPO/RTO | Hard-coded example credentials, `--clean` restore risk, Docker/AWS assumptions; database-specific scope | Partial: application-level restore evidence; commands omitted |
| `compliance/continuity/disaster-recovery/SKILL.md` | MIT-TM | RPO/RTO and failover | Business-defined RPO/RTO, measured exercises, failback planning | Mostly AWS automation/compliance cadence; failover script declares success too early | Partial: RPO/RTO/testing/failback concepts only |

## Conflicts resolved

| Topic | Conflict found | Debian-conservative resolution |
|---|---|---|
| APT signing | Vendor examples use Ubuntu repos and key downloads; older/global trust patterns appear across generic material | Verify vendor/release/fingerprint independently; operator key in `/etc/apt/keyrings`, package key in `/usr/share/keyrings`, source-scoped `Signed-By`; no `apt-key`, trust bypass, or pipe-to-shell. |
| Debian networking | linux-skills states Debian/Ubuntu use Netplan; other sources assume NetworkManager | Discover ifupdown, networkd, NetworkManager, cloud/provider or other owner. Netplan is not a Debian default assumption. |
| Firewall | Sources prioritize UFW or flush live iptables before default drop | Discover live owner; nftables-first Debian reasoning; never flush to add a rule; preserve remote path and dynamic container rules; validate and arrange rollback. |
| systemd paths | Sources alternate `/lib`, `/usr/lib`, and direct edits | Inspect `FragmentPath`; vendor files stay vendor-owned, local units/drop-ins go in `/etc/systemd/system`; use daemon-reload and restart/reload according to semantics. |
| SSH apply | Syntax checks are sometimes treated as sufficient | Use `sshd -t`, effective `sshd -T -C`, preserve session, test replacement auth and fresh independent connection before retiring old access. |
| Package recovery | Examples normalize `autoremove`, forceful or broad repair sequences | Inspect transaction and root cause; no lock/database deletion or force; configure interrupted dpkg only when idle, repair dependencies only when indicated, simulate removals. |
| Performance | Large preset sysctl catalogs conflict with measurement-first language | Retain only hypothesis-driven, one-variable, reversible experiments with equivalent before/after load and guardrails. |
| Backup | Archive/snapshot/software presence can be presented as backup | Require configured, running, successful and restorable evidence separately; test isolated restore and application correctness. |

## Notable rejected advice

- Any `curl | sh`, `apt-key`, PPA-as-Debian-default, or Ubuntu repository example for Debian.
- `apt upgrade -y` and `apt autoremove -y` as routine production maintenance.
- Flushing iptables/nftables, setting default DROP, enabling UFW, disabling Docker netfilter management, or removing compatibility packages without mapping the active ruleset and remote path.
- Blanket sysctl, THP, limits, anti-DDoS, SSH-hardening, CIS, or password-policy commands without workload/access evidence.
- `kill -9`, reboot, firewall/AppArmor disabling, log deletion, package-database deletion, force-dpkg, recursive permission changes, or destructive storage commands as first responses.
- Installing optional telemetry/management agents directly from GitHub releases as a canonical host-administration path.
