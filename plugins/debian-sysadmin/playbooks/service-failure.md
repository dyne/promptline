# Service failure

1. Confirm exact unit, expected behavior, failure time, impact, and recent package/config changes.
2. Inspect `systemctl status`, `systemctl cat`, effective properties, dependencies, failed units, and the unit journal for the relevant boot/time. Do not restart first.
3. Run the application's configuration/check mode and inspect missing files, permissions, ports, credentials, dependencies, resource limits, and package conffile transitions.
4. Form one hypothesis and test it read-only. For an upgrade regression, compare installed version, NEWS/changelog, conffile changes and dependency state.
5. Plan minimal edit/rollback. Validate and diff before authorized reload/restart.
6. Verify active state, process/listener, actual request or health check, recent journal, dependencies, and persistence. Record the cause rather than reporting only that restart worked.
