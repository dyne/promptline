# Failed upgrade

1. Stabilize: do not reboot a partially upgraded remote host without console/recovery. Capture exact errors, dpkg/apt logs, release/sources, package state, space/inodes, mounts and failed units.
2. Identify the failing package/maintainer script/trigger or repository mismatch. Check mixed suites, third-party repositories, holds/pins, conffile decisions, `/boot` capacity and kernel/initramfs/grub failures.
3. Restore package consistency using the narrowest supported action from [APT and dpkg](../references/apt-dpkg.md). Do not force, delete database state, or indiscriminately downgrade.
4. Reconcile conffiles and validate affected application configuration before service restart.
5. Verify `dpkg --audit`, candidate/installed origins, services, kernel/initramfs/bootloader artifacts, access, and whether reboot is actually required. Reboot only with a known-good boot entry and recovery path.
