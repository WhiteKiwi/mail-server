# CI runner routing

The owner requested a repository-scoped Ubuntu runner for this public repository on
2026-10-04. The current worker is physical Ubuntu after the Windows host retirement. Owner-triggered main pushes and same-repository pull requests authored
by the owner use `[self-hosted, Linux, X64, whitekiwi-linux-mail-server]`. Other
pull requests, including forks and bots, use standard GitHub-hosted Ubuntu.

All external contributors require maintainer approval for fork workflows in the
repository's Actions settings. Before approving a fork workflow, inspect its
workflow changes and retain hosted routing: a pull request can modify its own
workflow, so the checked-in selector alone is not an access-control boundary.

Ordinary source CI has sequential formatting, full normal unit/hermetic core,
vet and executable build stages. Go package and test schedulers use one. Fake
mail transport tests protect authentication, scope/template boundaries, headers
and bounded redaction without a real SMTP/provider call. They are cheap core
components because static checks alone cannot establish those route contracts.
No PostgreSQL server is provisioned by ordinary CI. The two explicit real SQL /
provisioned-runtime tests skip when their dedicated fixture inputs are absent;
a source-CI pass does not qualify SQL, ACLs, runtime roles or actual delivery.

For related SQL/migration/locking/runtime changes, run the original real checks
locally before submitting the PR, using a new disposable loopback PostgreSQL18
instance and dedicated test database. Do not point fixture inputs at an installed
service or reuse production data. Apply `internal/migrations/001_initial.sql`
twice with `psql -v ON_ERROR_STOP=1`, then run:

```sh
# Set this only to that disposable fixture, never an installed runtime database.
: "${MAIL_TEST_DATABASE_URL:?Set the disposable test DSN}"
go test -race -p=1 -parallel=1 -count=1 ./...
# Qualify provisioned runtime roles only with its separately prepared fixture:
: "${MAIL_TEST_RUNTIME_DATABASE_URL:?Set the dedicated runtime fixture DSN}"
go test -race -p=1 -parallel=1 -count=1 ./internal/delivery
```

Record exact source SHA, PostgreSQL/OS/Go versions, commands, migration/runtime
scope, actual case results, retries and skips/untested boundaries in the PR body
or comment. Missing fixture inputs or a skipped case are not real SQL acceptance.
Related memory-sharing changes need local race evidence; there is no separately
justified per-PR instrumentation profile for this small ordinary suite. An extra
component/race/E2E profile needs a critical failure, insufficient smaller checks,
per-PR automation necessity and a stable isolated budget. Do not add manual
Actions as the default substitute. Diagnose setup/build/cases and CPU/memory/I/O
before adding workers; product concurrency and all existing assertions remain.

Owner-triggered release jobs use the same Ubuntu label. The macOS arm64 binary is
cross-compiled with CGO disabled. The existing release publishing contract stays
in place. A manual `verify-only: true` run checks an existing tag and packages the
artifact without creating or changing a release. For example:

```sh
gh workflow run release.yml --ref main -f tag=v0.3.2 -f verify-only=true
```

Host installation, capacity, service identities and recovery are maintained in
the private infrastructure repository.
