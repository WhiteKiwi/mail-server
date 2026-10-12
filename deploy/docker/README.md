# Docker Desktop runtime

Build a Linux ARM64 `CGO_ENABLED=0` binary on the operator host with the committed
Go/module pins and `-trimpath`. Copy only that binary, Dockerfile and healthcheck
into a fresh credential-free runtime context. The pinned Node base is shared
with the other apps; its public CA bundle supplies Go's SMTP TLS trust and its
short-lived healthcheck reads `/readyz`. The Go server is the long-lived process.

Infra publishes only host loopback8092 and mounts one private configuration
directory read-only. `MAIL_CONFIG_FILE=/run/mail/production.json` retains the
native JSON byte-for-byte, including existing SMTP credentials and client policy.
`MAIL_DATABASE_TRANSPORT=docker-desktop` changes the in-memory listener from
`127.0.0.1:8092` to `0.0.0.0:8092` and maps only the exact
`mail_runtime@127.0.0.1:25432/mail` non-TLS connection to `host.docker.internal`.
Owner/migration users, another target/port, a role override and unknown transport
fail. Native startup and explicit migration defaults remain unchanged.

`mail-server check-runtime` loads the protected config, opens the runtime pool,
verifies the existing schema/role contract and exits without starting HTTP or
SMTP. Use this for pre-cutover admission. Docker transport rejects `migrate`;
do not mount migration/operator inputs or restore the retained stale Docker DB.
Infra owns writer drain, local readiness/authentication/client policy checks,
container recreation and native recovery against the same latest database.
No delivery probe or periodic backup is required for app-only cutover.
