import psycopg2
import json
from dotenv import load_dotenv
import os

# Load environment variables from .env file
load_dotenv('.env.local')

# Check if data.json exists, relative to path of this python file
data_json_path = os.path.join(os.path.dirname(__file__), 'data.json')
if not os.path.exists(data_json_path):
    print('data.json does not exist')
    exit()

# check if environment variables are set
if not os.getenv('DATABASE_URL'):
    print('Could not find DATABASE_URL environment variable')
    exit()
else:
    print('Found DATABASE_URL environment variable')
    print('Continuing...')

# Connect to your postgres DB
print('Connecting to database...')
conn = psycopg2.connect(os.getenv('DATABASE_URL'))

# Open a cursor to perform database operations
cur = conn.cursor()

# Open the JSON file
print('Loading data.json...')
with open(data_json_path, encoding='utf8') as f:
    data = json.load(f)

# Drop the table if it already exists
cur.execute("""
    DROP TABLE IF EXISTS packages;
    CREATE TABLE packages (
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
