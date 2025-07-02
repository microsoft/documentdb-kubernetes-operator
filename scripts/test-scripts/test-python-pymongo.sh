#!/bin/bash

# Python PyMongo Integration Test Script
# Tests MongoDB connection using PyMongo with comprehensive validation

set -e

# Default values
ARCHITECTURE=""
NAMESPACE=""
CLUSTER_NAME=""
POD_NAME=""
PORT=""
USERNAME=""
PASSWORD=""

# Function to display usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo "Options:"
    echo "  --architecture ARCH    Target architecture for logging"
    echo "  --namespace NS         Kubernetes namespace"
    echo "  --cluster-name NAME    DocumentDB cluster name"
    echo "  --pod-name NAME        Pod name (optional, defaults to CLUSTER_NAME-1)"
    echo "  --port PORT            Port to forward and connect to"
    echo "  --username USER        MongoDB username"
    echo "  --password PASS        MongoDB password"
    echo "  --help                 Show this help"
    exit 1
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --architecture)
            ARCHITECTURE="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --pod-name)
            POD_NAME="$2"
            shift 2
            ;;
        --port)
            PORT="$2"
            shift 2
            ;;
        --username)
            USERNAME="$2"
            shift 2
            ;;
        --password)
            PASSWORD="$2"
            shift 2
            ;;
        --help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate required parameters
if [[ -z "$ARCHITECTURE" || -z "$NAMESPACE" || -z "$CLUSTER_NAME" || -z "$PORT" || -z "$USERNAME" || -z "$PASSWORD" ]]; then
    echo "Error: Missing required parameters"
    usage
fi

# Set default pod name if not provided
if [[ -z "$POD_NAME" ]]; then
    POD_NAME="${CLUSTER_NAME}-1"
fi

echo "Testing with Python PyMongo client on $ARCHITECTURE architecture..."
echo "Using pod: $POD_NAME"
echo "Port: $PORT"

# Function to setup port forwarding with retry logic
setup_port_forward() {
    local max_attempts=3
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        echo "Attempt $attempt: Setting up port forwarding to pod $POD_NAME in namespace $NAMESPACE..."
        
        # Start port forward in background
        kubectl port-forward "pod/$POD_NAME" "$PORT:$PORT" -n "$NAMESPACE" &
        PF_PID=$!
        
        # Give it some time to start
        sleep 5
        
        # Check if port forward is working by testing the connection
        if timeout 30 bash -c "until nc -z 127.0.0.1 $PORT; do echo 'Waiting for port-forward...'; sleep 2; done"; then
            echo "✓ Port forwarding established successfully on attempt $attempt"
            return 0
        else
            echo "❌ Port forwarding failed on attempt $attempt"
            kill $PF_PID 2>/dev/null || true
            sleep 2
        fi
        
        ((attempt++))
    done
    
    echo "❌ Failed to establish port forwarding after $max_attempts attempts"
    return 1
}

# Function to cleanup port forwarding
cleanup_port_forward() {
    if [[ -n "$PF_PID" ]]; then
        echo "Cleaning up port forwarding (PID: $PF_PID)..."
        kill $PF_PID 2>/dev/null || true
        wait $PF_PID 2>/dev/null || true
        PF_PID=""
    fi
}

# Set up cleanup trap
trap cleanup_port_forward EXIT

# Install Python dependencies
echo "Installing Python dependencies..."
pip install pymongo

# Setup port forwarding
if ! setup_port_forward; then
    echo "Failed to setup port forwarding"
    exit 1
fi

# Test connection and ensure port-forward is ready
echo "Verifying port-forward is ready..."
timeout 60 bash -c "
until nc -z 127.0.0.1 $PORT; do
  echo 'Waiting for port-forward to be ready...'
  sleep 2
done
"

echo "Port-forward is ready, running Python tests..."

# Run the existing Python test script and validate it completes successfully
cd scripts/test-scripts
echo "Running existing Python test script on $ARCHITECTURE..."
if python3 mongo-python-data-pusher.py; then
    echo "✓ Existing Python test script completed successfully on $ARCHITECTURE"
else
    echo "❌ Existing Python test script failed on $ARCHITECTURE"
    exit 1
fi
cd - > /dev/null

# Create and run comprehensive additional Python tests
cat > additional_test.py << EOF
from pymongo import MongoClient
import ssl
import sys

def validate_test(condition, message):
    if not condition:
        print(f"❌ VALIDATION FAILED: {message}")
        sys.exit(1)
    print(f"✓ {message}")

# Connection parameters
client = MongoClient(
    "127.0.0.1",
    $PORT,
    username="$USERNAME",
    password="$PASSWORD",
    authSource="admin",
    authMechanism="SCRAM-SHA-256",
    tls=True,
    tlsAllowInvalidCertificates=True
)

# Test database operations
test_db = client["integration_test"]

# Test collection operations
collection = test_db["test_collection"]

# Clear any existing data
collection.drop()

# Insert test data and validate
docs = [
    {"type": "integration_test", "value": i, "status": "active"}
    for i in range(10)
]
result = collection.insert_many(docs)
print(f"Inserted {len(result.inserted_ids)} documents")

# Validate insertion
validate_test(len(result.inserted_ids) == 10, "Inserted exactly 10 documents")
validate_test(all(isinstance(id, object) for id in result.inserted_ids), "All inserted IDs are valid ObjectIds")

# Test queries and validate results
count = collection.count_documents({"status": "active"})
print(f"Found {count} active documents")
validate_test(count == 10, "Found exactly 10 active documents")

# Test specific value queries
value_5_docs = list(collection.find({"value": 5}))
validate_test(len(value_5_docs) == 1, "Found exactly 1 document with value 5")
validate_test(value_5_docs[0]["value"] == 5, "Document with value 5 has correct value")
validate_test(value_5_docs[0]["status"] == "active", "Document with value 5 has correct status")
validate_test(value_5_docs[0]["type"] == "integration_test", "Document with value 5 has correct type")

# Test range queries
high_value_docs = list(collection.find({"value": {"\\\$gte": 7}}))
validate_test(len(high_value_docs) == 3, "Found exactly 3 documents with value >= 7")
expected_values = {7, 8, 9}
found_values = {doc["value"] for doc in high_value_docs}
validate_test(found_values == expected_values, f"High value documents have correct values: {found_values}")

# Test aggregation and validate results
pipeline = [
    {"\\\$match": {"status": "active"}},
    {"\\\$group": {"_id": "\\\$status", "total": {"\\\$sum": "\\\$value"}, "count": {"\\\$sum": 1}}}
]
agg_result = list(collection.aggregate(pipeline))
print(f"Aggregation result: {agg_result}")

validate_test(len(agg_result) == 1, "Aggregation returned exactly 1 group")
validate_test(agg_result[0]["_id"] == "active", "Aggregation grouped by 'active' status")
expected_total = sum(range(10))  # 0+1+2+...+9 = 45
validate_test(agg_result[0]["total"] == expected_total, f"Aggregation total is correct: {expected_total}")
validate_test(agg_result[0]["count"] == 10, "Aggregation count is correct: 10")

# Test update operations
update_result = collection.update_many(
    {"value": {"\\\$lt": 5}},
    {"\\\$set": {"status": "updated"}}
)
validate_test(update_result.modified_count == 5, f"Updated exactly 5 documents (got {update_result.modified_count})")

# Validate update results
updated_docs = list(collection.find({"status": "updated"}))
validate_test(len(updated_docs) == 5, "Found exactly 5 updated documents")
updated_values = {doc["value"] for doc in updated_docs}
expected_updated_values = {0, 1, 2, 3, 4}
validate_test(updated_values == expected_updated_values, f"Updated documents have correct values: {updated_values}")

# Test that non-updated documents are unchanged
active_docs = list(collection.find({"status": "active"}))
validate_test(len(active_docs) == 5, "Found exactly 5 still-active documents")
active_values = {doc["value"] for doc in active_docs}
expected_active_values = {5, 6, 7, 8, 9}
validate_test(active_values == expected_active_values, f"Active documents have correct values: {active_values}")

# Test sorting
sorted_docs = list(collection.find().sort("value", -1))  # Descending order
validate_test(len(sorted_docs) == 10, "Sorted query returned all 10 documents")
sorted_values = [doc["value"] for doc in sorted_docs]
expected_sorted = list(range(9, -1, -1))  # [9, 8, 7, 6, 5, 4, 3, 2, 1, 0]
validate_test(sorted_values == expected_sorted, f"Documents sorted correctly: {sorted_values}")

# Test complex aggregation with multiple stages
complex_pipeline = [
    {"\\\$match": {"value": {"\\\$gte": 3}}},
    {"\\\$group": {"_id": "\\\$status", "avg_value": {"\\\$avg": "\\\$value"}, "max_value": {"\\\$max": "\\\$value"}}},
    {"\\\$sort": {"_id": 1}}
]
complex_result = list(collection.aggregate(complex_pipeline))
print(f"Complex aggregation result: {complex_result}")

# Validate complex aggregation
validate_test(len(complex_result) == 2, "Complex aggregation returned 2 groups (active and updated)")

# Find the results for each status
active_result = next((r for r in complex_result if r["_id"] == "active"), None)
updated_result = next((r for r in complex_result if r["_id"] == "updated"), None)

validate_test(active_result is not None, "Found active group in complex aggregation")
validate_test(updated_result is not None, "Found updated group in complex aggregation")

# For active status: values 5,6,7,8,9 -> avg = 7, max = 9
validate_test(abs(active_result["avg_value"] - 7.0) < 0.001, f"Active group avg_value is correct: {active_result['avg_value']}")
validate_test(active_result["max_value"] == 9, f"Active group max_value is correct: {active_result['max_value']}")

# For updated status: values 3,4 (only those >= 3) -> avg = 3.5, max = 4
validate_test(abs(updated_result["avg_value"] - 3.5) < 0.001, f"Updated group avg_value is correct: {updated_result['avg_value']}")
validate_test(updated_result["max_value"] == 4, f"Updated group max_value is correct: {updated_result['max_value']}")

print("All Python integration tests passed with validation!")
print(f"Test completed successfully on architecture: {sys.platform}")

client.close()
EOF

echo "Running Python validation tests on $ARCHITECTURE..."
if python3 additional_test.py; then
    echo "✓ Python validation tests completed successfully on $ARCHITECTURE"
else
    echo "❌ Python validation tests failed on $ARCHITECTURE"
    exit 1
fi

# Cleanup temporary test file
rm -f additional_test.py

echo "✅ All Python PyMongo tests completed successfully!"
