# Storage

## Map the stack

```sh
lsblk -e7 -o NAME,KNAME,PATH,MAJ:MIN,SIZE,RO,TYPE,FSTYPE,FSVER,LABEL,UUID,PARTUUID,MOUNTPOINTS,MODEL,SERIAL
findmnt --real
blkid
pvs; vgs; lvs -a -o +devices
```

For each relevant mount, call `toolbox.df` with its path for bounded block-usage data, and call `toolbox.cat` with `/proc/mdstat` when software RAID status is relevant. Use `stat -f` for inode counters because `toolbox.df` does not expose them. Add `mdadm --detail`, `cryptsetup status`, `smartctl -x`, mount-unit/systemd dependency, provider volume, multipath, or device-mapper inspection only when relevant. Never identify a destructive target by device letter alone; device names can change.

## Disk-space diagnosis

Distinguish bytes, inodes, reserved blocks, quotas, thin-pool exhaustion, and deleted-but-open files (`lsof +L1`). Use bounded `toolbox.du` calls only after mapping mount boundaries; its schema does not implement native one-filesystem flags, so do not let a result spanning nested mounts drive deletion. Find the writer and retention policy before deleting data. Do not delete logs, package state, or unknown large files as a diagnostic shortcut.

## Mounts and fstab

Prefer UUID/PARTUUID or the stack's stable identifier. Before changing `/etc/fstab`, inspect current mount options and dependencies, validate with `findmnt --verify --verbose`, and test the specific mount without unmounting an access-critical filesystem. Understand `_netdev`, `nofail`, automounts, and boot impact; do not use `mount -a` blindly when unrelated entries may be unsafe.

## High-risk operations

Partitioning, `mkfs`, signature wiping, filesystem shrink/repair, LVM/RAID/LUKS mutation, and destructive recovery are DESTRUCTIVE or at least DISRUPTIVE. Immediately before the command:

- re-run identity and holder/mount checks;
- show stable identifiers, size/model/serial and full stack mapping;
- state what data becomes inaccessible or destroyed;
- confirm backup/restorability and console/rescue needs;
- obtain explicit confirmation naming the exact target and operation.

For LVM resize, determine PV/VG/LV, filesystem type, supported grow/shrink direction, free extents, underlying device capacity, snapshots/thin-pool state, mount state, and backup. Separate block-layer and filesystem operations unless a verified tool safely combines them. Verify sizes at every layer and inspect kernel logs.

Never run filesystem repair on a mounted writable filesystem unless the filesystem's authoritative procedure explicitly permits it. Some recovery requires rescue media or offline access.
