# SSH failure

1. Preserve every working session. Determine whether failure is DNS/route/firewall/TCP, host-key, negotiation, authentication, account/PAM, authorization, or post-login/privilege.
2. Use bounded client verbosity and server journal timestamps. Inspect listeners, effective `sshd -T -C`, includes/Match blocks, account state, path ownership/modes, keys and PAM as relevant.
3. Never disable the only working authentication or privilege path. Test replacement keys/accounts first.
4. Validate configuration with `sshd -t`, inspect effective values, arrange recovery, and reload only with authority.
5. Verify a fresh independent connection, intended authentication, command/session behavior, and sudo if required while retaining the original session.
