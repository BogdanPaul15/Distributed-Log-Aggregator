#!/bin/bash

STACK_NAME="log_stack"
GENERATOR_URL="http://localhost:8081"

function show_menu() {
    echo "=============================="
    echo "   Log Stack Management"
    echo "=============================="
    echo "1. Set Log Generation Rate"
    echo "2. Scale Log Ingestor"
    echo "3. Scale Log Consumer"
    echo "4. Show Service Status"
    echo "5. Exit"
    echo "=============================="
}

function set_rate() {
    read -p "Enter new log rate (logs/sec): " rate
    if [[ ! "$rate" =~ ^[0-9]+$ ]]; then
        echo "Error: Rate must be an integer."
        return
    fi
    echo "Setting rate to $rate..."
    # The log-generator API expects POST /rate with {"rate": <int>}
    response=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d "{\"rate\": $rate}" "$GENERATOR_URL/rate")
    
    if [ "$response" -eq 200 ]; then
        echo "✅ Success: Rate updated."
    else
        echo "❌ Error: Failed to update rate (HTTP $response). Is log-generator running at $GENERATOR_URL?"
    fi
    echo ""
}

function scale_service() {
    service_short_name=$1
    service_name="${STACK_NAME}_${service_short_name}"
    
    read -p "Enter number of replicas for $service_short_name: " replicas
    if [[ ! "$replicas" =~ ^[0-9]+$ ]]; then
        echo "Error: Replicas must be an integer."
        return
    fi
    
    echo "Scaling $service_name to $replicas..."
    docker service scale "$service_name=$replicas"
}

function show_status() {
    echo "Current Stack Services:"
    docker service ls --filter "name=${STACK_NAME}"
}

# Check if curl is installed
if ! command -v curl &> /dev/null; then
    echo "Error: curl is required for this script."
    exit 1
fi

while true; do
    show_menu
    read -p "Select an option: " choice
    echo ""
    case $choice in
        1) set_rate ;;
        2) scale_service "log-ingestor" ;;
        3) scale_service "log-consumer" ;;
        4) show_status ;;
        5) exit 0 ;;
        *) echo "Invalid option." ;;
    esac
    echo ""
done
