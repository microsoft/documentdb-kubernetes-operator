from pymongo import MongoClient
from pprint import pprint
import ssl

# Connection parameters
host = "127.0.0.1" # Use localhost for local testing or replace with the actual load balancer endpoint
port = 10260
username = "default_user"
password = "Admin100" # Default is Admin100
auth_db = "admin"  # Default auth source unless otherwise needed

# Connect with TLS and skip cert validation
client = MongoClient(
    host,
    port,
    username=username,
    password=password,
    authSource=auth_db,
    authMechanism="SCRAM-SHA-256",
    tls=True,
    tlsAllowInvalidCertificates=True
)

# Use the database
club_db = client["soccer_league"]

# Insert a soccer club document
insert_result = club_db.clubs.insert_one({
    "name": "Manchester United",
    "country": "England",
    "founded": 1878,
    "stadium": "Old Trafford",
    "league": "Premier League",
    "titles": ["Premier League", "FA Cup", "Champions League"]
})

print(f"Inserted soccer club document ID: {insert_result.inserted_id}")

# Find all soccer clubs
for doc in club_db.clubs.find():
    pprint(doc)
