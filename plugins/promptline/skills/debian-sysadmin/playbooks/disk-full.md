# Root filesystem full

1. Confirm the affected mount with `findmnt`, call `toolbox.df` for its block usage, use `stat -f` for inode counters, and distinguish quota, reserved-space, read-only, and thin-pool exhaustion with stack-specific tools. Do not invoke host `df`.
2. Check deleted-open files (`lsof +L1`) and use bounded `toolbox.du` calls only after mapping nested mounts. Correlate growth with writers, logs, package cache, containers, databases, cores, temp files, or runaway jobs.
3. If service survival requires emergency space, choose the least destructive documented cleanup or stop the writer; preserve incident evidence. Do not delete unknown files or logs blindly.
4. Correct retention/rotation/writer behavior or expand capacity through a separately authorized storage plan.
5. Verify free blocks/inodes, writer/service function, journal/kernel errors, and that growth has stabilized.
