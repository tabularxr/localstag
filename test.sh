#!/bin/bash

# Unified End-to-End Test Script for Tabular Local Pipeline
# This script tests the complete localstag pipeline and stag queryability

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
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

print_header() {
    echo ""
    echo -e "${BLUE}================================================${NC}"
    echo -e "${BLUE} $1 ${NC}"
    echo -e "${BLUE}================================================${NC}"
    echo ""
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

# Function to cleanup on exit
cleanup_test() {
    print_test "Cleaning up test environment..."
    
    # Kill services by PID files
    for pidfile in test-stag.pid test-relay.pid; do
        if [ -f "$pidfile" ]; then
            local pid=$(cat "$pidfile")
            if kill -0 $pid 2>/dev/null; then
                kill $pid
                wait $pid 2>/dev/null || true
            fi
            rm -f "$pidfile"
        fi
    done
    
    # Kill by process name as backup
    pkill -f "bin/stag" 2>/dev/null || true
    pkill -f "bin/relay" 2>/dev/null || true
    
    # Clean up test files
    rm -rf test-stag-data test-data
    rm -f test-*.log *.pid *.log
    
    print_pass "Cleanup complete"
}

# Test 1: Build verification
test_build() {
    print_header "TEST 1: BUILD VERIFICATION"
    
    print_test "Building services..."
    if make build > /dev/null 2>&1; then
        print_pass "Build successful"
        
        # Verify binaries exist
        if [[ -f "./local-pipeline" && -f "./bin/stag" && -f "./bin/relay" ]]; then
            print_pass "All binaries created successfully"
            return 0
        else
            print_fail "Some binaries missing"
            return 1
        fi
    else
        print_fail "Build failed"
        return 1
    fi
}

# Test 2: Service startup and health
test_service_startup() {
    print_header "TEST 2: SERVICE STARTUP & HEALTH"
    
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
        return 0
    else
        print_fail "Services failed to start"
        return 1
    fi
}

# Test 3: API endpoints
test_api_endpoints() {
    print_header "TEST 3: API ENDPOINT VERIFICATION"
    
    # Test Stag service endpoints
    print_test "Testing Stag service endpoints..."
    
    if curl -s "http://localhost:$STAG_PORT/health" | grep -q "healthy"; then
        print_pass "Stag health endpoint working"
    else
        print_fail "Stag health endpoint failed"
        return 1
    fi
    
    if curl -s "http://localhost:$STAG_PORT/api/v1/stats" | grep -q "stag_count"; then
        print_pass "Stag stats endpoint working"
    else
        print_fail "Stag stats endpoint failed"
        return 1
    fi
    
    # Test Relay service endpoints
    print_test "Testing Relay service endpoints..."
    
    if curl -s "http://localhost:$RELAY_PORT/health" | grep -q "healthy"; then
        print_pass "Relay health endpoint working"
    else
        print_fail "Relay health endpoint failed"
        return 1
    fi
    
    if curl -s "http://localhost:$RELAY_PORT/stats" | grep -q "active_connections"; then
        print_pass "Relay stats endpoint working"
    else
        print_fail "Relay stats endpoint failed"
        return 1
    fi
    
    return 0
}

# Test 4: Data ingestion and persistence
test_data_ingestion() {
    print_header "TEST 4: DATA INGESTION & PERSISTENCE"
    
    print_test "Testing data ingestion pipeline..."
    
    # Create test spatial event (compatible with both data formats)
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
    
    if echo "$response" | grep -q "processed\|success"; then
        print_pass "Data ingestion successful"
        
        # Verify data was stored
        sleep 2
        local stags=$(curl -s "http://localhost:$STAG_PORT/api/v1/stags")
        if echo "$stags" | grep -q "$TEST_SESSION"; then
            print_pass "Data persistence verified"
            return 0
        else
            print_fail "Data persistence failed"
            print_info "Stags response: $stags"
            return 1
        fi
    else
        print_fail "Data ingestion failed: $response"
        return 1
    fi
}

# Test 5: Stag queryability and fetching
test_stag_queryability() {
    print_header "TEST 5: STAG QUERYABILITY & FETCHING"
    
    print_test "Testing stag query capabilities..."
    
    # Test listing stags
    print_test "Testing stag listing..."
    local stags_list=$(curl -s "http://localhost:$STAG_PORT/api/v1/stags")
    if echo "$stags_list" | jq . > /dev/null 2>&1; then
        print_pass "Stag listing endpoint working"
        
        # Extract stag ID for further testing
        local stag_id=$(echo "$stags_list" | jq -r '.[0].id // empty' 2>/dev/null || echo "$TEST_SESSION")
        
        if [ ! -z "$stag_id" ]; then
            print_pass "Found stag ID: $stag_id"
            
            # Test getting specific stag
            print_test "Testing specific stag query..."
            local stag_response=$(curl -s "http://localhost:$STAG_PORT/api/v1/stags/$stag_id")
            if echo "$stag_response" | jq . > /dev/null 2>&1; then
                print_pass "Specific stag query working"
                
                # Test getting anchors for this stag
                print_test "Testing stag anchors query..."
                local anchors_response=$(curl -s "http://localhost:$STAG_PORT/api/v1/stags/$stag_id/anchors")
                if echo "$anchors_response" | jq . > /dev/null 2>&1; then
                    print_pass "Stag anchors query working"
                    
                    # Test getting anchor history
                    print_test "Testing anchor history query..."
                    local history_response=$(curl -s "http://localhost:$STAG_PORT/api/v1/stags/$stag_id/anchors/test-anchor-1/history")
                    if echo "$history_response" | jq . > /dev/null 2>&1; then
                        print_pass "Anchor history query working"
                    else
                        print_fail "Anchor history query failed"
                        print_info "Response: $history_response"
                        return 1
                    fi
                else
                    print_fail "Stag anchors query failed"
                    print_info "Response: $anchors_response"
                    return 1
                fi
            else
                print_fail "Specific stag query failed"
                print_info "Response: $stag_response"
                return 1
            fi
        else
            print_fail "No stag ID found"
            return 1
        fi
    else
        print_fail "Stag listing endpoint failed"
        print_info "Response: $stags_list"
        return 1
    fi
    
    return 0
}

# Test 6: CLI operations
test_cli_operations() {
    print_header "TEST 6: CLI OPERATIONS"
    
    print_test "Testing CLI operations..."
    
    # Test list command
    if ./local-pipeline -list | grep -q "Total Stags\|Stag List"; then
        print_pass "List command working"
    else
        print_fail "List command failed"
        return 1
    fi
    
    # Test stats command
    if ./local-pipeline -stats | grep -q "System Statistics\|Statistics"; then
        print_pass "Stats command working"
    else
        print_fail "Stats command failed"
        return 1
    fi
    
    return 0
}

# Test 7: Performance validation
test_performance() {
    print_header "TEST 7: PERFORMANCE VALIDATION"
    
    print_test "Testing performance with multiple events..."
    
    # Create batch with multiple events
    local batch_events=""
    for i in {1..10}; do
        if [ $i -gt 1 ]; then
            batch_events+=","
        fi
        batch_events+=$(cat << EOF
        {
            "event_id": "perf-event-$i",
            "event_type": "mesh",
            "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
            "server_timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
            "session_id": "$TEST_SESSION",
            "client_id": "test-client",
            "device_id": "$TEST_DEVICE",
            "frame_number": $i,
            "mesh_data": {
                "anchor_id": "perf-anchor-$i",
                "vertices": [$i.0, 0.0, 0.0, $(($i + 1)).0, 0.0, 0.0, $i.0, 1.0, 0.0],
                "faces": [0, 1, 2],
                "transform": {
                    "translation": [$i.0, 0.0, 0.0],
                    "rotation": [0.0, 0.0, 0.0, 1.0],
                    "scale": [1.0, 1.0, 1.0]
                }
            },
            "metadata": {"batch_index": $i},
            "processing_info": {
                "received_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
                "processed_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
                "relay": "test-relay"
            }
        }
EOF
)
    done
    
    local perf_batch=$(cat << EOF
{
    "batch_id": "perf-batch-$(date +%s)",
    "events": [$batch_events],
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
    
    # Time the request
    local start_time=$(date +%s.%3N)
    local response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$perf_batch" \
        "http://localhost:$STAG_PORT/api/v1/ingest")
    local end_time=$(date +%s.%3N)
    
    local duration=$(echo "$end_time - $start_time" | bc -l 2>/dev/null || echo "N/A")
    
    if echo "$response" | grep -q "processed\|success"; then
        print_pass "Performance test passed (${duration}s for 10 events)"
        return 0
    else
        print_fail "Performance test failed: $response"
        return 1
    fi
}

# Main test execution
main() {
    print_info "🧪 Starting Comprehensive Tabular Local Pipeline Test"
    print_info "Test Session ID: ${TEST_SESSION}"
    print_info "Test Device ID: ${TEST_DEVICE}"
    echo
    
    # Clean up any existing processes first
    cleanup_test
    sleep 2
    
    # Run all tests
    local tests_passed=0
    local total_tests=7
    
    # Test 1: Build
    if test_build; then
        ((tests_passed++))
    fi
    
    # Test 2: Service startup
    if test_service_startup; then
        ((tests_passed++))
    else
        print_fail "Cannot continue without running services"
        cleanup_test
        exit 1
    fi
    
    # Test 3: API endpoints
    if test_api_endpoints; then
        ((tests_passed++))
    fi
    
    # Test 4: Data ingestion
    if test_data_ingestion; then
        ((tests_passed++))
    fi
    
    # Test 5: Stag queryability
    if test_stag_queryability; then
        ((tests_passed++))
    fi
    
    # Test 6: CLI operations
    if test_cli_operations; then
        ((tests_passed++))
    fi
    
    # Test 7: Performance
    if test_performance; then
        ((tests_passed++))
    fi
    
    # Cleanup
    cleanup_test
    
    # Results
    echo
    print_header "TEST RESULTS"
    print_info "🏁 Tests completed: $tests_passed/$total_tests tests passed"
    
    if [ $tests_passed -eq $total_tests ]; then
        print_pass "🎉 ALL TESTS PASSED! LocalStag pipeline is working correctly."
        echo
        print_info "✅ LocalStag end-to-end functionality: VERIFIED"
        print_info "✅ Stag queryability and fetching: VERIFIED" 
        print_info "✅ Data persistence and retrieval: VERIFIED"
        echo
        print_info "Usage Instructions:"
        echo "  ./start.sh                    # Start full pipeline"
        echo "  ./local-pipeline -list        # List stored stags"
        echo "  ./local-pipeline -stats       # Show system statistics"
        echo
        print_info "Connection Instructions:"
        echo "  WebSocket URL: ws://\$(./bin/relay -ip):8080/ws/streamkit"
        echo "  REST API URL: http://localhost:9000/api/v1/"
        echo
        exit 0
    else
        print_fail "❌ SOME TESTS FAILED ($((total_tests - tests_passed)) failures)"
        echo
        print_info "Check log files for details:"
        echo "  - test-stag.log"
        echo "  - test-relay.log"
        echo
        exit 1
    fi
}

# Trap cleanup on exit
trap cleanup_test EXIT

# Run main function
main "$@"