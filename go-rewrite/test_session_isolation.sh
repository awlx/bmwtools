#!/bin/bash

# Test script for bmwtools session isolation
# This script uploads two different JSON files in parallel and verifies that 
# each session only sees its own data

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

echo "=================== SESSION ISOLATION TEST ==================="
echo "Server URL: $SERVER_URL"
echo "JSON File 1: $JSON_FILE1"
echo "JSON File 2: $JSON_FILE2"
echo "Output directory: $OUTPUT_DIR"
echo "=============================================================="

# Function to check if the server is running
check_server() {
    local retries=0
    local max_retries=3
    
    while [ $retries -lt $max_retries ]; do
        echo "Checking server at ${SERVER_URL}..."
        SERVER_RESPONSE=$(curl -s "${SERVER_URL}/api/version")
        if [ $? -eq 0 ] && [ -n "$SERVER_RESPONSE" ]; then
            echo "Server is running. Server version: $SERVER_RESPONSE"
            return 0
        fi
        retries=$((retries + 1))
        echo "Server not responding, retrying in 2 seconds... ($retries/$max_retries)"
        sleep 2
    done
    echo "Error: Could not connect to server at ${SERVER_URL}"
    exit 1
}

# Check if server is running
check_server

# Function to perform uploads and tests for session 1
run_session_1() {
    echo "[Session 1] Starting..."
    
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
    
    SESSION_COUNT=$(echo "$UPLOAD_RESPONSE" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    echo "[Session 1] Uploaded $SESSION_COUNT sessions"
    
    # Wait a bit
    sleep $DELAY_SECONDS
    
    # Get sessions
    echo "[Session 1] Getting sessions"
    # Add explicit Cookie header to ensure it's being sent properly
    if [ -n "$SESSION_ID" ]; then
        echo "[Session 1] Using explicit session_id cookie: $SESSION_ID"
        SESSIONS_RESPONSE=$(curl -s -b "${OUTPUT_DIR}/cookies_1.txt" -H "Cookie: session_id=$SESSION_ID" -v "${SERVER_URL}/api/sessions" 2> "${OUTPUT_DIR}/sessions_curl_1.txt")
    else
        SESSIONS_RESPONSE=$(curl -s -b "${OUTPUT_DIR}/cookies_1.txt" -v "${SERVER_URL}/api/sessions" 2> "${OUTPUT_DIR}/sessions_curl_1.txt")
    fi
    
    # Save full response for analysis
    echo "$SESSIONS_RESPONSE" > "${OUTPUT_DIR}/sessions_response_1_full.json"
    
    # Check if response starts with an array bracket
    if [[ "$SESSIONS_RESPONSE" == \[* ]]; then
        # Count using simple grep for session objects - most reliable across systems
        RETRIEVED_COUNT=$(echo "$SESSIONS_RESPONSE" | grep -o '"avg_power"' | wc -l | tr -d ' ')
    else
        RETRIEVED_COUNT=0
    fi
    
    echo "[Session 1] Retrieved $RETRIEVED_COUNT sessions"
    
    # Display raw response for debugging
    echo "[Session 1] Raw sessions response (first 200 chars):"
    echo "$SESSIONS_RESPONSE" | head -c 200
    
    # Wait a bit more to ensure interleaving with session 2
    sleep $((DELAY_SECONDS * 2))
    
    # Get sessions again to verify no contamination from session 2
    echo "[Session 1] Getting sessions again (checking for leaks)"
    SESSIONS_RESPONSE_2=$(curl -s -b "${OUTPUT_DIR}/cookies_1.txt" "${SERVER_URL}/api/sessions")
    
    # Save full response for analysis
    echo "$SESSIONS_RESPONSE_2" > "${OUTPUT_DIR}/sessions_response_1_leak_check.json"
    
    # Check if response starts with an array bracket
    if [[ "$SESSIONS_RESPONSE_2" == \[* ]]; then
        # Count using simple grep for session objects - most reliable across systems
        RETRIEVED_COUNT_2=$(echo "$SESSIONS_RESPONSE_2" | grep -o '"avg_power"' | wc -l | tr -d ' ')
    else
        RETRIEVED_COUNT_2=0
    fi
    
    echo "[Session 1] Retrieved $RETRIEVED_COUNT_2 sessions"
    
    # Verify counts match
    if [ "$SESSION_COUNT" -eq "$RETRIEVED_COUNT_2" ]; then
        echo "[Session 1] ✅ Test PASSED: Session data is consistent"
    else
        echo "[Session 1] ❌ Test FAILED: Session count mismatch! Expected $SESSION_COUNT, got $RETRIEVED_COUNT_2"
        echo "$SESSIONS_RESPONSE_2" > "${OUTPUT_DIR}/leak_sessions_1.json"
    fi
    
    # Save full responses for detailed analysis
    echo "$UPLOAD_RESPONSE" > "${OUTPUT_DIR}/upload_response_1.json"
    echo "$SESSIONS_RESPONSE" > "${OUTPUT_DIR}/sessions_response_1.json"
    echo "$SESSIONS_RESPONSE_2" > "${OUTPUT_DIR}/sessions_response_2_1.json"
    
    echo "[Session 1] Complete"
}

# Function to perform uploads and tests for session 2
run_session_2() {
    # Wait a bit before starting to ensure first session is already in progress
    sleep $DELAY_SECONDS
    
    echo "[Session 2] Starting..."
    
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
    
    SESSION_COUNT=$(echo "$UPLOAD_RESPONSE" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    echo "[Session 2] Uploaded $SESSION_COUNT sessions"
    
    # Wait a bit
    sleep $((DELAY_SECONDS + 1))
    
    # Get sessions
    echo "[Session 2] Getting sessions"
    # Add explicit Cookie header to ensure it's being sent properly
    if [ -n "$SESSION_ID" ]; then
        echo "[Session 2] Using explicit session_id cookie: $SESSION_ID"
        SESSIONS_RESPONSE=$(curl -s -b "${OUTPUT_DIR}/cookies_2.txt" -H "Cookie: session_id=$SESSION_ID" -v "${SERVER_URL}/api/sessions" 2> "${OUTPUT_DIR}/sessions_curl_2.txt")
    else
        SESSIONS_RESPONSE=$(curl -s -b "${OUTPUT_DIR}/cookies_2.txt" -v "${SERVER_URL}/api/sessions" 2> "${OUTPUT_DIR}/sessions_curl_2.txt")
    fi
    
    # Save full response for analysis
    echo "$SESSIONS_RESPONSE" > "${OUTPUT_DIR}/sessions_response_2_full.json"
    
    # Check if response starts with an array bracket
    if [[ "$SESSIONS_RESPONSE" == \[* ]]; then
        # Count using simple grep for session objects - most reliable across systems
        RETRIEVED_COUNT=$(echo "$SESSIONS_RESPONSE" | grep -o '"avg_power"' | wc -l | tr -d ' ')
    else
        RETRIEVED_COUNT=0
    fi
    
    echo "[Session 2] Retrieved $RETRIEVED_COUNT sessions"
    
    # Display raw response for debugging
    echo "[Session 2] Raw sessions response (first 200 chars):"
    echo "$SESSIONS_RESPONSE" | head -c 200
    
    # Wait a bit more
    sleep $DELAY_SECONDS
    
    # Get sessions again to verify no contamination from session 1
    echo "[Session 2] Getting sessions again (checking for leaks)"
    SESSIONS_RESPONSE_2=$(curl -s -b "${OUTPUT_DIR}/cookies_2.txt" "${SERVER_URL}/api/sessions")
    
    # Save full response for analysis
    echo "$SESSIONS_RESPONSE_2" > "${OUTPUT_DIR}/sessions_response_2_leak_check.json"
    
    # Check if response starts with an array bracket
    if [[ "$SESSIONS_RESPONSE_2" == \[* ]]; then
        # Count using simple grep for session objects - most reliable across systems
        RETRIEVED_COUNT_2=$(echo "$SESSIONS_RESPONSE_2" | grep -o '"avg_power"' | wc -l | tr -d ' ')
    else
        RETRIEVED_COUNT_2=0
    fi
    
    echo "[Session 2] Retrieved $RETRIEVED_COUNT_2 sessions"
    
    # Verify counts match
    if [ "$SESSION_COUNT" -eq "$RETRIEVED_COUNT_2" ]; then
        echo "[Session 2] ✅ Test PASSED: Session data is consistent"
    else
        echo "[Session 2] ❌ Test FAILED: Session count mismatch! Expected $SESSION_COUNT, got $RETRIEVED_COUNT_2"
        echo "$SESSIONS_RESPONSE_2" > "${OUTPUT_DIR}/leak_sessions_2.json"
    fi
    
    # Save full responses for detailed analysis
    echo "$UPLOAD_RESPONSE" > "${OUTPUT_DIR}/upload_response_2.json"
    echo "$SESSIONS_RESPONSE" > "${OUTPUT_DIR}/sessions_response_1_2.json"
    echo "$SESSIONS_RESPONSE_2" > "${OUTPUT_DIR}/sessions_response_2_2.json"
    
    echo "[Session 2] Complete"
}

# Function to verify that sessions don't contaminate each other
verify_session_isolation() {
    echo "Verifying session isolation..."
    
    # Get the first session IDs from each session's responses
    # Using simple grep/cut to extract IDs from the first few sessions
    SESSION1_IDS=$(cat "${OUTPUT_DIR}/sessions_response_2_1.json" | grep -o '"id":"[^"]*"' | head -5 | sort)
    SESSION2_IDS=$(cat "${OUTPUT_DIR}/sessions_response_2_2.json" | grep -o '"id":"[^"]*"' | head -5 | sort)
    
    # Check if there's any overlap in session IDs
    OVERLAP=$(comm -12 <(echo "$SESSION1_IDS") <(echo "$SESSION2_IDS"))
    
    if [ -z "$OVERLAP" ]; then
        echo "✅ Session isolation test PASSED: No session data leakage detected"
    else
        echo "❌ Session isolation test FAILED: Found overlapping session IDs between sessions!"
        echo "Overlapping IDs:"
        echo "$OVERLAP"
    fi
    
    echo "Test output saved to $OUTPUT_DIR"
}

# Function for just uploading files in true parallel
upload_in_parallel() {
    echo "[Parallel Upload] Starting simultaneous uploads..."
    
    # Prepare upload commands for both sessions
    UPLOAD_CMD1="curl -s -X POST -F \"file=@$JSON_FILE1\" -F \"consent=false\" -c \"${OUTPUT_DIR}/cookies_1.txt\" -v \"${SERVER_URL}/api/upload\" 2> \"${OUTPUT_DIR}/curl_verbose_1.txt\" > \"${OUTPUT_DIR}/upload_response_1.txt\""
    UPLOAD_CMD2="curl -s -X POST -F \"file=@$JSON_FILE2\" -F \"consent=false\" -c \"${OUTPUT_DIR}/cookies_2.txt\" -v \"${SERVER_URL}/api/upload\" 2> \"${OUTPUT_DIR}/curl_verbose_2.txt\" > \"${OUTPUT_DIR}/upload_response_2.txt\""
    
    # Run both uploads with minimal delay between them
    eval "$UPLOAD_CMD1" &
    UPLOAD_PID1=$!
    sleep 0.1  # Very minimal delay to avoid exact same millisecond
    eval "$UPLOAD_CMD2" &
    UPLOAD_PID2=$!
    
    # Wait for uploads to complete
    wait $UPLOAD_PID1
    wait $UPLOAD_PID2
    
    echo "[Parallel Upload] Both uploads completed"
    
    # Display upload results
    echo "[Parallel Upload] Session 1 uploaded:"
    SESSION1_COUNT=$(cat "${OUTPUT_DIR}/upload_response_1.txt" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    echo "[Parallel Upload] Uploaded $SESSION1_COUNT sessions for Session 1"
    
    echo "[Parallel Upload] Session 2 uploaded:"
    SESSION2_COUNT=$(cat "${OUTPUT_DIR}/upload_response_2.txt" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    echo "[Parallel Upload] Uploaded $SESSION2_COUNT sessions for Session 2"
    
    # Extract session cookies
    SESSION_ID1=$(grep -A 10 "Set-Cookie:" "${OUTPUT_DIR}/curl_verbose_1.txt" | grep "session_id" | head -1 | sed -n 's/.*session_id=\([^;]*\).*/\1/p')
    SESSION_ID2=$(grep -A 10 "Set-Cookie:" "${OUTPUT_DIR}/curl_verbose_2.txt" | grep "session_id" | head -1 | sed -n 's/.*session_id=\([^;]*\).*/\1/p')
    
    echo "[Parallel Upload] Session 1 ID: $SESSION_ID1"
    echo "[Parallel Upload] Session 2 ID: $SESSION_ID2"
    
    # Check sessions immediately after parallel upload
    check_sessions_after_parallel_upload
}

# Function to check sessions immediately after parallel upload
check_sessions_after_parallel_upload() {
    echo "[Parallel Test] Checking sessions immediately after parallel upload"
    
    # Get sessions for first upload
    SESSION_ID1=$(grep -A 10 "Set-Cookie:" "${OUTPUT_DIR}/curl_verbose_1.txt" | grep "session_id" | head -1 | sed -n 's/.*session_id=\([^;]*\).*/\1/p')
    SESSIONS_RESPONSE1=$(curl -s -b "${OUTPUT_DIR}/cookies_1.txt" -H "Cookie: session_id=$SESSION_ID1" "${SERVER_URL}/api/sessions")
    echo "$SESSIONS_RESPONSE1" > "${OUTPUT_DIR}/sessions_immediate_1.json"
    RETRIEVED_COUNT1=$(echo "$SESSIONS_RESPONSE1" | grep -o '"avg_power"' | wc -l | tr -d ' ')
    
    # Get sessions for second upload
    SESSION_ID2=$(grep -A 10 "Set-Cookie:" "${OUTPUT_DIR}/curl_verbose_2.txt" | grep "session_id" | head -1 | sed -n 's/.*session_id=\([^;]*\).*/\1/p')
    SESSIONS_RESPONSE2=$(curl -s -b "${OUTPUT_DIR}/cookies_2.txt" -H "Cookie: session_id=$SESSION_ID2" "${SERVER_URL}/api/sessions")
    echo "$SESSIONS_RESPONSE2" > "${OUTPUT_DIR}/sessions_immediate_2.json"
    RETRIEVED_COUNT2=$(echo "$SESSIONS_RESPONSE2" | grep -o '"avg_power"' | wc -l | tr -d ' ')
    
    # Compare to upload counts
    SESSION1_COUNT=$(cat "${OUTPUT_DIR}/upload_response_1.txt" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    SESSION2_COUNT=$(cat "${OUTPUT_DIR}/upload_response_2.txt" | grep -o '"count":[0-9]*' | cut -d':' -f2)
    
    echo "[Parallel Test] Session 1: Expected $SESSION1_COUNT, Retrieved $RETRIEVED_COUNT1"
    echo "[Parallel Test] Session 2: Expected $SESSION2_COUNT, Retrieved $RETRIEVED_COUNT2"
    
    # Check for any data leakage
    if [ "$SESSION1_COUNT" -eq "$RETRIEVED_COUNT1" ]; then
        echo "[Parallel Test] ✅ Session 1 data consistent after parallel upload"
    else
        echo "[Parallel Test] ❌ Session 1 data inconsistent after parallel upload! Expected $SESSION1_COUNT, got $RETRIEVED_COUNT1"
    fi
    
    if [ "$SESSION2_COUNT" -eq "$RETRIEVED_COUNT2" ]; then
        echo "[Parallel Test] ✅ Session 2 data consistent after parallel upload"
    else
        echo "[Parallel Test] ❌ Session 2 data inconsistent after parallel upload! Expected $SESSION2_COUNT, got $RETRIEVED_COUNT2"
    fi
}

# Run the full isolation test with true parallel uploads first
upload_in_parallel

# Then run the full test with sessions as before
run_session_1 &
SESSION1_PID=$!

run_session_2 &
SESSION2_PID=$!

# Wait for both processes to finish
wait $SESSION1_PID
wait $SESSION2_PID

# Verify isolation between sessions
verify_session_isolation

echo "Test complete. Results saved in $OUTPUT_DIR"
