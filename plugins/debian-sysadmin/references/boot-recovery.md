# Boot and recovery

## Establish the failure boundary

Determine the last visible stage: firmware/hypervisor, bootloader, kernel, initramfs, root mount, systemd default target, login, or application. Obtain console output and the exact boot entry/kernel. A normally running SSH shell cannot diagnose every failed-boot path.

From a rescue or alternate boot, map disks and encryption/RAID/LVM before mounting. Prefer read-only inspection initially. Confirm the installed system's `/etc/os-release`, `/etc/fstab`, bootloader, EFI/BIOS mode, kernels, initramfs images, and root UUIDs rather than applying generic GRUB commands.

## Common evidence

- systemd: `journalctl -b`, previous boot logs, failed units, emergency target messages.
- fstab: stable identifiers, `findmnt --verify`, missing devices, network mounts, malformed options.
- kernel/initramfs: installed `linux-image-*`, `/boot` capacity, `lsinitramfs`, module/firmware errors, root stack support.
- GRUB: generated configuration and `/etc/default/grub`; do not edit generated `grub.cfg` directly.
- package failure: dpkg audit/logs, incomplete kernel/initramfs/grub triggers, available known-good kernel.

Use rescue/emergency targets for bounded recovery. Chroot only after mounting the correct root and required `/dev`, `/proc`, `/sys`, `/run`, EFI and boot filesystems with understood semantics. DNS in a chroot may require deliberate resolver setup.

Before regenerating initramfs or bootloader configuration, ensure `/boot` and EFI filesystems are the intended mounted targets and have space. Preserve at least one known-good kernel until the replacement boots. Filesystem repair may require unmounting and offline tooling.

## Verification

Recovery is not complete until the intended boot entry reaches the target, required filesystems mount, networking/access works, failed units and kernel messages are reviewed, and the original failure cause is addressed. Reboot is DISRUPTIVE and requires a console/recovery path.
