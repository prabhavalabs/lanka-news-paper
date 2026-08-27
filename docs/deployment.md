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

Keep `SNAP_BOOTSTRAP_ADMIN_PASSWORD_HASH` inside single quotes in that file.
Bcrypt hashes contain `$`, which Docker Compose otherwise treats as environment
variable interpolation.

`OPENROUTER_API_KEY` must be present in that file. Both API and worker containers
receive it; the database stores only the environment-variable reference.

### Morning newsletter

The administrator mailing list is stored in PostgreSQL and managed from the
admin application's **Mailing list** page. Configure delivery in
`/opt/lanka-news-paper/.env`:

The complete selection, writing, safety, and retry policy is documented in
[`docs/newsletter.md`](newsletter.md).

```dotenv
RESEND_API_KEY=<sending-only Resend API key>
SNAP_NEWSLETTER_ENABLED=true
SNAP_NEWSLETTER_BASE_URL=https://lankanewspaper.prabhavalabs.com
SNAP_NEWSLETTER_FROM="Morning Brief <brief@prabhavalabs.com>"
SNAP_NEWSLETTER_RECIPIENT=<optional initial address, imported only once>
SNAP_NEWSLETTER_TIMEZONE=Asia/Colombo
SNAP_NEWSLETTER_SEND_HOUR=8
```

Only recipients with `active` status are sent the prior 24-hour brief. The
worker records one edition per local calendar day and one idempotent delivery
per recipient, so restarts cannot duplicate an already-sent edition. Every
message includes web and one-click unsubscribe links.

Resend's sending domain must remain verified. Publish SPF and DKIM exactly as
shown by Resend, and publish this TXT record at `_dmarc.prabhavalabs.com`:

```text
v=DMARC1; p=none; rua=mailto:marc-reports@prabhavalabs.com; adkim=r; aspf=r; pct=100
```

The DNS value must contain a literal `@`, without a backslash. Ensure
`marc-reports@prabhavalabs.com` is a real mailbox or forwarding alias so the
aggregate reports are received. Begin with `p=none`; move to quarantine or
reject only after the reports show that all legitimate senders align.

## Codex CLI administrative backfills

The API image contains a pinned Codex CLI, but Codex is not used by regular
workflows. Its authentication is stored in a dedicated Docker volume shared by
the API readiness check and the isolated administrative-analysis worker.

After the first deployment (and whenever authentication expires), authenticate
the volume interactively on the VPS:

```sh
cd /opt/lanka-news-paper
docker compose --profile tools run --rm codex-login
```

Follow the device-login URL shown by the command, then confirm the Settings page
reports Codex CLI as ready. The application runs Codex in an empty temporary
workspace, disables interactive tools, strips database and provider secrets from
the child environment, enforces structured output, and processes only one
administrative article at a time. `CODEX_BACKFILL_MODELS` in `.env` can restrict
the model choices exposed to administrators.

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
