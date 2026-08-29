# AGENTS.md

## Security

- This service sends only server-owned transactional templates. Never add an arbitrary subject, body, or From-address endpoint.
- Never log recipient addresses, authorization tokens, template variables, SMTP credentials, or rendered messages.
- Persist recipient and request digests only; plaintext may exist only for the duration of an authenticated delivery request.
- Bind to loopback by default. A non-loopback deployment must sit behind authenticated TLS ingress.

## Git

- Commit messages use `{type}: {imperative message}`.
