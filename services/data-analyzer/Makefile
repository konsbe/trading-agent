.PHONY: tidy build docker-build run-technical run-fundamental run-macro

BIN := bin

tidy:
	go mod tidy

build:
	mkdir -p $(BIN)
	go build -o $(BIN)/technical-analysis ./cmd/technical-analysis
	go build -o $(BIN)/fundamental-analysis ./cmd/fundamental-analysis
	go build -o $(BIN)/macro-analysis ./cmd/macro-analysis
	go build -o $(BIN)/market-operations ./cmd/market-operations

docker-build:
	docker build -t trading-agent/data-analyzer:latest .

run-technical: build
	./$(BIN)/technical-analysis

run-fundamental: build
	./$(BIN)/fundamental-analysis

run-macro: build
	./$(BIN)/macro-analysis
