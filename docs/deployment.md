# Production deployment

Merges to `main` run tests, build immutable Linux/amd64 images in GitHub Actions, push them to GHCR, run database migrations, and health-check the full stack on the VPS.

## Runtime layout

- `/opt/lanka-news-paper/.env`: production secrets, stored only on the VPS.
- `lanka-news-paper_media-data`: local upload fallback volume.
- `lanka-news-paper_postgres-data`: PostgreSQL data volume.
- `127.0.0.1:15432`: PostgreSQL, reachable only through an authenticated SSH tunnel.
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

## Shared development data

Until the application has end users, a local API may use the deployed database and R2 bucket as a shared test environment. PostgreSQL remains private on the VPS; open a tunnel from a separate terminal:

```sh
ssh -N -o ExitOnForwardFailure=yes \
  -L 127.0.0.1:55433:127.0.0.1:15432 \
  <production-user>@178.104.111.17
```

Copy `infra/local/shared.env.example` to the repository-root `.env`, replace its placeholders with the production database password and bucket-scoped R2 credentials, then run the API and either frontend normally. The production worker remains the only ingestion scheduler.

`SNAP_ENV=shared-development` blocks `pnpm worker:dev` and `pnpm migrate`. Tests and schema migrations must continue to use an isolated local database. Admin actions from the local API still modify shared data intentionally.
