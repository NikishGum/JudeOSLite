# syntax=docker/dockerfile:1
FROM node:24.19.0-bookworm-slim AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
# Optional CA secret for managed build environments; ordinary builds use system trust.
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/proxy_ca; fi; \
    npm ci --strict-ssl=true
COPY frontend/ ./
COPY design-system/ /src/design-system/
RUN npm run build

FROM golang:1.27.2-bookworm AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server && \
    CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -trimpath -o /out/create-owner ./cmd/create-owner

FROM debian:bookworm-slim
ARG APP_REVISION=development
LABEL org.opencontainers.image.source="https://github.com/NikishGum/JudeOSLite" \
      org.opencontainers.image.revision="$APP_REVISION"
COPY --from=backend /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=frontend /src/frontend/dist /app/static
COPY --from=backend /out/server /out/migrate /out/create-owner /app/
RUN printf '{"sha":"%s"}\n' "$APP_REVISION" > /app/static/release.json
WORKDIR /app
ENV HTTP_ADDR=:8080 STATIC_DIR=/app/static
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=4s --start-period=10s --retries=3 CMD ["/app/server", "healthcheck"]
CMD ["/app/server"]
