# Provenance

This repository is a synthesis. Its wording and organization were newly written for a Debian-specific operating model. No source is vendored. Upstream commands and procedures were treated as review material, checked against Debian/tool semantics, and either rewritten or rejected.

## Upstream snapshots

| Repository | Origin | Commit | License found |
|---|---|---|---|
| linux-skills | `https://github.com/peterbamuhigire/linux-skills.git` | `54184b67cd736558ad08f279dc472a9c901cec2e` | No `LICENSE` or `COPYING` file found in the cloned snapshot. No expressive material was copied; only unprotectable operational ideas/facts were independently rewritten. |
| linux-sysadmin-skills | `https://github.com/HermeticOrmus/linux-sysadmin-skills.git` | `358769e5e43197609b093a354f182cc6ad1118c2` | MIT, copyright 2026 Diego Bodart. |
| awesome-copilot | `https://github.com/github/awesome-copilot.git` | `d0d9d9f014abb27bf0d8321851867500a3a46bba` | MIT, copyright GitHub, Inc. |
| DevOps-Security-Agent-Skills | `https://github.com/BagelHole/DevOps-Security-Agent-Skills.git` | `0365f57a079b1332f95cf26e31dd2d5332a8399f` | MIT, copyright 2026 Toby Miller. |

MIT requires preservation of its copyright and permission notice in copies or substantial portions. This synthesis does not reproduce substantial source text, but this file retains attribution and repository/commit identity. Consult each upstream repository for later license changes.

## Concept map

| Synthesized concept | Main upstream influence | Result |
|---|---|---|
| Routing from a compact hub to specialist detail | linux-skills `linux-sysadmin` and specialist references | Reworked into one skill with conditional references/playbooks; removed numeric menu and external engine dependencies. |
| Evidence-first diagnosis and explicit confirmation | linux-sysadmin-skills diagnose/monitor/maintain and examples | Strengthened into hypothesis, rollback, one-change and functional-verification loop. |
| Debian triage using apt/dpkg/systemd/AppArmor awareness | awesome-copilot `debian-linux-triage` | Retained as concise activation vocabulary; expanded with Debian mechanisms and safety. |
| systemd dependency/status/journal/drop-in practices | linux-skills service management; DevOps systemd-services | Rewritten with vendor/local path distinction and reload/restart/enable/mask semantics. |
| Performance bottleneck classification and before/after evidence | both sysadmin performance sources; linux-skills perf/sysctl; DevOps performance-tuning | Retained; generic sysctl recipes rejected. |
| RPO/RTO, restore testing and application validation | linux-skills backup/recovery; DevOps backup-recovery, database-backups, disaster-recovery | Rewritten into four backup states and isolated restore evidence. Cloud-specific automation omitted. |
| Remote access and firewall precautions | linux-skills access/firewall/networking; DevOps SSH/firewall | Rebuilt around current-session preservation, independent connection tests, manager discovery and timed rollback. Unsafe flush/default-drop examples rejected. |
| Shell defensive patterns | linux-skills bash-scripting | Rewritten without repository-specific helper framework or mechanical strict-mode rule. |

Detailed per-file decisions are in [UPSTREAM-REVIEW.md](UPSTREAM-REVIEW.md).
