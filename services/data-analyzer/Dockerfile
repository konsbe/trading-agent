FROM debian:bookworm-slim AS certs
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && update-ca-certificates \
  && rm -rf /var/lib/apt/lists/*

FROM golang:1.24-bookworm AS build
# go.mod may require a newer Go (e.g. 1.25); download matching toolchain during build.
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/technical-analysis ./cmd/technical-analysis \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/fundamental-analysis ./cmd/fundamental-analysis \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/macro-analysis ./cmd/macro-analysis

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=certs /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=build /out/technical-analysis /app/technical-analysis
COPY --from=build /out/fundamental-analysis /app/fundamental-analysis
COPY --from=build /out/macro-analysis /app/macro-analysis
USER nonroot:nonroot
