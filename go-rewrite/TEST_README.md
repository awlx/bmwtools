# Session Isolation Test Scripts

These scripts are designed to test the session isolation capabilities of the BMW Tools application to ensure that user data doesn't leak between sessions. The scripts include extensive debugging capabilities to help diagnose any issues with cookie handling or session management.

## Basic Session Isolation Test

The `test_session_isolation.sh` script performs a simple test by uploading two different JSON files in parallel and verifying that each session only has access to its own data.

### Usage:

```bash
./test_session_isolation.sh <json_file1> <json_file2>
```

Example:
```bash
./test_session_isolation.sh BMW-CarData-Ladehistorie_*.json BMW-CarData-Ladehistorie_*.json
```

By default, the script connects to a server running at `http://localhost:8080`. You can change this by setting the `SERVER_URL` environment variable:

```bash
SERVER_URL=http://your-server-url:8080 ./test_session_isolation.sh file1.json file2.json
```

## Comprehensive Session Isolation Test

The `test_comprehensive_isolation.sh` script performs a more thorough test by checking isolation across multiple API endpoints:
- `/api/sessions` - Verifies session data isolation
- `/api/stats` - Verifies statistics data isolation
- `/api/map` - Verifies map data isolation
- `/api/providers` - Verifies provider data isolation

### Usage:

```bash
./test_comprehensive_isolation.sh <json_file1> <json_file2>
```

Example:
```bash
./test_comprehensive_isolation.sh BMW-CarData-Ladehistorie_1.json BMW-CarData-Ladehistorie_2.json
```

## Test Results

Both scripts create a temporary directory to store test results. The path to this directory is displayed at the end of the test run. You can inspect the JSON responses from each endpoint to verify that data isolation is working correctly.

### Debugging Information

The test scripts generate extensive debugging information to help diagnose any issues with cookie handling or session management:

- Verbose curl output with request/response headers
- Cookie file contents
- Raw session IDs extracted from cookies
- Raw API responses for inspection
- Detailed error messages when tests fail

All this information is saved in the temporary output directory, which is displayed at the end of the test run.

## Testing With Delays

The scripts include deliberate delays between requests to ensure that concurrent sessions are properly isolated even under load. This simulates real-world scenarios where multiple users might be interacting with the system simultaneously.

## Troubleshooting

If the tests fail with "Session count mismatch" errors, check the following:

1. **Cookie Handling**: Make sure the server is setting the `session_id` cookie properly. Inspect the verbose curl output in the debug files.

2. **Server Configuration**: Verify that the server is running on the expected port (default: 8080). You can change this using the `SERVER_URL` environment variable.

3. **Session Timeout**: If sessions are expiring too quickly, it could cause tests to fail. Check the session expiration time in the server code.

4. **Raw Responses**: Examine the raw API responses saved in the output directory to see what data the server is actually returning.

5. **Server Logs**: Check the server logs for any errors or warnings related to session handling.