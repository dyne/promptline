# APT and dpkg

## Inspect before acting

```sh
dpkg-query -W -f='${binary:Package}\t${db:Status-Abbrev}\t${Version}\n' 'PACKAGE*'
apt-cache policy PACKAGE
apt-mark showhold
dpkg --audit
dpkg-query -S -- /path/to/file
dpkg-query -L PACKAGE
```

Inspect `/etc/apt/sources.list`, `.list` and deb822 `.sources` files, preferences, holds, architecture, and `/var/log/apt/` plus `/var/log/dpkg.log`. Check for an active apt/dpkg process and locks; never delete lock files merely to bypass a live transaction.

## Doctrine

- Use `apt` interactively when its user interface is useful; use stable `apt-get`/`apt-cache` interfaces in scripts.
- Prefer Debian repositories and packages. Confirm origin, suite, components, architecture, candidate version, and pin priority before install or upgrade.
- Prefer deb822 `.sources` for new source definitions when supported by the target release.
- For third-party sources, verify the vendor, transport, fingerprint through an independent channel, Debian suite support, and expected packages. Put operator-managed keyrings in `/etc/apt/keyrings`; use `Signed-By` scoped to that source. Package-managed keyrings belong in `/usr/share/keyrings`. Ensure `_apt` can read them. Never use `apt-key` or globally trust an unrelated repository key.
- Do not accept `trusted=yes`, insecure repositories, signature bypasses, or `curl | sh` as convenience.
- Simulate risky resolution with `apt-get -s ...`; inspect removals, newly installed packages, held packages, and essential/protected packages.
- `remove` retains conffiles; `purge` removes package conffiles but not arbitrary application data. Inspect conffile prompts/transitions during upgrades.
- Review `apt-get autoremove --dry-run`; do not run autoremove casually on production. Investigate meta-package removal and manually/automatically installed state first.

## Ownership and local variation

Do not overwrite package-owned files without understanding conffile behavior. Prefer documented `/etc` drop-ins. Use `dpkg-divert` only for a deliberate local policy with recorded ownership and reversal; use `update-alternatives` only for files managed by its link group. Inspect triggers and maintainer-script failures rather than deleting package database content.

## Recovery sequence

For an interrupted transaction, preserve exact errors, confirm no package manager is running, check space/inodes/read-only filesystems and repository health, then use the narrowest supported sequence:

```sh
sudo dpkg --configure -a
sudo apt-get -f install
sudo dpkg --audit
```

Do not run all three mechanically: address the first concrete maintainer-script, conffile, dependency, disk, or repository error before continuing. Re-run the originally intended operation only after package state is consistent. Never delete `/var/lib/dpkg`, use `dpkg --force-*`, or replace status files as routine repair.

For failed upgrades, identify old/new package versions and configured sources, read `/usr/share/doc/PACKAGE/NEWS.Debian*` and package changelogs where relevant, inspect `.dpkg-dist`, `.dpkg-old`, or `.ucf-*` transitions, and verify affected services after recovery.

## Verification

Verify dpkg status, installed and candidate versions, holds/pins, repository origin, failed units, affected service function, and any pending reboot reason. Package success alone does not prove application success.
