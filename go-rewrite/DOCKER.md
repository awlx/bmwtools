# Using the BMW Tools Docker Image

This project is automatically built and published to the GitHub Container Registry (ghcr.io) using GitHub Actions.

## How to Use the Docker Image

### Quick Start - Running Locally

For quick local testing without Docker Compose, you can run the image directly:

```bash
docker run -p 8050:8050 ghcr.io/awlx/bmwtools:latest
```

Then access the dashboard by visiting http://127.0.0.1:8050 in your browser.

### Production Setup with Docker Compose

For a more robust setup with Traefik for HTTPS and domain support, use Docker Compose:

```yaml
version: '3.8'

services:
  bmwtools-server:
    image: ghcr.io/awlx/bmwtools:latest
    container_name: bmwtools-server
    restart: always
    environment:
      - PORT=8050
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.bmwtools-server.rule=Host(`bmwtools.yourdomain.com`)"
      - "traefik.http.routers.bmwtools-server.entrypoints=websecure"
      - "traefik.http.routers.bmwtools-server.tls.certresolver=myresolver"
      - "traefik.http.services.bmwtools-server.loadbalancer.server.port=8050"
    networks:
      - webnet

  # Include your traefik configuration as shown in the main docker-compose.yaml
  
networks:
  webnet:
```

## Available Tags

The following tags are available for the Docker image:

- `latest`: The latest build from the main branch
- `vX.Y.Z`: Specific version (e.g., v1.0.0)
- `vX.Y`: Major.Minor version (e.g., v1.0)
- `sha-XXXXXXX`: Specific commit SHA (short format)

## Authentication

To pull the image from GitHub Container Registry, you may need to authenticate:

```bash
# Login to GitHub Container Registry
echo $CR_PAT | docker login ghcr.io -u USERNAME --password-stdin
```

Replace `$CR_PAT` with your GitHub Personal Access Token and `USERNAME` with your GitHub username.
