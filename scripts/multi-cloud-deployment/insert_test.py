#!/usr/bin/env python3

import sys
import time
from pymongo import MongoClient, errors
from datetime import datetime

def ts():
    return datetime.now().strftime("%H:%M:%S")

if len(sys.argv) != 2:
    print(f"[{ts()}] Usage: python insert_test.py <connection_string>")
    sys.exit(1)

connection_string = sys.argv[1]

client = MongoClient(connection_string)

db = client.testdb
collection = db.testcollection

print(f"[{ts()}] Starting insert operations for 10 minutes...")
print(f"[{ts()}] Using: {connection_string.split('@')[1] if '@' in connection_string else 'local'}")
print()
print(f"{'Timestamp':<12} {'Inserted Document':<30} {'Read Count':<15}")
print("-" * 77)
start_time = time.time()
end_time = start_time + (10 * 60)  # 10 minutes
count = 0

while time.time() < end_time:
    failed = False
    write_result = ""
    timestamp = ts()
    try:
        doc = {
            "count": count,
            "message": f"Insert operation {count}"
        }
        result = collection.insert_one(doc)
        write_result = result.inserted_id
        count += 1
    except Exception as e:
        failed = True
        short_err = getattr(getattr(e, 'details', {}), 'get', lambda *_: None)('errmsg')
        print(f"[{timestamp}] Error: {short_err or str(e)}")

    try:
        read_count = collection.count_documents({})
        if not failed:
            print(f"{timestamp:<12} {str(write_result):<30} {read_count:<15}")
        else :
            print(f"{timestamp:<12} {'READ AVAILABLE':<30} {read_count:<15}")
    except Exception as e:
        print(f"[{timestamp}] read error")
        pass
    
    time.sleep(1)  

print(f"[{ts()}] Completed {count} insert operations in 10 minutes")
final_read_count = collection.count_documents({})
print(f"[{ts()}] Final read count: {final_read_count}")
client.close()
