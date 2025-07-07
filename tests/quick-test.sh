#!/bin/bash

# Quick test script for local pipeline
set -e

echo "🧪 Quick Pipeline Test"

# Change to parent directory
cd "$(dirname "$0")/.."

# Clean up any existing processes
pkill -f "bin/stag" 2>/dev/null || true
pkill -f "bin/relay" 2>/dev/null || true
sleep 2

# Build first
echo "Building services..."
make build > /dev/null 2>&1 || {
    echo "❌ Build failed"
    exit 1
}

# Start Stag service
echo "Starting Stag service..."
./bin/stag -port 9000 -db ./test-data -log-level info > stag.log 2>&1 &
STAG_PID=$!
echo $STAG_PID > stag.pid

# Wait for Stag to start
sleep 3

# Test Stag health
echo "Testing Stag health..."
STAG_HEALTH=$(curl -s http://localhost:9000/health || echo "failed")
if [[ $STAG_HEALTH == *"healthy"* ]]; then
    echo "✅ Stag service is healthy"
else
    echo "❌ Stag service failed to start"
    kill $STAG_PID 2>/dev/null || true
    exit 1
fi

# Start Relay service
echo "Starting Relay service..."
./bin/relay -port 8080 -stag-endpoint http://localhost:9000/api/v1/ingest -log-level info > relay.log 2>&1 &
RELAY_PID=$!
echo $RELAY_PID > relay.pid

# Wait for Relay to start
sleep 3

# Test Relay health
echo "Testing Relay health..."
RELAY_HEALTH=$(curl -s http://localhost:8080/health || echo "failed")
if [[ $RELAY_HEALTH == *"healthy"* ]]; then
    echo "✅ Relay service is healthy"
else
    echo "❌ Relay service failed to start"
    kill $STAG_PID $RELAY_PID 2>/dev/null || true
    exit 1
fi

# Test data ingestion directly
echo "Testing data ingestion..."
TEST_DATA='{
    "batch_id": "test-batch-1",
    "events": [
        {
            "event_id": "test-event-1",
            "event_type": "mesh",
            "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
            "server_timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
            "session_id": "test-session",
            "client_id": "test-client",
            "device_id": "test-device",
            "frame_number": 1,
            "mesh_data": {
                "anchor_id": "test-anchor-1",
                "vertices": [0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0],
                "faces": [0, 1, 2]
            },
            "metadata": {},
            "processing_info": {
                "received_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
                "processed_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
                "relay": "test-relay"
            }
        }
    ],
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "relay_id": "test-relay",
    "processing_info": {
        "received_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
        "processed_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
        "relay": "test-relay"
    }
}'

INGEST_RESPONSE=$(curl -s -X POST \
    -H "Content-Type: application/json" \
    -d "$TEST_DATA" \
    http://localhost:9000/api/v1/ingest)

if [[ $INGEST_RESPONSE == *"processed"* ]]; then
    echo "✅ Data ingestion successful"
else
    echo "❌ Data ingestion failed: $INGEST_RESPONSE"
    kill $STAG_PID $RELAY_PID 2>/dev/null || true
    exit 1
fi

# Verify data was stored
sleep 1
echo "Verifying data storage..."
STAGS_LIST=$(curl -s http://localhost:9000/api/v1/stags)
if [[ $STAGS_LIST == *"test-session"* ]]; then
    echo "✅ Data storage verified"
else
    echo "❌ Data storage failed"
    kill $STAG_PID $RELAY_PID 2>/dev/null || true
    exit 1
fi

# Test CLI commands
echo "Testing CLI commands..."
./local-pipeline -db ./test-data -stats > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "✅ CLI commands working"
else
    echo "❌ CLI commands failed"
fi

# Clean up
echo "Cleaning up..."
kill $STAG_PID $RELAY_PID 2>/dev/null || true
rm -f *.pid *.log
rm -rf test-data

echo ""
echo "🎉 All tests passed! The pipeline is working correctly."
echo ""
echo "Usage:"
echo "  ./start.sh              # Start the full pipeline"
echo "  ./local-pipeline -list  # List stags"
echo "  ./local-pipeline -stats # Show statistics"
echo ""
echo "WebSocket URL for StreamKit:"
echo "  ws://$(./bin/relay -ip):8080/ws/streamkit"