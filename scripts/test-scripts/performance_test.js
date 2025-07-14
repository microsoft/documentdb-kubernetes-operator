// Performance Test Suite with Validation
print("=== Performance Test Suite with Validation ===");

// Validation helper function
function validate(condition, message) {
  if (!condition) {
    print("DEBUG: Performance validation failed for: " + message);
    print("DEBUG: Condition was:", condition);
    throw new Error("PERFORMANCE VALIDATION FAILED: " + message);
  }
  print("✓ " + message);
}

// Helper function to handle Long objects returned by some MongoDB drivers
function getLongValue(val) {
  if (typeof val === 'object' && val !== null && 'low' in val) {
    return val.low; // Extract the actual number from Long object
  }
  return val;
}

db = db.getSiblingDB('perftest');

// Large dataset insertion test
print("\n--- Large Dataset Insertion Test ---");
var startTime = new Date();
var docs = [];
for (let i = 0; i < 1000; i++) {
  docs.push({
    id: i,
    name: "User " + i,
    email: "user" + i + "@example.com",
    data: "This is sample data for user " + i,
    timestamp: new Date(),
    metadata: {
      source: "performance_test",
      batch: Math.floor(i / 100),
      random: Math.random()
    }
  });
}

validate(docs.length === 1000, "Created exactly 1000 test documents");

var insertStart = new Date();
var result = db.perfcollection.insertMany(docs);
var insertEnd = new Date();

// Debug the insert result
print("DEBUG: performance insertMany result:", JSON.stringify(result));

// Helper function to get insertedIds count (handles both array and object formats)
function getInsertedCount(result) {
  if (result.insertedIds) {
    if (Array.isArray(result.insertedIds)) {
      return result.insertedIds.length;
    } else if (typeof result.insertedIds === 'object') {
      return Object.keys(result.insertedIds).length;
    }
  }
  return 0;
}

var insertTime = insertEnd - insertStart;
validate(result.acknowledged === true, "Insertion was acknowledged");
validate(getInsertedCount(result) === 1000, "Inserted exactly 1000 documents");
validate(insertTime < 10000, "Insertion completed within 10 seconds (took " + insertTime + "ms)");

print("Inserted", getInsertedCount(result), "documents in", insertTime, "ms");

// Query performance test
print("\n--- Query Performance Test ---");

var queryStart = new Date();
var count = db.perfcollection.countDocuments();
var queryEnd = new Date();

var countTime = queryEnd - queryStart;
validate(count === 1000, "Count query returned correct result: 1000");
validate(countTime < 5000, "Count query completed within 5 seconds (took " + countTime + "ms)");

print("Count query took", countTime, "ms, result:", count);

// Range query performance test
print("\n--- Range Query Performance Test ---");

var queryStart2 = new Date();
var rangeResults = db.perfcollection.find({ id: { $gte: 500 } }).toArray();
var queryEnd2 = new Date();

var rangeTime = queryEnd2 - queryStart2;
validate(rangeResults.length === 500, "Range query returned exactly 500 documents");
validate(rangeTime < 5000, "Range query completed within 5 seconds (took " + rangeTime + "ms)");

// Validate range query results
var minId = Math.min(...rangeResults.map(r => r.id));
var maxId = Math.max(...rangeResults.map(r => r.id));
validate(minId === 500, "Minimum ID in range results is 500");
validate(maxId === 999, "Maximum ID in range results is 999");

print("Range query found", rangeResults.length, "documents in", rangeTime, "ms");

// Aggregation performance
print("\n--- Aggregation Performance Test ---");

var aggStart = new Date();
var aggResult = db.perfcollection.aggregate([
  { $match: { id: { $gte: 100 } } },
  { $group: { _id: "$metadata.batch", count: { $sum: 1 }, avgId: { $avg: "$id" } } },
  { $sort: { _id: 1 } }
]).toArray();
var aggEnd = new Date();

var aggTime = aggEnd - aggStart;
validate(aggResult.length === 9, "Aggregation returned 9 batches (batches 1-9)"); // 100-999 = batches 1-9
validate(aggTime < 5000, "Aggregation completed within 5 seconds (took " + aggTime + "ms)");

// Validate aggregation results
var totalDocs = aggResult.reduce((sum, batch) => sum + batch.count, 0);
validate(totalDocs === 900, "Aggregation processed exactly 900 documents (id >= 100)");

// Check specific batch
var batch5 = aggResult.find(r => r._id === 5);
validate(batch5 && batch5.count === 100, "Batch 5 has exactly 100 documents");
validate(batch5 && Math.abs(batch5.avgId - 549.5) < 0.1, "Batch 5 average ID is correct (~549.5)");

print("Aggregation processed", aggResult.length, "groups in", aggTime, "ms");

// Test sorting performance
print("\n--- Sorting Performance Test ---");

var sortStart = new Date();
var sortedResults = db.perfcollection.find({ id: { $lt: 100 } }).sort({ id: -1 }).toArray();
var sortEnd = new Date();

var sortTime = sortEnd - sortStart;
validate(sortedResults.length === 100, "Sort query returned exactly 100 documents");
validate(sortTime < 3000, "Sort query completed within 3 seconds (took " + sortTime + "ms)");

// Validate sorting
validate(sortedResults[0].id === 99, "First document has ID 99 (descending sort)");
validate(sortedResults[99].id === 0, "Last document has ID 0 (descending sort)");

for (let i = 0; i < sortedResults.length - 1; i++) {
  validate(sortedResults[i].id > sortedResults[i + 1].id, "Documents are sorted in descending order");
}

print("Sort query processed", sortedResults.length, "documents in", sortTime, "ms");

// Test update performance
print("\n--- Update Performance Test ---");

var updateStart = new Date();
var updateResult = db.perfcollection.updateMany(
  { "metadata.batch": { $in: [0, 1, 2] } },
  { $set: { updated: true, updateTime: new Date() } }
);
var updateEnd = new Date();

var updateTime = updateEnd - updateStart;

var perfUpdateMatchedCount = getLongValue(updateResult.matchedCount);
var perfUpdateModifiedCount = getLongValue(updateResult.modifiedCount);

validate(perfUpdateMatchedCount === 300, "Update matched exactly 300 documents (3 batches × 100)");
validate(perfUpdateModifiedCount === 300, "Update modified exactly 300 documents");
validate(updateTime < 3000, "Update completed within 3 seconds (took " + updateTime + "ms)");

// Verify updates
var updatedDocs = db.perfcollection.find({ updated: true }).toArray();
validate(updatedDocs.length === 300, "Found exactly 300 updated documents");
validate(updatedDocs.every(doc => doc.updateTime !== undefined), "All updated docs have updateTime");

print("Update modified", perfUpdateModifiedCount, "documents in", updateTime, "ms");

// Test delete performance
print("\n--- Delete Performance Test ---");

var deleteStart = new Date();
var deleteResult = db.perfcollection.deleteMany({ id: { $gte: 950 } });
var deleteEnd = new Date();

var deleteTime = deleteEnd - deleteStart;

var perfDeletedCount = getLongValue(deleteResult.deletedCount);

validate(perfDeletedCount === 50, "Deleted exactly 50 documents (IDs 950-999)");
validate(deleteTime < 2000, "Delete completed within 2 seconds (took " + deleteTime + "ms)");

// Verify deletions
var remainingCount = db.perfcollection.countDocuments();
validate(remainingCount === 950, "Exactly 950 documents remain after deletion");

var deletedDocs = db.perfcollection.find({ id: { $gte: 950 } }).toArray();
validate(deletedDocs.length === 0, "No documents with ID >= 950 remain");

print("Delete removed", perfDeletedCount, "documents in", deleteTime, "ms");

// Overall performance summary
print("\n--- Performance Summary ---");
var totalTime = new Date() - startTime;
validate(totalTime < 30000, "All performance tests completed within 30 seconds (took " + totalTime + "ms)");

print("Total performance test time:", totalTime, "ms");
print("Insert rate:", Math.round(1000 / (insertTime / 1000)), "docs/sec");
print("Query rate:", Math.round(1000 / (countTime / 1000)), "queries/sec");
print("Update rate:", Math.round(300 / (updateTime / 1000)), "updates/sec");
print("Delete rate:", Math.round(50 / (deleteTime / 1000)), "deletes/sec");

// Cleanup with validation
var dropStart = new Date();
db.perfcollection.drop();
var dropEnd = new Date();

var dropTime = dropEnd - dropStart;
validate(dropTime < 2000, "Collection drop completed within 2 seconds (took " + dropTime + "ms)");

// Verify cleanup
var collections = db.getCollectionNames();
validate(!collections.includes("perfcollection"), "Performance collection was dropped");

print("\n=== Performance Tests Completed Successfully with Validation! ===");
