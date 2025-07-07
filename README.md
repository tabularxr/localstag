# 🚀 Tabular Local Pipeline

A complete local development environment for the Tabular spatial computing platform. This pipeline enables you to run a full end-to-end system on your local machine, allowing StreamKit devices to connect via LAN and stream spatial data through a Relay service into a persistent Stag backend.

## 🎯 What This Provides

- **🌐 Relay Service** - WebSocket server that receives spatial data from StreamKit clients
- **🌍 Stag Service** - Persistent storage backend that maintains versioned spatial graphs  
- **🧪 Test Infrastructure** - Built-in tools for testing and development
- **📱 LAN Connectivity** - StreamKit devices can connect over WiFi
- **💾 Local Storage** - All data persisted locally with BoltDB

## 🛠️ Prerequisites

Before you begin, ensure you have the following installed:

### Required:
- **Go 1.21+** - [Download and install Go](https://golang.org/dl/)
- **Git** - For version control (usually pre-installed on macOS/Linux)
- **curl** - For testing API endpoints (usually pre-installed)
- **make** - For build automation (usually pre-installed)

### Verify Prerequisites:
```bash
# Check Go version (should be 1.21+)
go version

# Check other tools
git --version
curl --version
make --version
```

## 🚀 Step-by-Step Setup Guide

### Step 1: Get the Code

```bash
# Navigate to your projects directory
cd ~/Projects  # or wherever you keep your code

# Clone or navigate to the localstag directory
cd localstag
```

### Step 2: Install Dependencies

```bash
# Download Go module dependencies
go mod tidy

# Verify dependencies are installed
go mod download
```

### Step 3: Build the Services

```bash
# Build all services using make
make build

# This creates:
# - ./local-pipeline (main controller)
# - ./bin/stag (Stag service)
# - ./bin/relay (Relay service)
```

**Verify the build:**
```bash
# Check that binaries were created
ls -la bin/
ls -la local-pipeline

# Should see:
# bin/relay
# bin/stag
# local-pipeline
```

### Step 4: Initialize the Database

```bash
# Initialize a new database
make init

# Or manually:
./local-pipeline -init
```

### Step 5: Start the Pipeline

Choose one of these methods to start the services:

#### Option A: Using the Startup Script (Recommended)
```bash
# Start with comprehensive monitoring and output
./start.sh

# You'll see output like:
# 🚀 Starting services...
# 🌍 Stag Service: http://localhost:9000
# 🌐 Relay Service: ws://192.168.1.XXX:8080/ws/streamkit
# ✅ Services Ready!
```

#### Option B: Using Make Commands
```bash
# Start the full pipeline
make run

# Or start individual services:
make run-stag    # Stag service only
make run-relay   # Relay service only
```

#### Option C: Using the Main Controller
```bash
# Start both services
./local-pipeline

# Or start individual services:
./local-pipeline -stag-only
./local-pipeline -relay-only
```

### Step 6: Verify Everything is Working

#### Health Checks:
```bash
# Test Stag service (should return {"status":"healthy"})
curl http://localhost:9000/health

# Test Relay service (should return {"status":"healthy"})
curl http://localhost:8080/health
```

#### Run Tests:
```bash
# Quick validation test
make test

# Comprehensive test
make test-full

# Manual test from tests directory
./tests/quick-test.sh
```

#### Check System Status:
```bash
# View system statistics
./local-pipeline -stats

# List any existing stags
./local-pipeline -list
```

### Step 7: Get Connection Information

Once running, find your connection details:

```bash
# Get your LAN IP for StreamKit connections
./bin/relay -ip

# The output will show something like: "LAN IP: 192.168.1.XXX"
```

## 📱 Connecting StreamKit Devices

### Connection URL for StreamKit:
```
ws://[YOUR_LAN_IP]:8080/ws/streamkit?session_id=your_session&device_id=your_device
```

### Example:
If your LAN IP is `192.168.1.100`, configure StreamKit to connect to:
```
ws://192.168.1.100:8080/ws/streamkit?session_id=my_ar_session&device_id=iphone_12
```

### StreamKit Configuration (iOS):
```swift
// In your StreamKit app
let config = StreamConfigBuilder()
    .websocketURL("ws://192.168.1.100:8080/ws/streamkit")
    .sessionID("my_ar_session")
    .deviceID("iphone_12")
    .build()

let streamKit = try StreamKit.quickStart(
    streams: [.mesh, .camera, .pose],
    config: config
)
```

## 💡 Common Operations

### Basic Usage:
```bash
# Start the pipeline
./start.sh

# In another terminal:
# Monitor data
./local-pipeline -list        # List all spatial graphs (stags)
./local-pipeline -stats       # Show system statistics

# Stop the pipeline
./start.sh stop
```

### Development Workflow:
```bash
# Clean and rebuild
make clean
make build

# Test changes
make test

# View logs
tail -f *.log
```

### Database Management:
```bash
# View stored data
./local-pipeline -list

# Show statistics
./local-pipeline -stats

# Clean all data (careful!)
./local-pipeline -clean

# Reinitialize database
./local-pipeline -init
```

## 🔧 Configuration Options

### Environment Variables:
```bash
# Stag Service Configuration
export STAG_PORT=9000                    # HTTP server port
export STAG_DATABASE_PATH=./my-data      # Database directory
export STAG_LOG_LEVEL=debug              # Log level

# Relay Service Configuration  
export STAG_RELAY_ENDPOINT=http://localhost:9000/api/v1/ingest
```

### Custom Database Location:
```bash
# Use custom database path
./local-pipeline -db ./my-custom-data

# Or set environment variable
export STAG_DATABASE_PATH=./my-custom-data
./local-pipeline
```

### Logging Levels:
```bash
# Set log level (debug, info, warn, error)
./local-pipeline -log-level debug
```

## 📊 API Endpoints

### Stag Service (Port 9000):
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/v1/ingest` | POST | Ingest spatial events |
| `/api/v1/stags` | GET | List all stags |
| `/api/v1/stags/{id}` | GET | Get specific stag |
| `/api/v1/stags/{id}/anchors` | GET | List anchors in stag |
| `/api/v1/stags/{id}/anchors/{anchor_id}` | GET | Get specific anchor |
| `/api/v1/stags/{id}/anchors/{anchor_id}/history` | GET | Anchor version history |
| `/api/v1/stats` | GET | System statistics |

### Relay Service (Port 8080):
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/stats` | GET | Connection statistics |
| `/ws/streamkit` | WebSocket | StreamKit connection endpoint |

## 🧪 Testing

### Quick Test:
```bash
# Run validation test
make test

# Or manually
./tests/quick-test.sh
```

### Comprehensive Test:
```bash
# Run full pipeline test
make test-full

# Or manually
./tests/full-pipeline-test.sh
```

### Manual API Testing:
```bash
# Send test spatial data
curl -X POST http://localhost:9000/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "batch_id": "test-batch",
    "events": [{
      "event_id": "test-event",
      "event_type": "mesh",
      "timestamp": "2024-01-01T12:00:00Z",
      "session_id": "test-session",
      "client_id": "test-client",
      "device_id": "test-device",
      "frame_number": 1,
      "mesh_data": {
        "anchor_id": "test-anchor",
        "vertices": [0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0],
        "faces": [0, 1, 2]
      }
    }]
  }'

# Verify data was stored
curl http://localhost:9000/api/v1/stags
```

## 🐛 Troubleshooting

### "Port already in use"
```bash
# Check what's using the ports
lsof -i :8080  # Relay port
lsof -i :9000  # Stag port

# Kill processes if needed
pkill -f bin/relay
pkill -f bin/stag

# Or use the stop script
./start.sh stop
```

### "Build errors"
```bash
# Update dependencies
go mod tidy
make deps

# Clean and rebuild
make clean
make build
```

### "Can't connect from StreamKit device"
1. **Check firewall**: Ensure port 8080 is allowed
2. **Verify IP**: Use `./bin/relay -ip` to get correct LAN IP  
3. **Same network**: Ensure device is on same WiFi network
4. **Test locally first**: Try `ws://localhost:8080/ws/streamkit`

### "Database errors"
```bash
# Stop all services first
./start.sh stop

# Clean and reinitialize
make clean
./local-pipeline -clean
./local-pipeline -init
```

### Debug Mode:
```bash
# Run with debug logging
./local-pipeline -log-level debug

# Or with startup script
LOG_LEVEL=debug ./start.sh
```

### Reset Everything:
```bash
# Clean everything and start fresh
make clean
rm -rf stag-data
make build
make init
./start.sh
```

## 📁 Project Structure

```
localstag/
├── cmd/                     # Service entry points
│   ├── stag/main.go        # Stag service
│   ├── relay/main.go       # Relay service
│   └── test-client/main.go # Test client
├── internal/               # Core implementation
│   ├── config/             # Configuration management
│   ├── logging/            # Structured logging
│   ├── stag/              # Stag service logic
│   ├── relay/             # Relay service logic
│   └── storage/           # Data storage layer
├── tests/                  # Test scripts
├── local-pipeline.go       # Main executable
├── start.sh               # Startup script
├── Makefile               # Build automation
└── README.md              # This file
```

## 🔄 Data Flow

```
StreamKit Device → WiFi/LAN → Relay Service → HTTP/JSON → Stag Service → BoltDB
    (iOS/AR)         WebSocket     (Port 8080)      REST API    (Port 9000)    (Disk)
```

## 📈 Monitoring

### Real-time Logs:
```bash
# View live logs
tail -f *.log

# Or use startup script
./start.sh logs
```

### Health Monitoring:
```bash
# Automated health checks
watch -n 5 'curl -s http://localhost:8080/health | jq .status'
watch -n 5 'curl -s http://localhost:9000/health | jq .status'
```

### Statistics Dashboard:
```bash
# View real-time stats
watch -n 2 'curl -s http://localhost:9000/api/v1/stats | jq .'
```

## 🆘 Getting Help

### Quick Diagnostics:
```bash
# System info
./local-pipeline -version

# Service status
./start.sh status

# Recent logs
tail -50 *.log

# Network info
./bin/relay -ip
```

### Common Commands:
```bash
# Show all available commands
make help

# Get startup script help
./start.sh help

# Test everything
make test-full
```

## 🎯 What's Next?

Once your pipeline is running:

1. **Connect StreamKit**: Use the WebSocket URL shown in the startup output
2. **Monitor Data**: Use `./local-pipeline -list` to see incoming spatial data
3. **Explore APIs**: Check the API endpoints for querying your spatial graphs
4. **Develop**: Start building your spatial computing applications!

---

**🎉 You're ready to start building spatial computing experiences with Tabular!**

The pipeline provides a complete local infrastructure for AR/VR development with real-time spatial data streaming, persistent storage, and comprehensive monitoring. Happy building! 🥽✨