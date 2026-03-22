.PHONY: all deps frontend backend build dev run test clean

BINARY_NAME=cardlens
GO=go
NPM=npm

all: build

# Install dependencies
deps:
	$(GO) mod tidy
	cd frontend && $(NPM) ci

# Build frontend
frontend:
	cd frontend && $(NPM) run build
	rm -rf internal/assets/dist
	cp -r frontend/dist internal/assets/dist

# Build Go binary (with embedded frontend)
backend:
	$(GO) build -o $(BINARY_NAME) ./cmd/server

# Full production build
build: frontend backend

# Development: run Go server only (use Vite dev server separately)
dev:
	$(GO) run ./cmd/server

# Run production binary
run: build
	./$(BINARY_NAME)

# Run tests
test:
	$(GO) test ./... -v -count=1

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf frontend/dist
	rm -rf internal/assets/dist
