#!/bin/bash

# Comprehensive test script for bmwtools session isolation
# This script uploads two different JSON files in parallel and verifies that 
# each session only sees its own data across multiple API endpoints

# Command-line arguments check
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <json_file1> <json_file2>"
    echo "***REMOVED*** $0 file1.json file2.json"
    exit 1
fi

# Check if files exist
if [ ! -f "$1" ]; then
    echo "Error: File $1 does not exist"
    exit 1
fi

if [ ! -f "$2" ]; then
    echo "Error: File $2 does not exist"
    exit 1
fi

JSON_FILE1="$1"
JSON_FILE2="$2"
SERVER_URL="${SERVER_URL:-http://localhost:8050}"
OUTPUT_DIR=$(mktemp -d)
DELAY_SECONDS=2
MAX_RETRIES=3

echo "================= COMPREHENSIVE SESSION ISOLATION TEST =================="
echo "Server URL: $SERVER_URL"
echo "JSON File 1: $JSON_FILE1"
echo "JSON File 2: $JSON_FILE2"
echo "Output directory: $OUTPUT_DIR"
echo "======================================================================"

# Function to check if the server is running
check_server() {
    local retries=0
    while [ $retries -lt $MAX_RETRIES ]; do
        echo "Checking server at ${SERVER_URL}..."
        SERVER_RESPONSE=$(curl -s "${SERVER_URL}/api/version")
        if [ $? -eq 0 ] && [ -n "$SERVER_RESPONSE" ]; then
            echo "Server is running. Server version: $SERVER_RESPONSE"
            
            # Test a cookie-based endpoint with no cookie to verify the expected behavior
            echo "Testing cookie handling with no cookies..."
            NO_COOKIE_RESPONSE=$(curl -s "${SERVER_URL}/api/sessions")
            if [[ "$NO_COOKIE_RESPONSE" == "[]" || "$NO_COOKIE_RESPONSE" == "{\"error\":\"*\"}" ]]; then
                echo "Server correctly returns empty data when no session cookie is provided"
            else
                echo "WARNING: Server returns data without session cookie. Response: ${NO_COOKIE_RESPONSE:0:100}..."
            fi
            return 0
        fi
        retries=$((retries + 1))
        echo "Server not responding, retrying in 2 seconds... ($retries/$MAX_RETRIES)"
        sleep 2
    done
    echo "Error: Could not connect to server at ${SERVER_URL}"
    exit 1
}

# Function to perform comprehensive tests for session 1
run_comprehensive_test_session_1() {
    echo "[Session 1] Starting comprehensive tests..."
    
    # Upload JSON file
    echo "[Session 1] Uploading $JSON_FILE1"
    UPLOAD_RESPONSE=$(curl -s -X POST \
        -F "file=@$JSON_FILE1" \
        -F "consent=false" \
        -c "${OUTPUT_DIR}/cookies_1.txt" \
        -v \
        "${SERVER_URL}/api/upload" 2> "${OUTPUT_DIR}/curl_verbose_1.txt")
    
    # Debug cookie information
    echo "[Session 1] Cookies received:"
    cat "${OUTPUT_DIR}/cookies_1.txt"
    
    # Extract session_id cookie specifically
    SESSION_ID=$(grep -A 10 "Set-Cookie:" "${OUTPUT_DIR}/curl_verbose_1.txt" | grep "session_id" | head -1 | sed -n 's/.*session_id=\([^;]*\).*/\1/p')
    echo "[Session 1] Session ID from cookies: $SESSION_ID"
    
    # Extract the session count
    SESSION_COUNT=$(echo "$UPLOAD_RESPONSE" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    echo "[Session 1] Uploaded $SESSION_COUNT sessions"
    echo "$UPLOAD_RESPONSE" > "${OUTPUT_DIR}/upload_response_1.json"
    
    # Wait to ensure interleaving with session 2
    sleep $DELAY_SECONDS
    
    # Test API endpoints
    test_endpoints 1 "${OUTPUT_DIR}/cookies_1.txt" "$SESSION_COUNT"
    
    # Wait and test again to check for leaks
    sleep $((DELAY_SECONDS * 2))
    echo "[Session 1] Testing endpoints again after delay (checking for leaks)"
    test_endpoints 1 "${OUTPUT_DIR}/cookies_1.txt" "$SESSION_COUNT" "_after_delay"
    
    echo "[Session 1] Comprehensive tests complete"
}

# Function to perform comprehensive tests for session 2
run_comprehensive_test_session_2() {
    # Wait before starting to ensure first session is already in progress
    sleep $(($DELAY_SECONDS / 2))
    
    echo "[Session 2] Starting comprehensive tests..."
    
    # Upload JSON file
    echo "[Session 2] Uploading $JSON_FILE2"
    UPLOAD_RESPONSE=$(curl -s -X POST \
        -F "file=@$JSON_FILE2" \
        -F "consent=false" \
        -c "${OUTPUT_DIR}/cookies_2.txt" \
        -v \
        "${SERVER_URL}/api/upload" 2> "${OUTPUT_DIR}/curl_verbose_2.txt")
    
    # Debug cookie information
    echo "[Session 2] Cookies received:"
    cat "${OUTPUT_DIR}/cookies_2.txt"
    
    # Extract session_id cookie specifically
    SESSION_ID=$(grep -A 10 "Set-Cookie:" "${OUTPUT_DIR}/curl_verbose_2.txt" | grep "session_id" | head -1 | sed -n 's/.*session_id=\([^;]*\).*/\1/p')
    echo "[Session 2] Session ID from cookies: $SESSION_ID"
    
    # Extract the session count
    SESSION_COUNT=$(echo "$UPLOAD_RESPONSE" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    echo "[Session 2] Uploaded $SESSION_COUNT sessions"
    echo "$UPLOAD_RESPONSE" > "${OUTPUT_DIR}/upload_response_2.json"
    
    # Wait to ensure interleaving with session 1
    sleep $DELAY_SECONDS
    
    # Test API endpoints
    test_endpoints 2 "${OUTPUT_DIR}/cookies_2.txt" "$SESSION_COUNT"
    
    # Wait and test again to check for leaks
    sleep $((DELAY_SECONDS * 2))
    echo "[Session 2] Testing endpoints again after delay (checking for leaks)"
    test_endpoints 2 "${OUTPUT_DIR}/cookies_2.txt" "$SESSION_COUNT" "_after_delay"
    
    echo "[Session 2] Comprehensive tests complete"
}

# Function to test all relevant API endpoints
test_endpoints() {
    local session_num=$1
    local cookies_file=$2
    local expected_count=$3
    local suffix=$4  # Optional suffix for output files
    
    echo "[Session $session_num] Testing API endpoints..."
    
    # Check cookie file content before using it
    echo "[Session $session_num] Cookie file content:"
    cat "$cookies_file"
    
    # Manually extract session_id cookie for debugging
    SESSION_ID=$(grep "session_id" "$cookies_file" | cut -f7)
    echo "[Session $session_num] Session ID from cookie file: $SESSION_ID"
    
    # Use -v for verbose output to debug cookie issues
    echo "[Session $session_num] Testing /api/sessions endpoint"
    SESSIONS_RESPONSE=$(curl -s -b "$cookies_file" -v "${SERVER_URL}/api/sessions" 2> "${OUTPUT_DIR}/sessions_curl_${session_num}${suffix}.txt")
    
    # Save full response for analysis
    echo "$SESSIONS_RESPONSE" > "${OUTPUT_DIR}/sessions_response_${session_num}${suffix}.json"
    
    # Check if response starts with an array bracket
    if [[ "$SESSIONS_RESPONSE" == \[* ]]; then
        # Count using simple grep for session objects - most reliable across systems
        RETRIEVED_COUNT=$(echo "$SESSIONS_RESPONSE" | grep -o '"avg_power"' | wc -l | tr -d ' ')
    else
        RETRIEVED_COUNT=0
    fi
    
    echo "[Session $session_num] Retrieved $RETRIEVED_COUNT sessions (expected $expected_count)"
    
    # Display raw response for debugging
    echo "[Session $session_num] Raw sessions response (first 200 chars):"
    echo "$SESSIONS_RESPONSE" | head -c 200
    
    # Test /api/stats
    echo "[Session $session_num] Testing /api/stats endpoint"
    STATS_RESPONSE=$(curl -s -b "$cookies_file" "${SERVER_URL}/api/stats")
    echo "$STATS_RESPONSE" > "${OUTPUT_DIR}/stats_response_${session_num}${suffix}.json"
    
    # Test /api/map
    echo "[Session $session_num] Testing /api/map endpoint"
    # Add explicit Cookie header to ensure it's being sent properly
    if [ -n "$SESSION_ID" ]; then
        echo "[Session $session_num] Using explicit session_id cookie: $SESSION_ID"
        MAP_RESPONSE=$(curl -s -b "$cookies_file" -H "Cookie: session_id=$SESSION_ID" -v "${SERVER_URL}/api/map" 2> "${OUTPUT_DIR}/map_curl_${session_num}${suffix}.txt")
    else
        MAP_RESPONSE=$(curl -s -b "$cookies_file" -v "${SERVER_URL}/api/map" 2> "${OUTPUT_DIR}/map_curl_${session_num}${suffix}.txt")
    fi
    echo "$MAP_RESPONSE" > "${OUTPUT_DIR}/map_response_${session_num}${suffix}.json"
    MAP_COUNT=$(echo "$MAP_RESPONSE" | grep -o '"latitude"' | wc -l | tr -d ' ')
    echo "[Session $session_num] Retrieved $MAP_COUNT map locations"
    
    # Test /api/providers
    echo "[Session $session_num] Testing /api/providers endpoint"
    PROVIDERS_RESPONSE=$(curl -s -b "$cookies_file" "${SERVER_URL}/api/providers")
    echo "$PROVIDERS_RESPONSE" > "${OUTPUT_DIR}/providers_response_${session_num}${suffix}.json"
    
    # Verify session count consistency
    clean_retrieved=$(echo "$RETRIEVED_COUNT" | tr -d '\n')
    clean_expected=$(echo "$expected_count" | tr -d '\n')
    
    if [ "$clean_retrieved" = "$clean_expected" ]; then
        echo "[Session $session_num] ✅ Session count test PASSED: Expected $clean_expected, got $clean_retrieved"
    else
        echo "[Session $session_num] ❌ Session count test FAILED: Expected $clean_expected, got $clean_retrieved"
    fi
}

# Function to verify session isolation across all endpoints
verify_comprehensive_isolation() {
    echo "Verifying comprehensive session isolation..."
    
    # Check session data isolation
    SESSION1_IDS=$(cat "${OUTPUT_DIR}/sessions_response_1_after_delay.json" | grep -o '"id":"[^"]*"' | head -5 | sort)
    SESSION2_IDS=$(cat "${OUTPUT_DIR}/sessions_response_2_after_delay.json" | grep -o '"id":"[^"]*"' | head -5 | sort)
    
    # Check if there's any overlap in session IDs
    OVERLAP=$(comm -12 <(echo "$SESSION1_IDS") <(echo "$SESSION2_IDS"))
    
    if [ -z "$OVERLAP" ]; then
        echo "✅ Session data isolation test PASSED: No session data leakage detected"
    else
        echo "❌ Session data isolation test FAILED: Found overlapping session IDs between sessions!"
        echo "Overlapping IDs:"
        echo "$OVERLAP"
    fi
    
    # Check map data isolation
    MAP1_COUNT=$(cat "${OUTPUT_DIR}/map_response_1_after_delay.json" | grep -o '"latitude"' | wc -l | tr -d ' ')
    MAP2_COUNT=$(cat "${OUTPUT_DIR}/map_response_2_after_delay.json" | grep -o '"latitude"' | wc -l | tr -d ' ')
    
    MAP1_FIRST=$(cat "${OUTPUT_DIR}/map_response_1_after_delay.json" | grep -o '"name":"[^"]*"' | head -1)
    MAP2_FIRST=$(cat "${OUTPUT_DIR}/map_response_2_after_delay.json" | grep -o '"name":"[^"]*"' | head -1)
    
    echo "Map locations - Session 1: $MAP1_COUNT, Session 2: $MAP2_COUNT"
    echo "First location - Session 1: $MAP1_FIRST, Session 2: $MAP2_FIRST"
    
    if [ "$MAP1_FIRST" != "$MAP2_FIRST" ]; then
        echo "✅ Map data isolation test PASSED: Different map data between sessions"
    else
        # This is only a warning as it's possible two different files might have some identical locations
        echo "⚠️ Map data isolation WARNING: First location is identical in both sessions"
    fi
    
    # Check stats data isolation
    STATS1_EFFICIENCY=$(cat "${OUTPUT_DIR}/stats_response_1_after_delay.json" | grep -o '"overall_efficiency":[0-9.]*' | cut -d':' -f2)
    STATS2_EFFICIENCY=$(cat "${OUTPUT_DIR}/stats_response_2_after_delay.json" | grep -o '"overall_efficiency":[0-9.]*' | cut -d':' -f2)
    
    echo "Overall efficiency - Session 1: $STATS1_EFFICIENCY, Session 2: $STATS2_EFFICIENCY"
    
    if [ "$STATS1_EFFICIENCY" != "$STATS2_EFFICIENCY" ]; then
        echo "✅ Stats data isolation test PASSED: Different statistics between sessions"
    else
        echo "⚠️ Stats data isolation WARNING: Same efficiency in both sessions (may be coincidental)"
    fi
    
    echo "Test output saved to $OUTPUT_DIR"
    echo "To view the full results, check the JSON files in that directory"
}

# Main execution

# Check if server is running
check_server

# Run both test sessions in parallel
run_comprehensive_test_session_1 &
SESSION1_PID=$!

run_comprehensive_test_session_2 &
SESSION2_PID=$!

# Wait for both processes to finish
wait $SESSION1_PID
wait $SESSION2_PID

# Verify isolation between sessions
verify_comprehensive_isolation

echo "Comprehensive test complete. Results saved in $OUTPUT_DIR"
echo "To view all test data: ls -la $OUTPUT_DIR"
