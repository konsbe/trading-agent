FROM golang:1.24-bookworm AS build
# go.mod may require a newer Go (e.g. 1.25); download matching toolchain during build.
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/data-crypto ./cmd/data-crypto \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-equity ./cmd/data-equity \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-onchain ./cmd/data-onchain \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-sentiment ./cmd/data-sentiment

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
# Explicitly copy CA certs from the build stage so HTTPS works regardless of cache state.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/data-crypto /app/data-crypto
COPY --from=build /out/data-equity /app/data-equity
COPY --from=build /out/data-onchain /app/data-onchain
COPY --from=build /out/data-sentiment /app/data-sentiment
USER nonroot:nonroot
