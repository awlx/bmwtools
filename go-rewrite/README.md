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
docker pull ghcr.io/awlx/bmwtools/bmwtools:latest

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

## API Endpoints

- `POST /api/upload` - Upload JSON file
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

## Disclaimer

This application stores all uploaded data in memory. If you refresh, your session is lost.
CarData contains location data of your charges. Use at your own risk!
