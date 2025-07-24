# BMW Tools with Traefik

This project now uses Traefik as a reverse proxy instead of Nginx + Certbot.

## Setup Instructions

1. Make sure the `acme.json` file has the proper permissions:
   ***REMOVED***
   chmod 600 traefik/acme.json
   ***REMOVED***

2. Update the domain name in `docker-compose.yaml` if needed:
   - The default domain is set to `bmwtools.localhost`
   - Change the `traefik.http.routers.bmwtools-server.rule=Host(...)` label if you want to use a different domain

3. Update the email address in `traefik/traefik.yml`:
   - Look for `email: mail@localhost` in the ACME configuration
   - Change it to your email address for Let's Encrypt notifications

## Starting the Application

***REMOVED***
docker-compose up -d
***REMOVED***

## How It Works

- Traefik automatically detects the BMW Tools service through Docker labels
- HTTPS certificates are automatically obtained from Let's Encrypt
- All HTTP traffic is redirected to HTTPS
- Certificates are automatically renewed when needed

## Accessing the Application

Once the containers are running, you can access the application at:
- https://bmwtools.localhost(or your configured domain)
