# CI runner routing

The owner requested a Windows-hosted Ubuntu runner for this public repository on
2026-10-04. Owner-triggered main pushes and same-repository pull requests authored
by the owner use `[self-hosted, Linux, X64, whitekiwi-linux-mail-server]`. Other
pull requests, including forks and bots, use standard GitHub-hosted Ubuntu.

All external contributors require maintainer approval for fork workflows in the
repository's Actions settings. Before approving a fork workflow, inspect its
workflow changes and retain hosted routing: a pull request can modify its own
workflow, so the checked-in selector alone is not an access-control boundary.

CI keeps formatting, race tests, vet and two migration applications. PostgreSQL
uses the fully qualified `docker.io/library/postgres:18-alpine` image, and SQL is
sent to `psql` inside the service container. There is no fixed host port or host
PostgreSQL client dependency. This works with Docker on hosted runners and the
worker's rootless Podman Docker CLI.

Owner-triggered release jobs use the same Ubuntu label. The macOS arm64 binary is
cross-compiled with CGO disabled. The existing release publishing contract stays
in place. A manual `verify-only: true` run checks an existing tag and packages the
artifact without creating or changing a release. For example:

```sh
gh workflow run release.yml --ref main -f tag=v0.3.2 -f verify-only=true
```

Host installation, capacity, service identities and recovery are maintained in
the private infrastructure repository.
