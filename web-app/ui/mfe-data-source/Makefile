.PHONY: install build test coverage typecheck run run-prod docker clean

MFE_NAME := mfe-data-source
IMAGE_NAME ?= trading-agent-$(MFE_NAME)
BUILD_VERSION ?= latest
MOMENTUM_API_URL ?= http://localhost:8090

# Use Node 22 via nvm when available (nvm is a shell function, so it must be
# sourced in the same shell as the npm command); otherwise use the node on PATH.
NODE22 := if [ -s "$$HOME/.nvm/nvm.sh" ]; then . "$$HOME/.nvm/nvm.sh" && nvm use 22 >/dev/null; fi;
NPM_REGISTRY := --registry=https://registry.npmjs.org/

install:
	@$(NODE22) npm install --legacy-peer-deps $(NPM_REGISTRY)

build: install
	@$(NODE22) MOMENTUM_API_URL=$(MOMENTUM_API_URL) npm run build

test: install
	@$(NODE22) npm test

coverage: install
	@$(NODE22) npm run test:coverage

typecheck: install
	@$(NODE22) npm run typecheck

run: install
	@$(NODE22) MOMENTUM_API_URL=$(MOMENTUM_API_URL) npm run start-dev

run-prod: install
	@$(NODE22) MOMENTUM_API_URL=$(MOMENTUM_API_URL) npm run start-app

# Build context is web-app/ because the MFE compiles ../../shared-components from source.
docker:
	docker build --file ./Dockerfile --build-arg MOMENTUM_API_URL=$(MOMENTUM_API_URL) \
		--tag $(IMAGE_NAME):$(BUILD_VERSION) ../..

clean:
	rm -rf node_modules dist coverage .jest-cache
