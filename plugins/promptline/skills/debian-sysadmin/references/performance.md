# Performance

## Measure and classify

Capture workload and time window. Call `toolbox.uptime`, `toolbox.free`, and a bounded `toolbox.ps` first; do not invoke the same-named host tools. Their structured views are baselines, not substitutes for the detailed subsystem-native samples below:

```sh
vmstat 1 10
iostat -xz 1 5
pidstat -dur 1 5
```

Use `top`, `mpstat`, `sar`, `/proc/pressure/*`, `systemd-cgtop`, cgroup properties, `perf`, or application tracing only as the hypothesis narrows.

Interpret load average as runnable plus uninterruptible tasks, not CPU percentage. High load with idle CPU often points to `D`-state tasks, storage/NFS/device waits, locks, or constrained cgroups. Distinguish CPU saturation (`r` relative to CPUs, low idle), memory pressure (available memory, reclaim/PSI, faults, swap-in/out, OOM), I/O latency/queueing, network limits/retransmits, and application lock/contention.

Do not treat cache as wasted memory. Do not tune sysctls, scheduler, THP, swappiness, dirty ratios, congestion control, or limits from a generic guide. For any tuning experiment, record current provenance and value, measured bottleneck, comparable baseline/load, one bounded delta, guardrails, rollback, and before/after metrics. Persist only a reproducible improvement without collateral regression.

Sampling and profiling have overhead; bound duration and output. Stop if host health worsens. Capacity changes and service restarts require their own authorization and verification.
