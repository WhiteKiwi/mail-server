# Operations

Run `mail-server migrate` in a separate operator process with
`MAIL_MIGRATION_DATABASE_URL` supplied through protected process configuration.
There is no fallback to the runtime URL, no SMTP/client credential requirement and
no listener or SMTP delivery in this command. Run it before activating a compatible
server release. The server constructs a lazy pool and starts HTTP even while PostgreSQL is offline.
Each authenticated delivery verifies the selected schema in a bounded read-only
transaction before reserving or sending; unprepared/offline databases return503.
The pool reconnects on subsequent requests. Health never depends on PostgreSQL.

Runtime `database_url` needs only CONNECT, schema USAGE and SELECT/INSERT/UPDATE on
`public.mail_deliveries`. Keep bootstrap/migration credentials outside the runtime
account's readable files and environment. Existing ownership/grants, default
privileges, backup/restore and activation must be qualified together by the host.
This source change does not rotate a credential or migrate an installed service.

Production should set `MAIL_CONFIG_FILE` to a protected JSON file. The file must be
regular and no more permissive than `0640`; unknown fields and trailing documents
are rejected. It contains the equivalent of:

```json
{
  "listen_address": "127.0.0.1:8092",
  "database_url": "postgres://SERVICE_USER:PASSWORD@DATABASE_HOST:5432/mail?sslmode=require",
  "clients": [{
    "id": "c6s",
    "token": "RANDOM_32_BYTE_OR_LONGER_TOKEN",
    "templates": [
      "cerberus.organization-invitation",
      "cerberus.beta-invitation",
      "cerberus.ops-invitation"
    ],
    "from_address": "no-reply@whitekiwi.link"
  }, {
    "id": "obsdog",
    "token": "ANOTHER_RANDOM_32_BYTE_OR_LONGER_TOKEN",
    "templates": ["obsdog.organization-invitation"],
    "from_address": "notifications@obsdog.ai",
    "ses_configuration_set": "obsdog-transactional"
  }],
  "smtp_host": "SMTP_HOST",
  "smtp_port": 587,
  "smtp_username": "SES_SMTP_USERNAME",
  "smtp_password": "SES_SMTP_PASSWORD",
  "from_address": "no-reply@whitekiwi.link",
  "ses_configuration_set": "OPTIONAL_CONFIGURATION_SET"
}
```

Environment variables with the `MAIL_` equivalents remain available for local
development only. Provider and client credentials must never enter a plist, argv,
Git, logs, or CI artifacts.

Provider credentials belong only in the deployer's runtime secret store. The service
never prints them. Restrict the provider identity to the intended verified sender
domains. `from_address` on a client is optional for backwards compatibility and
otherwise falls back to the top-level sender; only the reviewed `whitekiwi.link`
and `obsdog.ai` domains are accepted. `ses_configuration_set` is likewise an
optional per-client override so product reputation streams remain distinct.

Provider failures are logged with an opaque delivery ID and one fixed stage code:
`connect`, `start`, `starttls_required`, `starttls`, `authenticate`, `sender`,
`recipient`, `data`, `write`, `commit`, `quit`, or `unknown`. Stage codes are safe
operational metadata; SMTP responses, addresses, message content, configuration,
and credentials must never be logged or returned to product clients.

Startup, liveness and readiness all use `GET /healthz` (204, application only).
There is no separate `/readyz`. Product calls use `POST /v1/deliveries` with `Authorization: Bearer …` and `Idempotency-Key`.

Provider provisioning, DNS records, host bootstrap, database placement, backup,
activation, and rollback are intentionally outside this public repository. Keep those
details in a private infrastructure repository.

## Kubernetes runtime

Use `MAIL_DATABASE_SCHEMA=app` for the registered target schema; the default
`public` remains available for explicit native recovery. Runtime SQL qualifies
the selected schema. The application pool is capped at five connections.
`mail-server migrate` uses the selected schema and never runs during HTTP startup.
A protected `MAIL_MIGRATION_DATABASE_URL_FILE` may replace the migration environment
URL; it must be a regular owner-only file. Runtime pods never mount this input.

An init container runs `mail-server prepare-runtime SOURCE_JSON POSTGRES_DIR DESTINATION`.
It preserves provider/client values and replaces only the listener/database transport.
The PostgreSQL input directory contains host, port, database, username, password and
sslmode files. It emits a new regular0400 file in a private0700 directory on a memory
volume. The runtime mounts only that directory read-only. Existing outputs are refused.
Migration init uses `mail-server prepare-migration POSTGRES_DIR DESTINATION`; it reads
no SMTP/client input and chooses only the migration principal/owner with pool1.

An explicitly selected private HTTP deployment can use a non-loopback listener behind
exact caller NetworkPolicies and the existing Bearer/template policy. The host owns
that accepted transport boundary; it is not an arbitrary remote HTTP client exception.
Requests have a20s work budget and bounded5s state finalization after known provider
acceptance, even if the client disconnects. SIGTERM stops acceptance and waits up to30s
for active handlers before process exit; the Kubernetes host allows45s.

The root Dockerfile builds an AMD64 image. Manual Publish image follows the common
private GHCR route and does not deploy. The pinned image/source, Secrets, Argo manual
Sync, data transfer and operating evidence belong to the private infra repository.

## One-time data transfer

`mail-transfer` accepts only a protected CSV and a protected target URL file. The source
is PostgreSQL18 COPY CSV HEADER, all11 columns in schema order, ordered by id, with
TimeZone UTC, DateStyle ISO/YMD and bytea_output hex. The old writer must be stopped.
Use `--input PATH --sha256 SOURCE_SHA256 --database-url-file PATH`. The tool validates
header/digests/order, requires an empty app target, imports under an exclusive table
lock, and compares the full target COPY bytes before commit. It never contacts SMTP.
Only rows/SHA256 are printed. After an uncertain response use `--verify-only`; do not
retry an import into a nonempty target or clear data. No periodic backup is introduced.

Use `mail-transfer --dry-run` against an empty prepared target to qualify COPY and
full comparison with synthetic data; it always rolls back. Do not combine it with
`--verify-only`. Neither mode performs DDL or contacts SMTP. An imported target
is never overwritten or truncated on retry.
