# DNS failure

1. Prove IP routing and the application endpoint work without DNS.
2. Compare `getent` (NSS/application view), default `dig`, and `dig @configured-server`. Check timeouts versus negative/incorrect answers.
3. Identify ownership of `/etc/resolv.conf`, NSS order, resolver/stub/cache status, DHCP/VPN/split-DNS inputs, search domains and application caching.
4. Fix the owner or authoritative/recursive layer shown faulty; do not overwrite a generated resolv.conf or change public resolvers reflexively.
5. Verify direct DNS, NSS, the application, both address families when relevant, and persistence after the network owner renews/reloads.
