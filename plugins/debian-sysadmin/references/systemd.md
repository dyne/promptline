# systemd

## Discover and diagnose

```sh
systemctl status UNIT --no-pager --full
systemctl show UNIT
systemctl cat UNIT
systemctl list-dependencies UNIT
systemctl list-dependencies --reverse UNIT
journalctl -u UNIT -b --since 'TIME' --no-pager
systemctl --failed
```

Use `systemctl list-unit-files`, `list-units --all`, and `show -p FragmentPath -p DropInPaths` to distinguish installed definitions, loaded state, aliases, generated units, and drop-ins. Inspect dependencies and ordering; `After=` orders but does not pull a unit in, while `Wants=`/`Requires=` express dependencies with different strength.

## Change units safely

- Do not edit packaged units under `/usr/lib/systemd/system` or Debian's compatibility path `/lib/systemd/system`. Use `systemctl edit UNIT` or a deliberate unit/drop-in under `/etc/systemd/system`.
- Use `systemd-analyze verify FILE...` where applicable and inspect `systemctl cat` after editing.
- `systemctl daemon-reload` reloads manager definitions; it does not restart a service or apply every changed runtime setting.
- `reload` asks a service to reload when supported; `restart` stops/starts it. Inspect the journal and validate application config before either.
- `enable` configures activation links; it does not mean the unit is currently running unless `--now` is explicitly used. `disable` removes enablement; `mask` prevents activation and is stronger.
- Prefer restart policies and resource controls only after understanding failure mode and workload. Avoid restart loops that hide crashes.

Inspect timers with `systemctl list-timers --all` and their paired service units. Inspect socket activation with `systemctl status NAME.socket` and `ss`. Before adding scheduled work, search system/user timers, `/etc/cron*`, user crontabs, and package-owned schedules.

## Boot analysis

Use `journalctl -b`, `journalctl -b -1`, `systemd-analyze time`, `critical-chain`, and `blame` as evidence. `blame` reports activation time and is not proof of root cause. Identify the boot ID and persistent-journal availability before assuming a previous boot exists.

## Verification

After a unit change, verify loaded fragment/drop-ins, active/sub states, process and cgroup properties, recent journal, dependencies/failed units, listener or application health, and behavior across the relevant activation path (manual, boot, timer, or socket).
