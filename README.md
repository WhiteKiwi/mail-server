# WhiteKiwi Mail Server

Reusable transactional email delivery for WhiteKiwi products. Callers select a reviewed template; they cannot supply arbitrary sender, subject, or message bodies.

The reviewed clients are Cerberus (`c6s`) organization, private-beta, and Ops
invitations plus ObsDog organization invitations. Each client has an exact
template allowlist and sender identity; PostgreSQL-backed idempotency prevents a
caller retry from intentionally producing a second delivery.

## Local development

```sh
go test -race ./...
go vet ./...
```

Production loads one protected JSON file through `MAIL_CONFIG_FILE`; local development
may use individual environment variables. See [operations](docs/OPERATIONS.md) and the
[deliverability checklist](docs/DELIVERABILITY.md). Host-specific deployment and
cloud infrastructure stay in a private infrastructure repository. Do not commit
credentials.
