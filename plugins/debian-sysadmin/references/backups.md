# Backups and restores

## Four distinct states

Do not call a system backed up because software is installed. Establish separately:

1. configured: intended sources, exclusions, destination, credentials, retention and encryption;
2. running: scheduler/job actually executes;
3. successful: recent job completed and artifacts/checks are valid;
4. restorable: a documented restore has been tested to an isolated target with application-level validation.

Determine RPO/RTO, consistency requirements, quiescing/snapshot semantics, off-host/offline protection, key custody, capacity, immutability, monitoring, and ownership. A filesystem copy of a live database may not be application-consistent. A snapshot on the same failure domain is not sufficient protection by itself.

Before restore, identify exact backup, timestamp relative to incident, chain/dependencies, checksum, decryptability, target mapping, overwrite/delete semantics, current-target preservation, downtime, and rollback. Test the smallest isolated restore first. Restore ownership, ACLs, xattrs, capabilities and application metadata where required; do not assume file extraction proves recovery.

Verification should include recent successful job evidence, artifact availability, integrity/decryption, an understood restore command/procedure, isolated test evidence, and application correctness. Record gaps plainly.
