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
db = client["sample_mflix"]

# Insert a document
insert_result = db.movies.insert_one({
    "title": "The Favourite MongoDB Movie",
    "genres": ["Drama", "History"],
    "runtime": 121,
    "rated": "R",
    "year": 2018,
    "directors": ["Yorgos Lanthimos"],
    "cast": ["Olivia Colman", "Emma Stone", "Rachel Weisz"],
    "type": "movie"
})

print(f"Inserted document ID: {insert_result.inserted_id}")

# Find all documents
for doc in db.movies.find():
    pprint(doc)
