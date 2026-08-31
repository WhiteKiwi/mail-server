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
    ]
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
domain.

Readiness is `GET /readyz`; liveness is `GET /healthz`. Product calls use `POST /v1/deliveries` with `Authorization: Bearer …` and `Idempotency-Key`.

Provider provisioning, DNS records, host bootstrap, database placement, backup,
activation, and rollback are intentionally outside this public repository. Keep those
details in a private infrastructure repository.
