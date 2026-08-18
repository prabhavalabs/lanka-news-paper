# Production deployment

Merges to `main` run tests, build immutable Linux/amd64 images in GitHub Actions, push them to GHCR, run database migrations, and health-check the full stack on the VPS.

## Runtime layout

- `/opt/lanka-news-paper/.env`: production secrets, stored only on the VPS.
- `lanka-news-paper_media-data`: local upload fallback volume.
- `lanka-news-paper_postgres-data`: PostgreSQL data volume.
- `127.0.0.1:18173`: public web container.
- `127.0.0.1:18174`: admin container.
- `127.0.0.1:18090`: API health/debug port.

Every application container has a CPU and memory ceiling so this stack cannot take over the shared VPS. Application images are built in GitHub, never on the VPS.

## GitHub environment

The `production` environment needs these secrets:

- `PRODUCTION_HOST`
- `PRODUCTION_USER`
- `PRODUCTION_SSH_PRIVATE_KEY`
- `PRODUCTION_KNOWN_HOSTS`

Application and database secrets are deliberately not copied through GitHub Actions. They are provisioned once in `/opt/lanka-news-paper/.env` with mode `0600`.

## DNS and TLS

Create proxied or DNS-only `A` records pointing to `178.104.111.17` for:

- `lankanewspaper.prabhavalabs.com`
- `admin.lankanewspaper.prabhavalabs.com`

After DNS resolves, install `infra/production/nginx-site.conf` and issue one certificate containing both names:

```sh
certbot certonly --webroot -w /var/www/lanka-news-paper-acme \
  --cert-name lankanewspaper.prabhavalabs.com \
  -d lankanewspaper.prabhavalabs.com \
  -d admin.lankanewspaper.prabhavalabs.com
nginx -t && systemctl reload nginx
```

## Operations

```sh
cd /opt/lanka-news-paper
docker compose ps
curl --fail http://127.0.0.1:18090/api/v1/health/ready
docker compose logs --tail=100 api worker
```

The deployed commit is recorded in `.deployed-image`. To redeploy a retained image manually, run `./deploy.sh <commit-sha>`.
