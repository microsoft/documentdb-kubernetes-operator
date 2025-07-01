#!/bin/bash

# MongoDB Connection Test Script
# Tests MongoDB connection using mongosh with comprehensive validation

set -e

# Default values
ARCHITECTURE=""
NAMESPACE=""
CLUSTER_NAME=""
POD_NAME=""
PORT=""
USERNAME=""
PASSWORD=""
TEST_TYPE="comprehensive"

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
    echo "  --test-type TYPE       Test type (basic, comprehensive)"
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
        --test-type)
            TEST_TYPE="$2"
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

echo "Testing connection with mongosh on $ARCHITECTURE architecture..."
echo "Using pod: $POD_NAME"
echo "Port: $PORT"
echo "Test type: $TEST_TYPE"

# Function to setup port forwarding with retry logic
setup_port_forward() {
    local max_attempts=3
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        echo "Port forward setup attempt $attempt/$max_attempts..."
        
        # Start port-forward in background
        kubectl port-forward pod/$POD_NAME $PORT:$PORT -n $NAMESPACE > /tmp/mongosh_pf.log 2>&1 &
        PF_PID=$!
        echo $PF_PID > /tmp/mongosh_pf.pid
        
        # Wait for port-forward to establish
        echo "Waiting for port-forward to establish..."
        sleep 10
        
        # Check if port-forward process is still running
        if ! kill -0 $PF_PID 2>/dev/null; then
            echo "❌ Port-forward process died (attempt $attempt)"
            if [ -f /tmp/mongosh_pf.log ]; then
                echo "Port-forward output:"
                cat /tmp/mongosh_pf.log
            fi
            ((attempt++))
            sleep 5
            continue
        fi
        
        # Test connection
        echo "Testing port-forward connection..."
        timeout 60 bash -c "
        until nc -z 127.0.0.1 $PORT; do
            echo 'Waiting for port-forward to be ready...'
            sleep 2
        done
        " && {
            echo "✓ Port-forward established successfully"
            return 0
        }
        
        echo "❌ Port-forward connection test failed (attempt $attempt)"
        kill $PF_PID 2>/dev/null || true
        ((attempt++))
        sleep 5
    done
    
    echo "❌ Failed to establish port-forward after $max_attempts attempts"
    return 1
}

# Function to cleanup port forwarding
cleanup_port_forward() {
    if [ -f /tmp/mongosh_pf.pid ]; then
        PF_PID=$(cat /tmp/mongosh_pf.pid)
        kill $PF_PID 2>/dev/null || true
        rm -f /tmp/mongosh_pf.pid
    fi
    rm -f /tmp/mongosh_pf.log
}

# Setup port forwarding
if ! setup_port_forward; then
    echo "❌ Failed to setup port forwarding"
    exit 1
fi

echo "Port-forward is ready, creating mongosh test script..."

# Create comprehensive test script
cat > /tmp/test_mongosh.js << 'MONGOSH_SCRIPT'
// Comprehensive MongoDB Connection Test Script
print("=== Starting MongoDB Connection Test ===");
print("Connected to DocumentDB!");

// Switch to test database
db = db.getSiblingDB('mongosh_test_db');
print("Using database: mongosh_test_db");

// Test 1: Basic Connection and Database Operations
print("\n=== Test 1: Basic Connection and Database Operations ===");

// Drop collection if it exists (cleanup from previous runs)
db.test_collection.drop();

// Create collection and insert test data
print("Creating collection and inserting test data...");
db.createCollection("test_collection");

var testData = [
  { name: "Alice", age: 30, department: "Engineering", salary: 75000 },
  { name: "Bob", age: 25, department: "Marketing", salary: 55000 },
  { name: "Charlie", age: 35, department: "Sales", salary: 65000 },
  { name: "Diana", age: 28, department: "Engineering", salary: 70000 },
  { name: "Eve", age: 32, department: "Marketing", salary: 60000 }
];

var insertResult = db.test_collection.insertMany(testData);
print("Inserted documents:", Object.keys(insertResult.insertedIds).length);

// Validate insertion
var insertedCount = Object.keys(insertResult.insertedIds).length;
if (insertedCount !== 5) {
  throw new Error("Expected 5 inserted documents, got " + insertedCount);
}
print("✓ Insertion validation passed");

// Test 2: Query Operations
print("\n=== Test 2: Query Operations ===");

// Count documents
var totalDocs = db.test_collection.countDocuments({});
print("Total documents:", totalDocs);
if (totalDocs !== 5) {
  throw new Error("Expected 5 total documents, found " + totalDocs);
}
print("✓ Document count validation passed");

// Query with filters
var engineers = db.test_collection.find({ department: "Engineering" }).toArray();
print("Engineers found:", engineers.length);
if (engineers.length !== 2) {
  throw new Error("Expected 2 engineers, found " + engineers.length);
}
print("✓ Department filter validation passed");

// Range query
var youngEmployees = db.test_collection.find({ age: { $lt: 30 } }).toArray();
print("Employees under 30:", youngEmployees.length);
if (youngEmployees.length !== 2) {
  throw new Error("Expected 2 employees under 30, found " + youngEmployees.length);
}
print("✓ Range query validation passed");

// Test 3: Aggregation Operations
print("\n=== Test 3: Aggregation Operations ===");

// Average age calculation
var avgAgeResult = db.test_collection.aggregate([
  { $group: { _id: null, avgAge: { $avg: "$age" }, count: { $sum: 1 } } }
]).toArray();

var avgAge = avgAgeResult[0].avgAge;
var expectedAvgAge = (30 + 25 + 35 + 28 + 32) / 5; // 30
print("Average age:", avgAge, "Expected:", expectedAvgAge);

if (Math.abs(avgAge - expectedAvgAge) > 0.01) {
  throw new Error("Expected average age " + expectedAvgAge + ", got " + avgAge);
}
print("✓ Aggregation validation passed");

// Group by department
var deptStats = db.test_collection.aggregate([
  { $group: { 
      _id: "$department", 
      count: { $sum: 1 }, 
      avgSalary: { $avg: "$salary" },
      maxSalary: { $max: "$salary" }
    }},
  { $sort: { _id: 1 } }
]).toArray();

print("Department statistics:", JSON.stringify(deptStats));
if (deptStats.length !== 3) {
  throw new Error("Expected 3 departments, found " + deptStats.length);
}
print("✓ Department grouping validation passed");

// Test 4: Update Operations
print("\n=== Test 4: Update Operations ===");

// Update single document
var updateResult = db.test_collection.updateOne(
  { name: "Alice" },
  { $set: { title: "Senior Engineer", lastModified: new Date() } }
);

print("Update result - Modified:", updateResult.modifiedCount, "Matched:", updateResult.matchedCount);
if (updateResult.modifiedCount !== 1 || updateResult.matchedCount !== 1) {
  throw new Error("Expected 1 modified and 1 matched document, got modified=" + updateResult.modifiedCount + ", matched=" + updateResult.matchedCount);
}
print("✓ Single update validation passed");

// Verify update content
var aliceUpdated = db.test_collection.findOne({ name: "Alice" });
if (!aliceUpdated.title || aliceUpdated.title !== "Senior Engineer") {
  throw new Error("Alice title update validation failed: " + JSON.stringify(aliceUpdated));
}
print("✓ Update content validation passed");

// Bulk update
var bulkUpdateResult = db.test_collection.updateMany(
  { salary: { $lt: 60000 } },
  { $inc: { salary: 5000 }, $set: { salaryAdjusted: true } }
);

print("Bulk update result - Modified:", bulkUpdateResult.modifiedCount);
if (bulkUpdateResult.modifiedCount !== 1) { // Only Bob should match
  throw new Error("Expected 1 document to be updated in bulk operation, got " + bulkUpdateResult.modifiedCount);
}
print("✓ Bulk update validation passed");

// Test 5: Sorting and Limiting
print("\n=== Test 5: Sorting and Limiting Operations ===");

// Sort by age ascending
var sortedByAge = db.test_collection.find().sort({ age: 1 }).toArray();
var ages = sortedByAge.map(doc => doc.age);
print("Ages in ascending order:", ages);

// Verify sorting
for (var i = 1; i < ages.length; i++) {
  if (ages[i] < ages[i-1]) {
    throw new Error("Sorting validation failed: ages not in ascending order");
  }
}
print("✓ Sorting validation passed");

// Test limit and skip
var limitedResults = db.test_collection.find().sort({ age: 1 }).limit(2).toArray();
if (limitedResults.length !== 2) {
  throw new Error("Expected 2 documents with limit, got " + limitedResults.length);
}
print("✓ Limit operation validation passed");

var skippedResults = db.test_collection.find().sort({ age: 1 }).skip(2).limit(2).toArray();
if (skippedResults.length !== 2) {
  throw new Error("Expected 2 documents with skip+limit, got " + skippedResults.length);
}
print("✓ Skip operation validation passed");

// Test 6: Index Operations (if supported)
print("\n=== Test 6: Index Operations ===");

try {
  // Create index
  var indexResult = db.test_collection.createIndex({ department: 1 });
  print("Index created:", indexResult);
  
  // List indexes
  var indexes = db.test_collection.getIndexes();
  print("Total indexes:", indexes.length);
  
  print("✓ Index operations completed");
} catch (e) {
  print("Index operations not fully supported or failed:", e.message);
  // This is not a critical failure for the test
}

// Test 7: Complex Aggregation Pipeline
print("\n=== Test 7: Complex Aggregation Pipeline ===");

var complexPipeline = [
  { $match: { age: { $gte: 25 } } },
  { $group: { 
      _id: "$department", 
      avgAge: { $avg: "$age" },
      totalSalary: { $sum: "$salary" },
      employees: { $push: "$name" }
    }},
  { $project: {
      department: "$_id",
      avgAge: { $round: ["$avgAge", 1] },
      totalSalary: 1,
      employeeCount: { $size: "$employees" },
      employees: 1
    }},
  { $sort: { totalSalary: -1 } }
];

var complexResult = db.test_collection.aggregate(complexPipeline).toArray();
print("Complex aggregation result:", JSON.stringify(complexResult, null, 2));

if (complexResult.length === 0) {
  throw new Error("Complex aggregation returned no results");
}
print("✓ Complex aggregation validation passed");

// Test 8: Delete Operations
print("\n=== Test 8: Delete Operations ===");

// Insert a temporary document for deletion test
var tempInsert = db.test_collection.insertOne({ name: "Temp", age: 99, department: "Temp", temporary: true });
print("Temporary document inserted:", tempInsert.insertedId);

// Delete the temporary document
var deleteResult = db.test_collection.deleteOne({ temporary: true });
print("Delete result - Deleted count:", deleteResult.deletedCount);

if (deleteResult.deletedCount !== 1) {
  throw new Error("Expected 1 document to be deleted, got " + deleteResult.deletedCount);
}
print("✓ Delete operation validation passed");

// Verify document was deleted
var tempDoc = db.test_collection.findOne({ temporary: true });
if (tempDoc !== null) {
  throw new Error("Temporary document was not properly deleted");
}
print("✓ Delete verification passed");

// Final validation - ensure we still have our original data
var finalCount = db.test_collection.countDocuments({});
if (finalCount !== 5) {
  throw new Error("Expected 5 documents after cleanup, found " + finalCount);
}
print("✓ Final document count validation passed");

// Test Summary
print("\n=== Test Summary ===");
print("✓ All mongosh tests completed successfully!");
print("✓ Basic connection: PASSED");
print("✓ Query operations: PASSED");
print("✓ Aggregation operations: PASSED");
print("✓ Update operations: PASSED");
print("✓ Sorting and limiting: PASSED");
print("✓ Index operations: COMPLETED");
print("✓ Complex aggregation: PASSED");
print("✓ Delete operations: PASSED");
print("✓ Data integrity: VERIFIED");

print("\nMongoDB connection test completed successfully!");
MONGOSH_SCRIPT

echo "Running mongosh validation tests..."

# Run the comprehensive test script
if mongosh 127.0.0.1:$PORT \
  -u "$USERNAME" \
  -p "$PASSWORD" \
  --authenticationMechanism SCRAM-SHA-256 \
  --tls \
  --tlsAllowInvalidCertificates \
  --file /tmp/test_mongosh.js; then
  echo "✓ Mongosh validation tests completed successfully on $ARCHITECTURE"
else
  echo "❌ Mongosh validation tests failed on $ARCHITECTURE"
  echo "=== Port-forward logs ==="
  cat /tmp/mongosh_pf.log 2>/dev/null || echo "No port-forward logs available"
  cleanup_port_forward
  exit 1
fi

# Cleanup
cleanup_port_forward
rm -f /tmp/test_mongosh.js

echo "✓ MongoDB connection test completed successfully on $ARCHITECTURE"
