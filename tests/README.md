# Tabular Local Pipeline Tests

This directory contains test scripts for validating the local pipeline functionality.

## Test Scripts

### `quick-test.sh`
Fast validation test that checks:
- Service builds
- Health endpoints
- Basic data ingestion
- Data storage

Usage:
```bash
cd /path/to/localstag
./tests/quick-test.sh
```

### `full-pipeline-test.sh`
Comprehensive end-to-end test that validates:
- Service builds and startup
- Health checks
- REST API endpoints
- Data ingestion and persistence
- CLI operations

Usage:
```bash
cd /path/to/localstag
./tests/full-pipeline-test.sh
```

## Running Tests

Make sure you're in the localstag directory when running tests:

```bash
# Quick test
./tests/quick-test.sh

# Full pipeline test
./tests/full-pipeline-test.sh

# Or using make
make test
```

## Test Requirements

- Go 1.21+
- curl
- Available ports 8080 and 9000
- Write permissions in the project directory

## Test Output

All tests will:
- Create temporary test data in ignored directories
- Generate logs in ignored files
- Clean up after completion
- Report pass/fail status with clear output