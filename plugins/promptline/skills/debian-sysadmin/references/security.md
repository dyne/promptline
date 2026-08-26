# Practical Debian security

## Evidence-led review

Inspect Debian release support, configured repositories, package/update state, exposed listeners and paths, SSH effective policy, firewall rules/ownership, privileged accounts, sudo/PAM, secrets permissions, scheduled jobs, AppArmor status when installed, kernel messages, and backup/restoration evidence.

Use Debian security tracker/advisories and package versions rather than generic version-string assumptions. Determine whether a fix is backported. Treat adding a repository or binary as a supply-chain decision; verify origin and signing scope.

Hardening is workload-specific. Establish assets, access paths, required services, compatibility constraints, threat model, maintenance window and recovery first. Do not translate CIS controls directly into commands or equate stricter with safer. Test AppArmor policy and service behavior; do not disable AppArmor globally to make an error disappear.

## Basic compromise indicators

Correlate unexpected accounts/keys/sudo rules, new listeners or outbound connections, altered package files/config, unusual persistence, authentication anomalies, suspicious processes, kernel/module messages, and missing/tampered logs. Each can have benign explanations; preserve evidence and timestamps.

If compromise is credible, avoid ordinary cleanup that destroys evidence. Escalate for containment, credential rotation from a trusted system, forensic capture, rebuild/restore decisions, and notification requirements. Do not trust the suspected host to prove its own cleanliness.

## Verification

After a security change, test required workload and access, effective policy, logs/denials, update provenance, firewall exposure, and recovery. Confirm backups are recent and restorable; installed backup software is not backup evidence.
