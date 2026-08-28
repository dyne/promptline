# Toolchain and release integrity policy

Promptline preserves Go 1.24 language semantics through `go 1.24.0` in
`go.mod`, and compiles with the supported patch compiler named by both the
`toolchain` directive and `.go-version`. This separation lets security fixes
in newer compilers land without silently enabling newer language features.

To update the compiler, choose a currently supported Go patch release, update
both declarations together, run `make check-go-policy vulncheck test-all`, and
record the reviewed release notes in the change review. Never replace a
workflow action digest with a tag: update the digest and its version comment
together after reviewing the action's signed upstream release. Semantic-release
plugins must likewise carry an explicit reviewed version.

CI defaults to read-only `contents` access. Only main-branch push jobs that
create, attest, remove an incomplete release, or upload verified artifacts
receive write scopes. Pull requests never reach those jobs. The weekly
scheduled `security-policy` job reruns the reachable-vulnerability scan.

Release builds use `-trimpath`, emit SHA-256 manifests and SPDX metadata, and
are attested by GitHub before upload. The upload job verifies checksums before
publishing; an artifact-build failure invokes the narrowly scoped cleanup job
instead of leaving a completed release visible.
