# Tabular Local Pipeline - Directory Structure

## 🎯 Clean Organization

The repository has been cleaned and organized with the following structure:

```
localstag/
├── .gitignore                # Git ignore rules for build artifacts, logs, data
├── README.md                 # Main documentation
├── STRUCTURE.md             # This file
├── Makefile                 # Build automation and commands
├── go.mod                   # Go module definition
├── go.sum                   # Go module checksums
├── local-pipeline.go        # Main executable source
├── start.sh                 # Startup script with monitoring
│
├── cmd/                     # Service entry points
│   ├── relay/main.go        # Relay service main
│   ├── stag/main.go         # Stag service main
│   └── test-client/main.go  # Test client main
│
├── internal/                # Core implementation (private)
│   ├── config/config.go     # Configuration management
│   ├── logging/logging.go   # Structured logging
│   ├── relay/service.go     # Relay service implementation
│   ├── stag/service.go      # Stag service implementation
│   └── storage/             # Storage layer
│       ├── storage.go       # BoltDB implementation
│       └── types.go         # Data type definitions
│
└── tests/                   # Test scripts and utilities
    ├── README.md            # Test documentation
    ├── quick-test.sh        # Fast validation test
    └── full-pipeline-test.sh # Comprehensive test
```

## 🧹 What Was Cleaned Up

### Removed Files/Directories:
- `bin/` - Build artifacts (ignored by .gitignore)
- `*-data/` - Test data directories (ignored)
- `*.log` - Log files (ignored)
- `*.pid` - Process ID files (ignored)
- Old test scripts from root directory
- Temporary files and outputs

### Organized Files:
- Moved test scripts to `tests/` directory
- Added comprehensive .gitignore
- Updated Makefile with proper test commands
- Created clean documentation structure

## 🚀 Usage

### Quick Start:
```bash
cd localstag
./start.sh              # Start the pipeline
```

### Development:
```bash
make build              # Build all services
make test               # Run quick test
make test-full          # Run comprehensive test
make clean              # Clean build artifacts
```

### Testing:
```bash
./tests/quick-test.sh           # Fast validation
./tests/full-pipeline-test.sh   # Full end-to-end test
```

## 📁 Git Ignore Rules

The `.gitignore` file excludes:
- Build artifacts (`bin/`, `*.exe`)
- Runtime data (`*-data/`, `*.db`, `*.log`, `*.pid`)
- IDE files (`.vscode/`, `.idea/`)
- OS files (`.DS_Store`, `Thumbs.db`)
- Test outputs and temporary files

## 🎯 Ready for Development

The repository is now clean and ready for:
- Version control with Git
- Development and testing
- Production deployment
- Collaboration and sharing

All test outputs, logs, and build artifacts are properly ignored, keeping the repository clean while preserving all functionality.