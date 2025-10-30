#!/usr/bin/env python3

import sys
import time
from pymongo import MongoClient, errors
from datetime import datetime

if len(sys.argv) != 2:
    print(f"Usage: python insert_test.py <connection_string>")
    sys.exit(1)

connection_string = sys.argv[1]

client = MongoClient(connection_string)

db = client.testdb
collection = db.testcollection

print(f"{'Inserted Document':<30} {'Insert Count':<15}")
print("-" * 77)
start_time = time.time()
end_time = start_time + (10 * 60)  # 10 minutes
count = 0

while time.time() < end_time:
    failed = False
    write_result = ""
    try:
        doc = {
            "count": count,
            "message": f"Insert operation {count}"
        }
        result = collection.insert_one(doc)
        write_result = result.inserted_id
        count += 1
        print(f"{str(write_result):<30} {count:<15}")
    except Exception as e:
        failed = True
        short_err = getattr(getattr(e, 'details', {}), 'get', lambda *_: None)('errmsg')
        print(f"Error: {short_err or str(e)}")

    
    time.sleep(1)  

print(f"Completed {count} insert operations in 10 minutes")
final_read_count = collection.count_documents({})
print(f"Final read count: {final_read_count}")
client.close()
