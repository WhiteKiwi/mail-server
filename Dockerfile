FROM golang:1.27.3-bookworm@sha256:5c3e9b43cfd3bb95c7937462e29d87ada57b481a96fa23ad0eb61778d7a00110 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG REVISION
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=${REVISION}" -o /out/mail-server ./cmd/mail-server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mail-transfer ./cmd/mail-transfer

FROM node:24.15.0-bookworm-slim@sha256:4e6b70dd6cbfc88c8157ba19aa3d9f9cce6ba4703576d55459e45efcbc9c5f5d
ARG REVISION
LABEL org.opencontainers.image.source="https://github.com/WhiteKiwi/mail-server" \
      org.opencontainers.image.revision=$REVISION
WORKDIR /app
COPY --from=build /out/mail-server /out/mail-transfer ./
COPY deploy/docker/healthcheck.cjs ./
RUN node -e "require('fs').writeFileSync('/app/ca-certificates.pem',require('tls').rootCertificates.join('\n'))" && chmod 0555 /app/mail-server /app/mail-transfer
ENV SSL_CERT_FILE=/app/ca-certificates.pem
USER 10001:10001
HEALTHCHECK --interval=15s --timeout=4s --start-period=30s --retries=3 CMD ["node", "/app/healthcheck.cjs"]
ENTRYPOINT ["/app/mail-server"]
