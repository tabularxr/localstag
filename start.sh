#!/bin/bash

# Tabular Local Pipeline Startup Script
# This script provides a comprehensive way to start and manage the local pipeline

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
STAG_PORT=9000
RELAY_PORT=8080
DB_PATH="./stag-data"
LOG_LEVEL="info"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${PURPLE}$1${NC}"
}

print_command() {
    echo -e "${CYAN}$1${NC}"
}

# Function to check if port is available
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        return 1
    else
        return 0
    fi
}

# Function to get LAN IP
get_lan_ip() {
    local ip
    if command -v ip >/dev/null 2>&1; then
        # Linux
        ip=$(ip route get 8.8.8.8 | sed -n '/src/{s/.*src *\([^ ]*\).*/\1/p;q}')
    elif command -v route >/dev/null 2>&1; then
        # macOS
        ip=$(route get default | grep interface | awk '{print $2}' | xargs ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}' | head -n1)
    else
        # Fallback
        ip=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "localhost")
    fi
    echo "$ip"
}

# Function to wait for service to be ready
wait_for_service() {
    local url=$1
    local service_name=$2
    local max_attempts=30
    local attempt=1
    
    print_status "Waiting for $service_name to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "$url" >/dev/null 2>&1; then
            print_status "$service_name is ready!"
            return 0
        fi
        
        printf "\r  Attempt %d/%d..." $attempt $max_attempts
        sleep 1
        ((attempt++))
    done
    
    print_error "$service_name failed to start within $max_attempts seconds"
    return 1
}

# Function to build services
build_services() {
    print_header "🔨 Building Tabular Local Pipeline..."
    
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found. Please run this script from the project root directory."
        exit 1
    fi
    
    # Create bin directory
    mkdir -p bin
    
    # Build main pipeline controller
    print_status "Building pipeline controller..."
    go build -o local-pipeline . || {
        print_error "Failed to build pipeline controller"
        exit 1
    }
    
    # Build Stag service
    print_status "Building Stag service..."
    go build -o bin/stag ./cmd/stag || {
        print_error "Failed to build Stag service"
        exit 1
    }
    
    # Build Relay service
    print_status "Building Relay service..."
    go build -o bin/relay ./cmd/relay || {
        print_error "Failed to build Relay service"
        exit 1
    }
    
    print_status "Build complete!"
}

# Function to check dependencies
check_dependencies() {
    print_header "🔍 Checking Dependencies..."
    
    # Check Go
    if ! command -v go >/dev/null 2>&1; then
        print_error "Go is not installed. Please install Go 1.21 or later."
        exit 1
    fi
    
    local go_version=$(go version | grep -o 'go[0-9]*\.[0-9]*' | sed 's/go//')
    print_status "Go version: $go_version"
    
    # Check ports
    if ! check_port $STAG_PORT; then
        print_error "Port $STAG_PORT is already in use. Please free this port or change STAG_PORT."
        exit 1
    fi
    
    if ! check_port $RELAY_PORT; then
        print_error "Port $RELAY_PORT is already in use. Please free this port or change RELAY_PORT."
        exit 1
    fi
    
    print_status "All dependencies satisfied!"
}

# Function to start services
start_services() {
    print_header "🚀 Starting Services..."
    
    # Get LAN IP
    local lan_ip=$(get_lan_ip)
    print_status "Detected LAN IP: $lan_ip"
    
    # Start Stag service
    print_status "Starting Stag service on port $STAG_PORT..."
    ./bin/stag -port $STAG_PORT -db "$DB_PATH" -log-level $LOG_LEVEL > stag.log 2>&1 &
    local stag_pid=$!
    echo $stag_pid > stag.pid
    
    # Wait for Stag to be ready
    if ! wait_for_service "http://localhost:$STAG_PORT/health" "Stag service"; then
        print_error "Failed to start Stag service"
        cleanup
        exit 1
    fi
    
    # Start Relay service
    print_status "Starting Relay service on port $RELAY_PORT..."
    ./bin/relay -port $RELAY_PORT -stag-endpoint "http://localhost:$STAG_PORT/api/v1/ingest" -log-level $LOG_LEVEL > relay.log 2>&1 &
    local relay_pid=$!
    echo $relay_pid > relay.pid
    
    # Wait for Relay to be ready
    if ! wait_for_service "http://localhost:$RELAY_PORT/health" "Relay service"; then
        print_error "Failed to start Relay service"
        cleanup
        exit 1
    fi
    
    print_header "✅ Services Ready!"
    echo
    print_status "🌍 Stag Service: http://localhost:$STAG_PORT"
    print_command "   - Health: curl http://localhost:$STAG_PORT/health"
    print_command "   - API: curl http://localhost:$STAG_PORT/api/v1/stags"
    print_command "   - Database: $DB_PATH"
    echo
    print_status "🌐 Relay Service: ws://$lan_ip:$RELAY_PORT/ws/streamkit"
    print_command "   - Local URL: ws://localhost:$RELAY_PORT/ws/streamkit"
    print_command "   - Health: curl http://localhost:$RELAY_PORT/health"
    print_command "   - Stats: curl http://localhost:$RELAY_PORT/stats"
    echo
    print_status "📱 StreamKit Connection:"
    print_command "   URL: ws://$lan_ip:$RELAY_PORT/ws/streamkit"
    print_command "   Parameters: ?session_id=your_session&device_id=your_device"
    echo
    print_status "💡 Management Commands:"
    print_command "   - View logs: tail -f *.log"
    print_command "   - List stags: ./local-pipeline -list"
    print_command "   - Show stats: ./local-pipeline -stats"
    print_command "   - Run test: ./local-pipeline -test"
    print_command "   - Stop services: ./start.sh stop"
    echo
}

# Function to stop services
stop_services() {
    print_header "🛑 Stopping Services..."
    
    if [ -f "stag.pid" ]; then
        local stag_pid=$(cat stag.pid)
        if kill -0 $stag_pid 2>/dev/null; then
            print_status "Stopping Stag service (PID: $stag_pid)..."
            kill $stag_pid
            wait $stag_pid 2>/dev/null || true
        fi
        rm -f stag.pid
    fi
    
    if [ -f "relay.pid" ]; then
        local relay_pid=$(cat relay.pid)
        if kill -0 $relay_pid 2>/dev/null; then
            print_status "Stopping Relay service (PID: $relay_pid)..."
            kill $relay_pid
            wait $relay_pid 2>/dev/null || true
        fi
        rm -f relay.pid
    fi
    
    print_status "Services stopped!"
}

# Function to cleanup on exit
cleanup() {
    stop_services
}

# Function to show status
show_status() {
    print_header "📊 Service Status"
    
    local stag_status="❌ Stopped"
    local relay_status="❌ Stopped"
    
    if [ -f "stag.pid" ] && kill -0 $(cat stag.pid) 2>/dev/null; then
        if curl -s "http://localhost:$STAG_PORT/health" >/dev/null 2>&1; then
            stag_status="✅ Running"
        else
            stag_status="⚠️  Started but not healthy"
        fi
    fi
    
    if [ -f "relay.pid" ] && kill -0 $(cat relay.pid) 2>/dev/null; then
        if curl -s "http://localhost:$RELAY_PORT/health" >/dev/null 2>&1; then
            relay_status="✅ Running"
        else
            relay_status="⚠️  Started but not healthy"
        fi
    fi
    
    echo "Stag Service (port $STAG_PORT):  $stag_status"
    echo "Relay Service (port $RELAY_PORT): $relay_status"
    echo
    
    if [ -f "stag.pid" ] || [ -f "relay.pid" ]; then
        print_status "Log files:"
        print_command "  - tail -f stag.log"
        print_command "  - tail -f relay.log"
    fi
}

# Function to run tests
run_tests() {
    print_header "🧪 Running Tests..."
    
    # Check if services are running
    if ! curl -s "http://localhost:$RELAY_PORT/health" >/dev/null 2>&1; then
        print_error "Relay service is not running. Start services first with: ./start.sh start"
        exit 1
    fi
    
    if ! curl -s "http://localhost:$STAG_PORT/health" >/dev/null 2>&1; then
        print_error "Stag service is not running. Start services first with: ./start.sh start"
        exit 1
    fi
    
    # Run test client
    print_status "Running test client..."
    ./local-pipeline -test
    
    # Show results
    echo
    print_status "Test Results:"
    print_command "  - View stags: ./local-pipeline -list"
    print_command "  - View stats: ./local-pipeline -stats"
}

# Function to show help
show_help() {
    print_header "🦾 Tabular Local Pipeline Startup Script"
    echo
    echo "Usage: $0 [COMMAND]"
    echo
    echo "Commands:"
    echo "  start     Start all services (default)"
    echo "  stop      Stop all services"
    echo "  restart   Restart all services"
    echo "  status    Show service status"
    echo "  test      Run test client"
    echo "  logs      Show live logs"
    echo "  build     Build services only"
    echo "  clean     Clean build artifacts and data"
    echo "  help      Show this help message"
    echo
    echo "Environment Variables:"
    echo "  STAG_PORT     Stag service port (default: $STAG_PORT)"
    echo "  RELAY_PORT    Relay service port (default: $RELAY_PORT)"
    echo "  DB_PATH       Database path (default: $DB_PATH)"
    echo "  LOG_LEVEL     Log level (default: $LOG_LEVEL)"
    echo
    echo "Examples:"
    echo "  $0                   # Start services"
    echo "  $0 start             # Start services"
    echo "  $0 stop              # Stop services"
    echo "  $0 test              # Run tests"
    echo "  STAG_PORT=9001 $0    # Start with custom port"
}

# Function to show logs
show_logs() {
    print_header "📋 Live Logs"
    
    if [ ! -f "stag.log" ] && [ ! -f "relay.log" ]; then
        print_warning "No log files found. Services may not be running."
        return 1
    fi
    
    print_status "Showing live logs (Ctrl+C to exit)..."
    echo
    
    if [ -f "stag.log" ] && [ -f "relay.log" ]; then
        tail -f stag.log relay.log
    elif [ -f "stag.log" ]; then
        tail -f stag.log
    elif [ -f "relay.log" ]; then
        tail -f relay.log
    fi
}

# Function to clean everything
clean_all() {
    print_header "🧹 Cleaning Everything..."
    
    # Stop services first
    stop_services
    
    # Remove build artifacts
    print_status "Removing build artifacts..."
    rm -rf bin/
    rm -f local-pipeline
    rm -f *.log
    rm -f *.pid
    
    # Remove database
    print_status "Removing database..."
    rm -rf "$DB_PATH"
    
    print_status "Clean complete!"
}

# Trap to cleanup on script exit
trap cleanup EXIT

# Main script logic
case "${1:-start}" in
    start)
        check_dependencies
        build_services
        start_services
        
        print_header "🎉 Pipeline Started Successfully!"
        print_warning "Press Ctrl+C to stop all services"
        
        # Wait for interrupt
        while true; do
            sleep 1
        done
        ;;
    stop)
        stop_services
        ;;
    restart)
        stop_services
        sleep 2
        $0 start
        ;;
    status)
        show_status
        ;;
    test)
        run_tests
        ;;
    logs)
        show_logs
        ;;
    build)
        check_dependencies
        build_services
        ;;
    clean)
        clean_all
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        echo
        show_help
        exit 1
        ;;
esac