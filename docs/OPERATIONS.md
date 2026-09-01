# Operations

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

Readiness is `GET /readyz`; liveness is `GET /healthz`. Product calls use `POST /v1/deliveries` with `Authorization: Bearer …` and `Idempotency-Key`.

Provider provisioning, DNS records, host bootstrap, database placement, backup,
activation, and rollback are intentionally outside this public repository. Keep those
details in a private infrastructure repository.
