# Deploying Family Hub

One server runs everything with Docker Compose: Caddy (HTTPS), the app and Postgres (architecture §14). These steps are for AWS Lightsail; any Ubuntu server works the same from step 3.

## First deploy

1. **Create the server** in Lightsail: Linux/Unix, OS only, **Ubuntu 24.04 LTS**, the **$12 plan with IPv4**, in the US region closest to most of the family.
   - Networking: attach a **static IP**, and add a firewall rule for **HTTPS (TCP 443)** and one for **UDP 443**. SSH (22) and HTTP (80) are open by default; Caddy needs 80 to get its certificate.
   - Turn on **automatic snapshots**.
2. **Point the domain at it:** an `A` record for the site's name (e.g. `family.example.com`) to the static IP.
3. **Install Docker** on the server, from Docker's own apt repository (docs.docker.com/engine/install/ubuntu), then `sudo usermod -aG docker ubuntu` and sign in again. Ubuntu installs security updates by itself (`unattended-upgrades`).
4. **Let the server pull the app.** The image is private on GitHub's registry. Make a GitHub token (classic) with only the `read:packages` scope, then on the server:

   ```sh
   docker login ghcr.io -u <github-username>   # paste the token as the password
   ```

5. **Copy the files** from this folder to `~/family-hub` on the server: `compose.yaml`, `Caddyfile`, and the two templates, saved as `.env` and `app.env`:

   ```sh
   cp env.example .env && cp app.env.example app.env && chmod 600 .env app.env
   ```

   Fill them in. `APP_VERSION` is the latest release tag on GitHub; make `POSTGRES_PASSWORD` with `openssl rand -hex 24`. Keep a copy of both files somewhere safe off the server, like a password manager.
6. **Start it**, then create your admin login (it asks for the password):

   ```sh
   docker compose up -d --wait   # returns once the app is healthy, after its migrations
   docker compose exec app ./family-hub admin create --email <you@example.com>
   ```

   Open `https://<your domain>` and sign in. If the page doesn't load, `docker compose logs caddy` shows certificate problems, usually DNS not pointing at the server yet.

## Updating to a new release

Set `APP_VERSION` in `.env` to the new tag, then:

```sh
docker compose pull app && docker compose up -d --wait
```

The app runs any new database migrations as it starts.

## Checking on it

- `docker compose ps`: all three should be up, the app and Postgres `(healthy)`.
- `curl -s 127.0.0.1:8080/ready`: answers only on the server; from the internet `/health` and `/ready` are a 404.
- `docker compose logs -f app`: one JSON line per request.
