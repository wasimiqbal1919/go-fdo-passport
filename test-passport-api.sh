#!/bin/bash

# Direct test of Passport API integration
# This can be run from Ubuntu terminal with VPN access

echo "Testing Passport API Integration..."
echo ""

# Generate a unique test UUID
TEST_UUID=$(uuidgen 2>/dev/null || echo "test-$(date +%s)-$(shuf -i 1000-9999 -n 1)")

# Generate timestamp (nanoseconds since Unix epoch)
TIMESTAMP=$(date +%s%N)

echo "Test Parameters:"
echo "  Controller UUID: $TEST_UUID"
echo "  Timestamp: $TIMESTAMP"
echo ""

# Test 1: Create commissioning passport (matches our integration)
echo "1. Testing POST /create-comissioning-passport..."
echo ""

curl -X POST http://cmulk1.cymanii.org:8000/create-comissioning-passport \
     -H "Content-Type: application/json" \
     -d "{
           \"controller_uuid\": \"$TEST_UUID\",
           \"cert\": \"type_10_enc_1_body_6d6f636b2d6d616e7566616374757265722d7075626c69632d6b65792d646174612d666f722d6465766963652d6365727469666963617465\",
           \"deployed_location\": \"Ubuntu Test Environment - FDO Integration\",
           \"timestamp\": \"$TIMESTAMP\"
         }" \
     -w "\n\nHTTP Status: %{http_code}\nTotal Time: %{time_total}s\n" \
     -v

echo ""
echo "---"
echo ""

# Test 2: Query the passport (if query endpoint exists)
echo "2. Testing GET /product_item/?uuid=..."
echo ""

curl -X GET "http://cmulk1.cymanii.org:8000/product_item/?uuid=$TEST_UUID" \
     -H "Accept: application/json" \
     -w "\n\nHTTP Status: %{http_code}\nTotal Time: %{time_total}s\n" \
     -v

echo ""
echo "---"
echo ""

# Test 3: Using the same format as your original example
echo "3. Testing with your exact example format..."
echo ""

curl -X POST http://cmulk1.cymanii.org:8000/create-comissioning-passport \
     -H "Content-Type: application/json" \
     -d '{
           "controller_uuid": "191e886b-dfff-4f39-9618-d7a364ec0c90",
           "cert": "string",
           "deployed_location": "string", 
           "timestamp": "1754509904342152960"
         }' \
     -w "\n\nHTTP Status: %{http_code}\nTotal Time: %{time_total}s\n"

echo ""
echo "✅ API test complete!"
echo ""
echo "If successful, this proves the integration will work correctly"
echo "when the FDO library calls the same API endpoints."
