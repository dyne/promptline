# Caddy reverse proxy

## Establish ownership and the active configuration

Treat Caddy as an Internet-facing service and its reload as potentially DISRUPTIVE. Before changing it, identify the installed build, package origin, unit, active command line, configuration adapter, imports, modules, listeners, and upstream application owner.

```sh
dpkg-query -W -f='${Package}\t${Version}\t${Status}\n' caddy
apt-cache policy caddy
caddy version
caddy list-modules --packages
systemctl status caddy --no-pager --full
systemctl cat caddy
systemctl show caddy -p FragmentPath -p DropInPaths -p ExecStart -p ExecReload -p User -p Group -p EnvironmentFiles
ss -lntup
journalctl -u caddy -b --since 'TIME' --no-pager
```

Do not assume `/etc/caddy/Caddyfile`, the `caddy` user, or a particular `ExecReload`; confirm them from the installed unit. Debian stable and oldstable can carry different Caddy versions, and third-party builds can add modules or change service layout. Prefer Debian's package when it meets the requirement. If a newer upstream package or custom module is required, load [APT and dpkg](apt-dpkg.md) and treat the repository, signing key, binary provenance, and upgrade path as a supply-chain change.

Determine whether configuration is file-driven or has diverged through the admin API. Preserve the active configuration and the exact rollback command before editing. Do not directly edit packaged unit files; use a systemd drop-in when the service definition must change.

## Reverse-proxy preflight

Before exposing or moving a site, verify:

- the public name and intended HTTP/HTTPS behavior;
- authoritative A/AAAA records and whether IPv6 is actually reachable;
- inbound reachability for ports 80 and 443 through host, provider, and upstream firewalls;
- that no other process owns the intended listeners;
- the upstream address, protocol, health endpoint, Host expectation, and direct behavior from the Caddy host;
- whether Caddy is the first proxy seen by clients or sits behind a CDN/load balancer;
- filesystem or Unix-socket access for the service user;
- certificate issuance constraints, rate limits, DNS challenge requirements, and writable Caddy state storage.

Test the upstream directly before blaming Caddy. Distinguish connection refusal, timeout, TLS verification/SNI failure, an application error, and a path or Host mismatch. Do not open a backend listener to the public Internet merely to make proxying work; prefer loopback, a private address, or a permission-controlled Unix socket.

## Minimal Caddyfile patterns

A public hostname normally activates automatic HTTPS and HTTP-to-HTTPS redirects when DNS and ports 80/443 are usable:

```caddyfile
example.com {
	log
	reverse_proxy 127.0.0.1:8080
}
```

Caddy sets `X-Forwarded-For`, `X-Forwarded-Proto`, and `X-Forwarded-Host` for proxied requests and rejects untrusted incoming values by default. Do not add customary proxy headers blindly. Configure trusted proxies only when a known proxy/CDN is actually in front, restrict trust to its maintained CIDRs, and use syntax supported by the installed Caddy version. Never trust all client addresses.

For multiple stable upstreams, make health and retry behavior explicit only when the application semantics support it:

```caddyfile
example.com {
	reverse_proxy 10.0.0.11:8080 10.0.0.12:8080 {
		health_uri /healthz
		health_timeout 2s
		fail_duration 30s
		lb_try_duration 5s
	}
}
```

Confirm that the health endpoint does not require interactive authentication and accurately represents readiness. Retries can duplicate non-idempotent requests; do not tune retry matching without understanding request methods and application behavior.

For a Unix socket, grant the `caddy` service user access through deliberate socket ownership/group/mode rather than making it world-writable:

```caddyfile
example.com {
	reverse_proxy unix//run/example/app.sock
}
```

For an HTTPS upstream, preserve certificate verification. Older Caddy releases may require an explicit upstream Host matching TLS SNI:

```caddyfile
example.com {
	reverse_proxy https://app.internal.example:8443 {
		header_up Host {upstream_hostport}
	}
}
```

Use the installed release's supported trust-pool configuration for a private CA. Do not use `tls_insecure_skip_verify` as a repair; it removes upstream identity verification and can conceal a name or trust-chain mistake.

`reverse_proxy` preserves the incoming method and URI by default. Use `handle_path /prefix/*` only when the upstream expects the prefix stripped; otherwise use a matcher that preserves the path. Caddy handles WebSocket upgrades automatically. A config reload can close long-lived streams depending on version and stream settings, so account for reconnect behavior before reloading a busy proxy.

## Safe change and reload

Make the smallest Caddyfile delta and keep imports relative to their actual parent file. Before applying it:

```sh
caddy fmt --diff /etc/caddy/Caddyfile
caddy adapt --config /etc/caddy/Caddyfile --adapter caddyfile --pretty
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
```

`adapt` exposes generated JSON and warnings; `validate` additionally loads and provisions modules. Run validation with the service user's effective file access when permissions or imported files are in doubt. Review the diff and warnings; do not use formatting to hide an unrelated rewrite.

When the installed unit has a correct `ExecReload`, apply with:

```sh
systemctl reload caddy
```

Do not stop/start or restart Caddy for an ordinary configuration change. If reload fails, preserve the running configuration, inspect the journal, correct or restore the file, and validate again. If a changed systemd drop-in or binary truly requires restart, load [systemd](systemd.md), classify the outage risk, and establish an access and service recovery path first.

## Verify the proxy, not just the process

After reload, verify:

```sh
systemctl is-active caddy
systemctl status caddy --no-pager --full
journalctl -u caddy --since 'TIME' --no-pager
curl --fail --show-error --silent --resolve example.com:443:127.0.0.1 https://example.com/healthz
```

Then test through the real public path and inspect certificate name/issuer/expiry, redirect behavior, response headers, application function, access logs, recent runtime errors, upstream health, and the previous site's behavior. Do not use `curl -k` for acceptance because it suppresses the TLS property being verified. Confirm persistence from the file and unit that will be used after reboot; a reboot itself requires separate authority.

## Failure isolation

- **502/503:** correlate the Caddy log timestamp with direct upstream reachability, listener address, socket permissions, protocol, TLS trust/SNI, health status, and upstream logs. A running upstream process is not proof it accepts the proxied request.
- **Certificate issuance:** check authoritative DNS, A and AAAA reachability, port 80/443 ownership, firewall/provider policy, ACME errors, clock, and rate-limit messages. Do not repeatedly retry a known-bad challenge.
- **Redirect loop:** map every TLS terminator and each layer's trusted forwarded-header policy. Prove which hop changes scheme or Host before editing application or Caddy redirects.
- **Wrong client IP:** identify the immediate peer and trust only that proxy's current ranges. Never accept client-supplied forwarding headers directly from the Internet.
- **WebSocket or streaming disconnects:** reproduce across a reload boundary, inspect upgrade/response headers and timeouts, then tune stream behavior only for an understood workload.
- **Path-specific 404:** compare the URI received directly by the upstream with the proxied URI; check `handle_path`, `rewrite`, and application base-path assumptions.

Use the authoritative [Caddy reverse proxy](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy), [command-line](https://caddyserver.com/docs/command-line), and [systemd service](https://caddyserver.com/docs/running) documentation for the installed version's exact syntax and semantics.
