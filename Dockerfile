FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/data-crypto ./cmd/data-crypto \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-equity ./cmd/data-equity \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-onchain ./cmd/data-onchain \
  && CGO_ENABLED=0 GOOS=linux go build -o /out/data-sentiment ./cmd/data-sentiment

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/data-crypto /app/data-crypto
COPY --from=build /out/data-equity /app/data-equity
COPY --from=build /out/data-onchain /app/data-onchain
COPY --from=build /out/data-sentiment /app/data-sentiment
USER nonroot:nonroot
