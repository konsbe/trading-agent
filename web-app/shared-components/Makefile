.PHONY: all build test clean tidy fmt vet coverage docker helm

DIST_DIR=./dist

all: clean fmt vet tidy build test

build:
	@echo "Building shared-components..."
	@echo "Installing only production dependencies (no devDependencies)..."
	npm install --legacy-peer-deps --ignore-scripts --omit=dev || true
	@echo "Installing TypeScript compiler (needed for build)..."
	npm install --no-save --legacy-peer-deps typescript @types/node
	npm run build
	@echo "Shared components ready"

test:
	@echo "Running tests with coverage for shared-components..."
	npm install --legacy-peer-deps
	npm run test:coverage

coverage:
	@echo "Running coverage for shared-components..."
	npm install --legacy-peer-deps
	npm run test:coverage

clean:
	@echo "Cleaning shared-components..."
	rm -rf node_modules $(DIST_DIR) coverage
	rm -rf package-lock.json

tidy:
	@echo "Tidy shared-components..."

fmt:
	@echo "Formatting shared-components..."

vet:
	@echo "Vetting shared-components..."

print:
	@echo "shared-components"

docker:
	@echo "Skipping docker build for shared-components (library only)"

helm:
	@echo "Skipping helm packaging for shared-components (library only)"
