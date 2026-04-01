FROM debian:bookworm-slim AS certs
# Inject corporate proxy CA into the system trust store.
# corp-ca.pem is machine-specific and gitignored. An empty placeholder is committed so this
# COPY always succeeds. The [ -s ] check skips installation when the file is empty (no proxy).
COPY corp-ca.pem /tmp/corp-ca.pem
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && if [ -s /tmp/corp-ca.pem ]; then \
       cp /tmp/corp-ca.pem /usr/local/share/ca-certificates/corp-ca.crt; \
     fi \
  && update-ca-certificates \
  && rm -rf /var/lib/apt/lists/* /tmp/corp-ca.pem

FROM golang:1.24-bookworm AS build
# go.mod may require a newer Go (e.g. 1.25); download matching toolchain during build.
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/data-crypto ./cmd/data-crypto \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-equity ./cmd/data-equity \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-fundamental ./cmd/data-fundamental \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-onchain ./cmd/data-onchain \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-sentiment ./cmd/data-sentiment \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-technical ./cmd/data-technical
# data-technical: fetches daily/weekly OHLCV bars only (no computation).
# Indicator computation lives in services/data-analyzer.

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
# Replace distroless cert tree with Debian's (hashed PEMs + ca-certificates.crt).
# Do not set SSL_CERT_FILE so Go loads the usual Linux paths and cert directory.
COPY --from=certs /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=build /out/data-crypto /app/data-crypto
COPY --from=build /out/data-equity /app/data-equity
COPY --from=build /out/data-fundamental /app/data-fundamental
COPY --from=build /out/data-onchain /app/data-onchain
COPY --from=build /out/data-sentiment /app/data-sentiment
COPY --from=build /out/data-technical /app/data-technical
USER nonroot:nonroot
