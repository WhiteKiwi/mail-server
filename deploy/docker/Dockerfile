FROM node:24.15.0-bookworm-slim@sha256:4e6b70dd6cbfc88c8157ba19aa3d9f9cce6ba4703576d55459e45efcbc9c5f5d
ARG REVISION
LABEL org.opencontainers.image.source="https://github.com/WhiteKiwi/mail-server" \
      org.opencontainers.image.revision=$REVISION
WORKDIR /app
COPY mail-server healthcheck.cjs ./
RUN chmod 0555 /app/mail-server && \
    node -e "require('fs').writeFileSync('/app/ca-certificates.pem',require('tls').rootCertificates.join('\n'))"
ENV SSL_CERT_FILE=/app/ca-certificates.pem
USER 501:20
HEALTHCHECK --interval=15s --timeout=4s --start-period=30s --retries=3 CMD ["node", "/app/healthcheck.cjs"]
ENTRYPOINT ["/app/mail-server"]
