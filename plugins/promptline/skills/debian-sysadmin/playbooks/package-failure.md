# Package failure

1. Capture the exact command/error and confirm no apt/dpkg process is active. Inspect space, inodes, read-only mounts, network/time, sources, policy, holds, and `dpkg --audit`.
2. Classify: repository/signature/suite error, dependency resolution, unpack conflict, maintainer-script failure, conffile question, interrupted configuration, trigger, or filesystem fault.
3. Fix the underlying condition. Never delete locks, `/var/lib/dpkg` state, or use force options as routine recovery.
4. When dpkg reports interruption and no transaction remains, run `dpkg --configure -a`; address its first concrete failure. Use `apt-get -f install` only when dependency repair is actually required and inspect the proposed transaction.
5. Verify dpkg audit, package state/version/origin, holds/pins, affected services and logs. Re-run only the originally intended operation.
