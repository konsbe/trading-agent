.PHONY: build test run docker clean

SVC_NAME := svc-mcp
IMAGE_NAME := trading-agent-$(SVC_NAME)
BUILD_VERSION ?= latest
PORT ?= 8095

build:
	go build -o bin/$(SVC_NAME) ./cmd/server

test:
	go test ./...

run:
	go run ./cmd/server

docker: build
	docker build --file ./Dockerfile --tag $(IMAGE_NAME):$(BUILD_VERSION) .

clean:
	rm -rf bin
