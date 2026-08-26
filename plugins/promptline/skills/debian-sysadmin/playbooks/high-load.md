# High load

1. Record time, workload impact, CPU count, uptime/load, bounded `vmstat`, PSI and process states.
2. If CPU is idle, look for `D`-state tasks, I/O wait/latency, NFS/device failures, locks, and cgroup throttling. Load is not CPU usage.
3. Branch by evidence: CPU → per-CPU/process/profile; memory → pressure/swap/OOM/cgroups; I/O → device latency/queue/filesystem/kernel; network → errors/retransmits/limits; contention → application locks and blocked stacks.
4. Stabilizing actions such as stopping a workload, renicing, changing cgroup limits, or restarting a service are DISRUPTIVE and need impact/rollback planning.
5. Compare equivalent before/after samples and verify application latency/errors. Do not apply generic sysctl tuning.
