.PHONY: tidy build docker-build run-crypto run-equity run-onchain run-sentiment run-fundamental run-technical

BIN := bin

tidy:
	go mod tidy

build:
	mkdir -p $(BIN)
	go build -o $(BIN)/data-crypto ./cmd/data-crypto
	go build -o $(BIN)/data-equity ./cmd/data-equity
	go build -o $(BIN)/data-fundamental ./cmd/data-fundamental
	go build -o $(BIN)/data-onchain ./cmd/data-onchain
	go build -o $(BIN)/data-sentiment ./cmd/data-sentiment
	go build -o $(BIN)/data-technical ./cmd/data-technical
	go build -o $(BIN)/data-macro-intel ./cmd/data-macro-intel

docker-build:
	docker build -t trading-agent/data-ingestion:latest .

run-crypto: build
	./$(BIN)/data-crypto

run-equity: build
	./$(BIN)/data-equity

run-fundamental: build
	./$(BIN)/data-fundamental

run-onchain: build
	./$(BIN)/data-onchain

run-sentiment: build
	./$(BIN)/data-sentiment

run-technical: build
	./$(BIN)/data-technical

run-macro-intel: build
	./$(BIN)/data-macro-intel
