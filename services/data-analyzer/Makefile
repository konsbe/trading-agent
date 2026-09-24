.PHONY: tidy build docker-build run-technical run-fundamental run-macro run-momentum-api run-momentum-daily test test-integration

BIN := bin

tidy:
	go mod tidy

build:
	mkdir -p $(BIN)
	go build -o $(BIN)/technical-analysis ./cmd/technical-analysis
	go build -o $(BIN)/fundamental-analysis ./cmd/fundamental-analysis
	go build -o $(BIN)/macro-analysis ./cmd/macro-analysis
	go build -o $(BIN)/market-operations ./cmd/market-operations
	go build -o $(BIN)/momentum-api ./cmd/momentum-api
	go build -o $(BIN)/momentum-scanner ./cmd/momentum-scanner
	go build -o $(BIN)/momentum-tracker ./cmd/momentum-tracker
	go build -o $(BIN)/momentum-daily ./cmd/momentum-daily

docker-build:
	docker build -t trading-agent/data-analyzer:latest .

run-technical: build
	./$(BIN)/technical-analysis

run-fundamental: build
	./$(BIN)/fundamental-analysis

run-macro: build
	./$(BIN)/macro-analysis

# Read-only momentum scanner API (docs/MOMENTUM_SCANNER_API.md). Binds 127.0.0.1
# by default: it has no auth and must stay on a trusted network.
run-momentum-api: build
	./$(BIN)/momentum-api

# Daily chain: waits for the session's bars, then scanner -> tracker. Add
# ARGS=-once to handle the latest due session and exit.
run-momentum-daily: build
	./$(BIN)/momentum-daily $(ARGS)

test:
	go test ./...

# Real-Postgres tests. Fixtures run inside rolled-back transactions.
test-integration:
	TEST_DATABASE_URL=$${TEST_DATABASE_URL:-$$DATABASE_URL} go test -tags integration -count=1 ./internal/testdb/ ./internal/store/ ./internal/momentumapi/
