.PHONY: tidy build docker-up docker-down run-crypto

tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o bin/data-crypto ./cmd/data-crypto
	go build -o bin/data-equity ./cmd/data-equity
	go build -o bin/data-onchain ./cmd/data-onchain
	go build -o bin/data-sentiment ./cmd/data-sentiment

docker-up:
	docker compose up -d timescaledb redis

docker-down:
	docker compose down

run-crypto: build
	./bin/data-crypto
