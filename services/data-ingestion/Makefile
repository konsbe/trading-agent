.PHONY: tidy build docker-build run-crypto run-equity run-onchain run-sentiment

BIN := bin

tidy:
	go mod tidy

build:
	mkdir -p $(BIN)
	go build -o $(BIN)/data-crypto ./cmd/data-crypto
	go build -o $(BIN)/data-equity ./cmd/data-equity
	go build -o $(BIN)/data-onchain ./cmd/data-onchain
	go build -o $(BIN)/data-sentiment ./cmd/data-sentiment

docker-build:
	docker build -t trading-agent/data-ingestion:latest .

run-crypto: build
	./$(BIN)/data-crypto

run-equity: build
	./$(BIN)/data-equity

run-onchain: build
	./$(BIN)/data-onchain

run-sentiment: build
	./$(BIN)/data-sentiment
