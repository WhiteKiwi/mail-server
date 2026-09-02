# Architecture

`c6s-server` keeps its domain outbox and calls this service only when an invitation is ready. The mail server authenticates the product client, validates an idempotency key, renders a server-owned template, reserves the request in PostgreSQL, and submits it through the configured SMTP provider.

Security boundaries:

- `c6s-server` owns invitation lifecycle and the one-time acceptance token.
- `mail-server` owns provider credentials, sender identity, rendering, and cross-retry deduplication.
- A client token is scoped to an explicit template allowlist.
- Recipient plaintext and rendered bodies are never persisted or logged.
- Delivery records contain SHA-256 digests, template identity, state, and timestamps only.
- Each provider submission carries the opaque local delivery ID as the bounded
  SES message tag `whitekiwi_delivery_id`. SES event publishing can therefore
  correlate delivery, bounce, and complaint outcomes without recipient
  plaintext, rendered content, or invitation tokens.
- The default listener is loopback. Public ingress requires TLS and an additional network allowlist.

The API is deliberately not a general email relay. Adding a template requires source review, validation rules, tests, and an explicit client allowlist entry. Host identity, database placement, backup policy, and release activation belong to the deployer's private infrastructure configuration.
