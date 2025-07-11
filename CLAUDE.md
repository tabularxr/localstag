# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based local development pipeline for the Tabular spatial computing platform. It provides a complete local infrastructure for AR/VR development with real-time spatial data streaming from StreamKit devices.

**Architecture**: Two-service system with WebSocket-based data ingestion and HTTP API for data access:
- **Relay Service** (port 8080): WebSocket server that receives spatial data from StreamKit clients
- **Stag Service** (port 9000): Persistent storage backend with REST API for querying spatial graphs
- **Data Flow**: StreamKit Device → WebSocket → Relay → HTTP/JSON → Stag → BoltDB

## Common Commands

### Building and Running
```bash
# Build all services
make build

# Start full pipeline
make run
./start.sh              # Alternative with monitoring
./local-pipeline        # Direct execution

# Start individual services
make run-stag           # Stag service only
make run-relay          # Relay service only
./local-pipeline -stag-only
./local-pipeline -relay-only
```

### Testing
```bash
# Quick validation test
make test
./tests/quick-test.sh

# Comprehensive test
make test-full
./tests/full-pipeline-test.sh

# Run test client simulation
./local-pipeline -test
```

### Database Operations
```bash
# Initialize database
make init
./local-pipeline -init

# List all spatial graphs (stags)
make list
./local-pipeline -list

# Show system statistics
make stats
./local-pipeline -stats

# Clean database (removes all data)
make clean-db
./local-pipeline -clean
```

### Development
```bash
# Clean and rebuild
make clean && make build

# View logs
tail -f *.log

# Get LAN IP for StreamKit connections
./bin/relay -ip
```

## Key Architecture Components

### Service Entry Points
- `cmd/stag/main.go` - Stag service main (HTTP server, BoltDB storage)
- `cmd/relay/main.go` - Relay service main (WebSocket server, HTTP client)
- `local-pipeline.go` - Main controller that orchestrates both services

### Core Implementation (`internal/`)
- `config/config.go` - Configuration management using Viper with environment variable support
- `logging/logging.go` - Structured logging across services
- `stag/service.go` - Stag service implementation (spatial graph storage)
- `relay/service.go` - Relay service implementation (WebSocket handling)
- `storage/` - BoltDB-based storage layer with versioned spatial data

### Configuration
The system uses Viper for configuration with environment variable support:
- `STAG_PORT` - HTTP server port (default: 9000)
- `STAG_DATABASE_PATH` - Database directory (default: ./stag-data)
- `STAG_LOG_LEVEL` - Logging level (debug, info, warn, error)
- `STAG_RELAY_ENDPOINT` - Relay endpoint URL

### Data Model
- **Stags**: Spatial graphs representing AR/VR sessions
- **Anchors**: Individual spatial objects within a stag (meshes, poses, etc.)
- **Versions**: Immutable versions of anchors with timestamps and change tracking
- **Events**: Raw spatial data events from StreamKit devices

### API Endpoints
**Stag Service (port 9000):**
- `GET /health` - Health check
- `POST /api/v1/ingest` - Ingest spatial events (used by Relay)
- `GET /api/v1/stags` - List all stags
- `GET /api/v1/stags/{id}` - Get specific stag
- `GET /api/v1/stags/{id}/anchors` - List anchors in stag
- `GET /api/v1/stags/{id}/anchors/{anchor_id}/history` - Anchor version history

**Relay Service (port 8080):**
- `GET /health` - Health check
- `WS /ws/streamkit` - StreamKit WebSocket connection
- `GET /stats` - Connection statistics

## StreamKit Integration

StreamKit devices connect via WebSocket:
```
ws://[LAN_IP]:8080/ws/streamkit?session_id=your_session&device_id=your_device
```

The system automatically builds and manages LAN connectivity for AR/VR devices on the same network.

## Testing Strategy

The project uses shell script-based testing:
- `tests/quick-test.sh` - Fast validation (health checks, basic data flow)
- `tests/full-pipeline-test.sh` - Comprehensive end-to-end testing
- Built-in test client for WebSocket simulation

## Logging and Monitoring

### Enhanced Logging System
The system now includes comprehensive pipeline-aware logging:
- **Human-readable format**: Emoji icons, color coding, and structured context
- **Pipeline tracing**: End-to-end trace IDs for debugging data flow
- **Performance monitoring**: Built-in metrics tracking with throughput and timing
- **Health monitoring**: Automatic stag health checks and alerts
- **Context-aware**: Logs include stag_id, anchor_id, event_type, client_id, etc.

### Performance Optimizations
- **Batch processing**: Events processed in batches for better throughput
- **Optimized hashing**: Fast content change detection with sampling for large meshes
- **Geometry-aware detection**: Separate geometric signatures for mesh changes
- **Connection pooling**: Reusable hash objects to reduce GC pressure
- **Asynchronous processing**: Non-blocking event ingestion

### Monitoring Commands
```bash
# View real-time logs with enhanced formatting
tail -f *.log

# Check stag health status
./local-pipeline -stats

# Monitor performance metrics
watch -n 5 'curl -s http://localhost:9000/api/v1/stats | jq .'
```

## Development Notes

- Go 1.21+ required
- Uses BoltDB for local storage (no external database needed)
- Gorilla WebSocket for StreamKit connections
- Services are designed to be stateless and containerizable
- All runtime data (logs, database, PIDs) are gitignored
- Makefile provides comprehensive build and test automation
- Optimized for CV segmentation workloads with efficient spatial data handling