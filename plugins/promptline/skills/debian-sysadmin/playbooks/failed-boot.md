# Failed boot

1. Obtain console/hypervisor/physical access and capture the exact last successful stage and messages. Try a known-good boot entry or recovery target without overwriting evidence.
2. In rescue, identify and map the correct disks, encryption, RAID, LVM, root, `/boot`, and EFI filesystems. Mount read-only for initial inspection where practical.
3. Branch on evidence: fstab/UUID/mount failure; filesystem/device error; initramfs/root-stack issue; kernel package; bootloader/EFI; systemd unit/target.
4. Back up current metadata/config, make one offline repair with its rollback and authoritative procedure, then validate generated artifacts and mount identities.
5. Boot with console attached. Verify target, filesystems, network/SSH, failed units and kernel journal. Keep a known-good kernel/recovery path until the repaired boot is proven.
