# BMW CarData Battery Health Tracking

This extension to the BMW CarData Tools adds persistent storage and fleet-wide battery health tracking, allowing you to:

1. Store all uploaded charging data in SQLite for persistence between service restarts
2. Track battery degradation over time across the fleet, similar to TeslaLogger
3. Filter data by different BMW models
4. Prevent duplicate uploads with content hashing
5. Preserve privacy through FIN (vehicle ID) hashing

## New Features

### Database Storage

All charging data is now stored in an SQLite database located in the `./data` directory, making it persistent between application restarts. The original JSON files are never stored - only the processed, anonymized data is saved.

### Battery Health Tracking

The system now tracks battery health (estimated capacity) over time, calculated from charging sessions with significant SOC changes. This gives you insights into battery degradation patterns across your fleet.

### Model Filtering

You can now filter battery health data by different BMW models, allowing you to compare degradation patterns across different vehicle types.

### Privacy Protection

Vehicle identification numbers (FINs) are securely hashed before storage to maintain privacy while still allowing tracking of individual vehicles over time. No personal data or location information is stored in a way that could identify users.

## Using the Battery Health Dashboard

1. Navigate to `/battery` in your browser
2. View fleet-wide battery health trends
3. Use the model filter to focus on specific BMW models
4. Analyze both individual data points and the monthly trend line

## Technical Implementation

- SQLite database for persistent storage
- Content hashing to prevent duplicate uploads
- One-way hashing for vehicle identifiers
- Responsive visualization using Plotly.js

## Database Schema

The system uses the following tables:

- `uploads`: Tracks uploaded files with content hashes to prevent duplicates
- `vehicles`: Stores hashed vehicle identifiers and models
- `sessions`: Stores anonymized charging session data
- `battery_health`: Tracks battery capacity estimates over time

## Building with Database Support

***REMOVED***
# Install SQLite dependencies and build
make setup-db
make build

# Or do everything at once
make all
***REMOVED***

## Docker Deployment

The Docker configuration includes a persistent volume for the database:

***REMOVED***
docker-compose up -d
***REMOVED***

This will mount a `./data` directory to store the database file.
