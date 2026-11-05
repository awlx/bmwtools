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

### Model and Battery Selection

At upload, select the model and the battery size. Trims with the same battery are one option, e.g. "BMW i4 eDrive40 / xDrive40 / M50 / M60 (80.7 kWh)". The list comes from `pkg/vehicles/catalog.go` and from `GET /api/vehicle-models`. The server rejects unknown values. Old values (e.g. "BMW i4") stay valid.

### Battery Groups

One model can have different battery sizes. The page puts each car into one battery group:

1. **Selected:** the owner selected the battery at upload.
2. **Matched:** old uploads without a battery. The page groups these cars by start capacity (a gap of 7 % or more starts a new group). Then it matches each group to the nearest known battery (maximum 7 % difference).
3. **Auto:** no known battery is within 7 %, but one is within 15 %. The label is "~X kWh pack (auto)".

**Plausibility check:** if the start capacity of a car is more than 15 % from every known battery of its model, the page excludes the car (e.g. a wrong model selection). The page shows the number of excluded cars. Models without known batteries have no check.

**Short charges:** the analysis ignores measurements with a SoC change below 40 %. These measurements have a large error and read approx. 4.5 % too high. The database keeps them.

**Start capacity:** the median of the first measurements of a car (first 3,000 km, minimum 5 points). A car needs 3 or more measurements. If the odometer goes back by more than 1,000 km, the page treats the data as a different car.

### Degradation Charts

- One chart for each battery group.
- Each car starts at 100 % of its own start capacity. The x-axis shows the km or months since the first data of that car. Cars that join at high mileage thus show their real loss.
- **Line:** median of the cars in each step (5,000 km or 6 months). Each car counts once per step. The line starts at 100 % at 0. The measurements that make the start capacity are not in the line, because they are near 100 % by definition.
- **Minimum cars:** a step needs 5 or more cars for the line. Steps with 2 to 4 cars show as faded dots. The hover shows the number of cars.
- **Seasons:** charging losses change with temperature. Estimated values are approx. 1.5 % higher in summer than in winter. A 6-month step holds warm and cold months, which makes this effect smaller.
- **Band:** the middle 50 % of the cars.
- **Title figure:** the median slope of the cars, e.g. "≈ −0.7 % per 10,000 km". The page fits one straight line per car (% of start capacity). Only cars with 10 or more measurements over 20,000 km or more count. The value per year also needs 12 months of data per car. The page shows the value only when 5 or more cars count. Otherwise it shows "Not enough data".
- **Left axis:** kWh for the median start capacity of the group. **Right axis:** % of start capacity.
- Single measurements are hidden. Select "Show single measurements" to see them.
- Groups with too few cars for a line are listed below the charts.

### Privacy Protection

Vehicle identification numbers (FINs) are securely hashed before storage to maintain privacy while still allowing tracking of individual vehicles over time. No personal data or location information is stored in a way that could identify users.

## Using the Battery Health Dashboard

1. Go to `/battery`.
2. Select the model. The page opens with the model that has the most cars.
3. Optional: select one battery group, or the x-axis (km or months).
4. Read the loss rate in the chart title, e.g. "≈ −0.7 % per 10,000 km".

The API (`GET /api/battery-health?model=...`) does not return vehicle IDs or hashes.

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
- `battery_health`: Tracks battery capacity estimates over time with mileage information

**Note:** If you're upgrading from a previous version, you may need to run the database migration script to add the mileage column. See [MIGRATION.md](MIGRATION.md) for details.

## Building with Database Support

```bash
# Install SQLite dependencies and build
make setup-db
make build

# Or do everything at once
make all
```

## Docker Deployment

The Docker configuration includes a persistent volume for the database:

```bash
docker-compose up -d
```

This will mount a `./data` directory to store the database file.
