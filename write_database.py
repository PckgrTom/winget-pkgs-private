import psycopg2
import json
from dotenv import load_dotenv
import os

# Load environment variables from .env file
load_dotenv('.env.local')

# Check if data.json exists, relative to path of this python file
if not os.path.exists('data.json'):
    print('data.json does not exist')
    exit()

# Get environment variables
host = os.getenv('PGHOST')
user = os.getenv('PGUSER')
dbname = os.getenv('PGDATABASE')
password = os.getenv('PGPASSWORD')

# check if environment variables are set
if not host or not user or not dbname or not password:
    print('Environment variables not set')
    exit()
else:
    print('Environment variables set')
    print('Continuing...')

# Connect to your postgres DB
print('Connecting to database...')
conn = psycopg2.connect(dbname=dbname, user=user, password=password, host=host)

# Open a cursor to perform database operations
cur = conn.cursor()

# Open the JSON file
print('Loading data.json...')
with open('data.json') as f:
    data = json.load(f)

# If the table does not exist, create it
cur.execute("""
    CREATE TABLE IF NOT EXISTS packages (
        "PackageIdentifier" TEXT PRIMARY KEY,
        "PackageVersion" TEXT NOT NULL,
        "PackageName" TEXT NOT NULL,
        "Publisher" TEXT NOT NULL,
        "Moniker" TEXT,
        "ProductCode" TEXT[],
        "Commands" TEXT[],
        "Tags" TEXT[],
        "PackageFamilyName" TEXT[]
    )
""")

# Delete all rows from the table
print('Deleting all rows from table...')
cur.execute("TRUNCATE TABLE packages")

# Use INSERT INTO to add the new data
# json_populate_recordset function can help to convert json to record set
print('Inserting new data...')
cur.execute("""
    INSERT INTO packages
    SELECT * FROM json_populate_recordset(NULL::packages, %s)
""", (json.dumps(data),))

# Commit the changes
print('Committing changes...')
conn.commit()

# Close the cursor and connection
cur.close()
conn.close()
print('Done!')
