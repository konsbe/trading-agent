.PHONY: all install build test coverage run dev clean docker

IMAGE_NAME ?= trading-agent-spog
BUILD_VERSION ?= latest
DIST_DIR = ./dist

all: build test

install:
	npm install --legacy-peer-deps

build: install
	npm run build

test: install
	npm test

coverage: install
	npm run test:coverage

run: install
	npm run start-app

dev: install
	npm run start-dev:all

clean:
	rm -rf ./node_modules $(DIST_DIR) ./coverage ./.jest-cache

docker:
	docker build -f Dockerfile -t $(IMAGE_NAME):$(BUILD_VERSION) ..
