# Logs, processes, and diagnosis

## Time-scoped evidence

Use the smallest useful slice:

```sh
journalctl --no-pager -b -p warning..alert --since 'TIME' --until 'TIME'
journalctl --no-pager -u UNIT -b --since 'TIME'
journalctl --no-pager -k -b
journalctl --no-pager -b -1
dmesg --ctime --level=err,warn
coredumpctl list
```

Confirm clock/time zone and boot identity when correlating sources. Inspect application logs and logrotate/journald persistence as configured. Do not delete or truncate logs to diagnose; if storage is critical, preserve the relevant window and use supported vacuum/rotation only with authorization.

## Processes and signals

Start process discovery with `toolbox.ps` using a bounded `limit` and optional `name`; do not invoke host `ps`. For a selected numeric PID, use `toolbox.cat` with `/proc/PID/status`. The toolbox process view is intentionally narrow, so use the subsystem-native commands below only for details it does not expose:

```sh
systemctl status UNIT --no-pager --full
lsof -p PID
```

Prefer service-manager operations for service-managed processes. Use the least forceful appropriate signal, allow the documented timeout, and verify replacement state. `SIGKILL` prevents cleanup and is a last resort after understanding why normal termination failed.

- Zombie: already exited; fix/restart the parent that has not reaped it, not the zombie.
- `D` state: uninterruptible kernel wait; investigate storage/NFS/device/kernel evidence. Signals may not take effect.
- OOM: correlate kernel messages, cgroup limits, memory pressure and victim selection; do not infer from a vanished process alone.
- Manually launched process: identify why it is outside service management before killing or wrapping it.

## Incident boundary

If compromise is plausible, avoid destroying volatile evidence or executing untrusted binaries. Record times, processes, connections, identities and package provenance; escalate to an incident-response procedure when isolation, imaging, credential rotation, or legal handling is required.
