# BMW CarData - Charging Session Dashboard (Go Version)

A Go-based reimplementation of the BMW CarData dashboard for improved performance. This application allows BMW electric vehicle owners to analyze their charging data exported from the My BMW app.

## Features

- Upload and analyze BMW CarData JSON files
- Interactive dashboard with visualizations of charging data
- Analyze charging efficiency and power consumption
- View charging locations on a map
- Track battery State of Health (SoH) over time
- Compare charging sessions and providers
- Toggle between kilometers and miles

### Charge point providers

- BMW can list many possible charge points for one session. The app counts the provider names of all matches and uses the most frequent name.
- If two providers have the same count, the app uses the provider that wins most often at the same place in the same upload. If this does not decide, the app uses the first match.
- The app removes the charge point type ("HPC", "DC", "AC"), legal forms ("GmbH", "AG", "B.V." and more) and extra spaces. Known names of one operator become one name (for example "HPC/DC IONITY GmbH" becomes "IONITY").
- Provider lists show only operator names. Sessions without a known operator do not show in provider lists. They stay in the session totals.
- A session with no energy and no SoC gain is a failed start. It counts as failed for the provider. Efficiency, power and energy averages use successful sessions only.
- Stored fleet data is cleaned when it is read. No database change is necessary.

## Architecture

This application consists of:

1. **Backend**: Go-based HTTP server
   - JSON parsing and data processing
   - RESTful API endpoints
   - High-performance concurrency

2. **Frontend**: HTML/CSS/JavaScript
   - Interactive dashboard using Plotly.js for visualizations
   - Leaflet.js for map functionality
   - Responsive design

3. **Deployment**: Traefik as reverse proxy and SSL termination
   - Automatic HTTPS certificate management via Let's Encrypt
   - Docker integration with service discovery
   - HTTP to HTTPS redirection

## Installation

### Using Pre-built Docker Image (recommended)

The application is automatically built and published to GitHub Container Registry:

```bash
# Pull the latest version
docker pull ghcr.io/awlx/bmwtools:latest

# Create a docker-compose.yml file (see DOCKER.md for complete example)
# Run the application
docker-compose up -d
```

For more details on using the pre-built Docker image, see [DOCKER.md](DOCKER.md).

### Building Docker Image Locally

```bash
# Clone the repository
git clone https://github.com/awlx/bmwtools.git
cd bmwtools/go-rewrite

# Prepare Traefik configuration
chmod 600 traefik/acme.json

# Build and start the application with Docker Compose
docker-compose up -d
```

The application will be available at https://bmwtools.localhost (or your configured domain)

#### Customizing the Domain

Edit the `docker-compose.yaml` file and update the domain in the Traefik labels:

```yaml
labels:
  - "traefik.http.routers.bmwtools-server.rule=Host(`your-domain.com`)"
```

Also update the email address in `traefik/traefik.yml` for Let's Encrypt notifications.

### Manual Installation

```bash
# Clone the repository
git clone https://github.com/awlx/bmwtools.git
cd bmwtools/go-rewrite

# Build the Go application
go build -o bmwtools-server ./cmd/server/main.go

# Run the server
./bmwtools-server
```

The application will be available at http://localhost:8050

## Usage

1. Export your charging history from the My BMW app
2. Visit the dashboard in your web browser
3. Upload your BMW-CarData-Ladehistorie_*.json file
4. Explore your charging data through the interactive dashboard
5. Optional: share your data with the fleet statistics. You can do this at upload, or later with the "Share my data with the fleet" panel. Demo data cannot be shared.

## API Endpoints

- `POST /api/upload` - Upload JSON file
- `GET /api/contribute` - Tell if the loaded data can be shared with the fleet
- `POST /api/contribute` - Share the loaded data with the fleet (`consent=true`, `model`). Data that is already stored is not stored again.
- `GET /api/demo` - Load demo data
- `GET /api/sessions` - Get all charging sessions
- `GET /api/session/:id` - Get details for a specific session
- `GET /api/stats` - Get statistical data
- `GET /api/map` - Get map data

## Why Go?

The Go implementation offers several advantages over the original Python version:

- **Performance**: Significantly faster JSON parsing and data processing
- **Concurrency**: Efficient handling of multiple concurrent users
- **Memory Efficiency**: Lower memory footprint
- **Deployment**: Single binary deployment
- **Security**: Strong type system and memory safety

## Configuration

| Variable | Default | Use |
|---|---|---|
| `PORT` | `8050` | HTTP port. |
| `BMWTOOLS_VIN_SALT` | built-in value | Salt for the one-way VIN hash. Set your own secret for a new database. Do not change it for an existing database: the hashes then do not match and re-uploads create new vehicles. |

## Fleet data identity

- The server never stores the VIN. It stores a salted hash of the VIN from the upload filename.
- If the filename has no VIN, the server uses a hash of the file content.
- Before the server creates a vehicle, it compares the upload with stored sessions (start time, SoC start, SoC end, energy). If at least 30% or 20 sessions match one vehicle, the upload joins that vehicle. The server keeps the new hash as an alias.
- Two vehicles with different VIN hashes are never joined.
- The server skips sessions that are already stored, by id or by the same values.
- A later upload can make the model more specific (for example `BMW i4` to an i4 battery group). A change to a different model needs a battery capacity that fits that model.
- The server ignores records without a start time. A file with no valid CarData sessions gives an error.

## Database Migration

If you're upgrading from a previous version, you may need to migrate your database schema.
See [MIGRATION.md](MIGRATION.md) for instructions on updating your database.

### Fleet repair (one-off)

The repair command fixes old fleet data:

- It merges duplicate vehicles (same sessions) and removes duplicate sessions and battery points.
- It deletes records with a start time of 1970.
- It fixes 1970 end times from the session duration, or clears them.
- It splits a vehicle that holds two cars (odometer drop of more than 1,000 km). Vehicles with a VIN hash are not split.

```bash
# Dry run on a temporary copy. The database does not change.
go run ./migrations/repair_fleet -db ./data/bmwtools.db
# Repair. The command writes a backup next to the database first.
go run ./migrations/repair_fleet -db ./data/bmwtools.db -apply
```

Stop the server before you use `-apply`.

## Disclaimer

This application stores all uploaded data in memory. If you refresh, your session is lost.
CarData contains location data of your charges. Use at your own risk!
