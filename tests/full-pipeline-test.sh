#!/bin/bash

# Comprehensive test script for Tabular Local Pipeline
set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test configuration
STAG_PORT=9000
RELAY_PORT=8080
TEST_SESSION="test-session-$(date +%s)"
TEST_DEVICE="test-device-$(date +%s)"

print_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

print_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

print_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

# Change to parent directory
cd "$(dirname "$0")/.."

# Function to check if service is healthy
check_service_health() {
    local url=$1
    local service_name=$2
    
    if curl -s "$url" > /dev/null 2>&1; then
        print_pass "$service_name is healthy"
        return 0
    else
        print_fail "$service_name is not healthy"
        return 1
    fi
}

# Function to wait for service
wait_for_service() {
    local url=$1
    local service_name=$2
    local max_attempts=30
    local attempt=1
    
    print_test "Waiting for $service_name to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "$url" > /dev/null 2>&1; then
            print_pass "$service_name is ready"
            return 0
        fi
        sleep 1
        ((attempt++))
    done
    
    print_fail "$service_name failed to start within $max_attempts seconds"
    return 1
}

# Function to test REST API endpoints
test_rest_apis() {
    print_test "Testing REST API endpoints..."
    
    # Test Stag service health
    if curl -s "http://localhost:$STAG_PORT/health" | grep -q "healthy"; then
        print_pass "Stag health endpoint working"
    else
        print_fail "Stag health endpoint failed"
        return 1
    fi
    
    # Test Relay service health
    if curl -s "http://localhost:$RELAY_PORT/health" | grep -q "healthy"; then
        print_pass "Relay health endpoint working"
    else
        print_fail "Relay health endpoint failed"
        return 1
    fi
    
    # Test Stag stats endpoint
    if curl -s "http://localhost:$STAG_PORT/api/v1/stats" | grep -q "stag_count"; then
        print_pass "Stag stats endpoint working"
    else
        print_fail "Stag stats endpoint failed"
        return 1
    fi
    
    # Test Relay stats endpoint
    if curl -s "http://localhost:$RELAY_PORT/stats" | grep -q "active_connections"; then
        print_pass "Relay stats endpoint working"
    else
        print_fail "Relay stats endpoint failed"
        return 1
    fi
    
    return 0
}

# Function to test data ingestion
test_data_ingestion() {
    print_test "Testing data ingestion..."
    
    # Create test spatial event
    local test_event=$(cat << EOF
{
    "batch_id": "test-batch-$(date +%s)",
    "events": [
        {
            "event_id": "test-event-1",
            "event_type": "mesh",
            "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
            "server_timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
            "session_id": "$TEST_SESSION",
            "client_id": "test-client",
            "device_id": "$TEST_DEVICE",
            "frame_number": 1,
            "mesh_data": {
                "anchor_id": "test-anchor-1",
                "vertices": [0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0.0],
                "faces": [0, 1, 2],
                "transform": {
                    "translation": [0.0, 0.0, 0.0],
                    "rotation": [0.0, 0.0, 0.0, 1.0],
                    "scale": [1.0, 1.0, 1.0]
                }
            },
            "metadata": {},
            "processing_info": {
                "received_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
                "processed_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
                "relay": "test-relay"
            }
        }
    ],
    "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "relay_id": "test-relay",
    "processing_info": {
        "received_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
        "processed_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
        "relay": "test-relay"
    }
}
EOF
)
    
    # Send test event to Stag service
    local response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$test_event" \
        "http://localhost:$STAG_PORT/api/v1/ingest")
    
    if echo "$response" | grep -q "processed"; then
        print_pass "Data ingestion successful"
        
        # Verify data was stored
        sleep 1
        local stags=$(curl -s "http://localhost:$STAG_PORT/api/v1/stags")
        if echo "$stags" | grep -q "$TEST_SESSION"; then
            print_pass "Data persistence verified"
        else
            print_fail "Data persistence failed"
            return 1
        fi
    else
        print_fail "Data ingestion failed: $response"
        return 1
    fi
    
    return 0
}

# Function to test CLI operations
test_cli_operations() {
    print_test "Testing CLI operations..."
    
    # Test list command
    if ./local-pipeline -list | grep -q "Total Stags"; then
        print_pass "List command working"
    else
        print_fail "List command failed"
        return 1
    fi
    
    # Test stats command
    if ./local-pipeline -stats | grep -q "System Statistics"; then
        print_pass "Stats command working"
    else
        print_fail "Stats command failed"
        return 1
    fi
    
    return 0
}

# Function to cleanup on exit
cleanup_test() {
    print_test "Cleaning up test environment..."
    
    if [ -f "test-stag.pid" ]; then
        local stag_pid=$(cat test-stag.pid)
        if kill -0 $stag_pid 2>/dev/null; then
            kill $stag_pid
            wait $stag_pid 2>/dev/null || true
        fi
        rm -f test-stag.pid
    fi
    
    if [ -f "test-relay.pid" ]; then
        local relay_pid=$(cat test-relay.pid)
        if kill -0 $relay_pid 2>/dev/null; then
            kill $relay_pid
            wait $relay_pid 2>/dev/null || true
        fi
        rm -f test-relay.pid
    fi
    
    # Clean up test files
    rm -rf test-stag-data
    rm -f test-*.log
    
    print_pass "Cleanup complete"
}

# Main test execution
main() {
    print_info "🧪 Starting Tabular Local Pipeline End-to-End Test"
    echo
    
    # Clean up any existing processes
    pkill -f "bin/stag" 2>/dev/null || true
    pkill -f "bin/relay" 2>/dev/null || true
    sleep 2
    
    # Build services
    print_test "Building services..."
    if make build > /dev/null 2>&1; then
        print_pass "Build successful"
    else
        print_fail "Build failed"
        exit 1
    fi
    
    # Start services
    print_test "Starting services..."
    
    # Start Stag service
    ./bin/stag -port $STAG_PORT -db ./test-stag-data -log-level warn > test-stag.log 2>&1 &
    local stag_pid=$!
    echo $stag_pid > test-stag.pid
    
    # Start Relay service
    ./bin/relay -port $RELAY_PORT -stag-endpoint "http://localhost:$STAG_PORT/api/v1/ingest" -log-level warn > test-relay.log 2>&1 &
    local relay_pid=$!
    echo $relay_pid > test-relay.pid
    
    # Wait for services to be ready
    if wait_for_service "http://localhost:$STAG_PORT/health" "Stag service" && \
       wait_for_service "http://localhost:$RELAY_PORT/health" "Relay service"; then
        print_pass "Services started successfully"
    else
        print_fail "Services failed to start"
        cleanup_test
        exit 1
    fi
    
    # Run tests
    local tests_passed=0
    local total_tests=4
    
    # Test 1: Health checks
    if check_service_health "http://localhost:$STAG_PORT/health" "Stag service" && \
       check_service_health "http://localhost:$RELAY_PORT/health" "Relay service"; then
        ((tests_passed++))
    fi
    
    # Test 2: REST APIs
    if test_rest_apis; then
        ((tests_passed++))
    fi
    
    # Test 3: Data ingestion
    if test_data_ingestion; then
        ((tests_passed++))
    fi
    
    # Test 4: CLI operations
    if test_cli_operations; then
        ((tests_passed++))
    fi
    
    # Cleanup
    cleanup_test
    
    # Results
    echo
    print_info "🏁 Test Results: $tests_passed/$total_tests tests passed"
    
    if [ $tests_passed -eq $total_tests ]; then
        print_pass "🎉 All tests passed! Pipeline is working correctly."
        echo
        print_info "You can now use the pipeline with:"
        echo "  ./start.sh                    # Start full pipeline"
        echo "  ./local-pipeline -test        # Run built-in test client"
        echo "  ./local-pipeline -list        # List stags"
        echo "  ./local-pipeline -stats       # Show statistics"
        exit 0
    else
        print_fail "❌ Some tests failed. Check the logs for details."
        echo
        print_info "Log files:"
        echo "  - test-stag.log"
        echo "  - test-relay.log"
        exit 1
    fi
}

# Trap cleanup on exit
trap cleanup_test EXIT

# Run main function
main "$@"