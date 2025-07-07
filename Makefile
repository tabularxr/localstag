# Tabular Local Pipeline Makefile

.PHONY: build clean run test stop deps help

# Default target
all: build

# Build all services
build:
	@echo "🔨 Building Tabular Local Pipeline..."
	@mkdir -p bin
	@go mod tidy
	@go build -o local-pipeline .
	@go build -o bin/stag ./cmd/stag
	@go build -o bin/relay ./cmd/relay
	@echo "✅ Build complete"

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies installed"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f local-pipeline
	@rm -f *.log
	@rm -f *.pid
	@rm -rf stag-data/
	@rm -rf test-data/
	@rm -rf *-data/
	@echo "✅ Clean complete"

# Run the full pipeline
run: build
	@echo "🚀 Starting Tabular Local Pipeline..."
	@./local-pipeline

# Run only the Stag service
run-stag: build
	@echo "🌍 Starting Stag service only..."
	@./local-pipeline -stag-only

# Run only the Relay service
run-relay: build
	@echo "🌐 Starting Relay service only..."
	@./local-pipeline -relay-only

# Initialize database
init: build
	@echo "🔧 Initializing database..."
	@./local-pipeline -init

# List all stags
list: build
	@echo "📊 Listing stags..."
	@./local-pipeline -list

# Show statistics
stats: build
	@echo "📈 Showing statistics..."
	@./local-pipeline -stats

# Clean database
clean-db: build
	@echo "🧹 Cleaning database..."
	@./local-pipeline -clean

# Run quick test
test: build
	@echo "🧪 Running quick test..."
	@./tests/quick-test.sh

# Run full pipeline test
test-full: build
	@echo "🧪 Running full pipeline test..."
	@./tests/full-pipeline-test.sh

# Show help
help:
	@echo "🦾 Tabular Local Pipeline Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  make build      - Build all services"
	@echo "  make deps       - Install dependencies"
	@echo "  make clean      - Clean build artifacts"
	@echo ""
	@echo "Run Commands:"
	@echo "  make run        - Start full pipeline (Relay + Stag)"
	@echo "  make run-stag   - Start Stag service only"
	@echo "  make run-relay  - Start Relay service only"
	@echo ""
	@echo "Database Commands:"
	@echo "  make init       - Initialize database"
	@echo "  make list       - List all stags"
	@echo "  make stats      - Show system statistics"
	@echo "  make clean-db   - Clean database"
	@echo ""
	@echo "Test Commands:"
	@echo "  make test       - Run quick validation test"
	@echo "  make test-full  - Run comprehensive pipeline test"
	@echo ""
	@echo "Utility Commands:"
	@echo "  make help       - Show this help message"
	@echo ""
	@echo "Example Usage:"
	@echo "  1. Initialize: make init"
	@echo "  2. Start pipeline: make run"
	@echo "  3. Test: make test"
	@echo "  4. View data: make list"
	@echo "  5. Clean up: make clean"

# Development shortcuts
dev: clean build run

# Quick test
quick-test: build
	@./local-pipeline -test