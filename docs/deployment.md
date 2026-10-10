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

For migration, delete `OPENROUTER_API_KEY` from `/opt/lanka-news-paper/.env`
and revoke the retired key in the OpenRouter dashboard. Remove the obsolete
`CODEX_BACKFILL_MODELS` and `SNAP_ADMIN_ANALYSIS_PROVIDER_SORT` settings too.

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

## Codex CLI authentication and capacity

Codex CLI is the only LLM provider. The pinned API image contains the CLI;
API (Watch Tower) and worker both mount `codex-data` at `CODEX_HOME=/data/codex`.
Treat `auth.json` and volume backups as secrets: never log, commit, or copy them
into images. Use one owner-controlled ChatGPT account on this trusted private server.

After first deployment, authentication expiry, or account rotation, run on the VPS:

```sh
cd /opt/lanka-news-paper
docker compose --profile tools run --rm codex-login
```

Follow the device-login URL and authenticate as the intended owner. For rotation,
first revoke the old account session using its account security controls, then
stop both API and worker, repeat this login with the replacement owner account,
then restart both processes to discard cached login confirmation. The CLI manages credential
refresh; expired or revoked credentials require this interactive login again.
Never print the credential file to diagnose readiness.

Check **AI routing** for provider status, version, authentication method, and
cooldown, or **Settings** for CLI readiness. Authenticated administrators can
also request `GET /api/admin/llm/providers` and
`GET /api/admin/settings/codex` using their existing authenticated session.
The latter is a login probe; provider status also includes quota/auth cooldown.
A successful login probe cannot establish that quota has recovered.

```dotenv
SNAP_CODEX_CONCURRENCY=2
SNAP_CODEX_QUOTA_COOLDOWN=15m
SNAP_CODEX_MODELS=gpt-6-luna,gpt-6.1-sol,gpt-6-sol,gpt-5.6-luna,gpt-5.6-terra
```

Concurrency limits CLI executions per process (API and worker have independent
limits). Auth or quota failures pause pipeline dispatch during the cooldown;
blocked retries do not renew the cooldown. After it expires, a new request can
retry; this does not guarantee restored quota. Reauthenticate on `auth_expired`.
Configure models actually available to the owner account; the catalog is a
configured allowlist, not an account entitlement check.

The worker retains its 2 GB ceiling. API memory is 1 GB to accommodate the CLI
processes alongside the Go service and request buffers. These ceilings are
configuration, not measured capacity; measure peak usage before increasing concurrency.

### Shared credential-volume refresh risk

API and worker currently share a writable credential volume, and each can run
multiple independent CLI processes. In pinned CLI 0.162.1, the refresh semaphore
is local to an AuthManager; file storage truncates and rewrites `auth.json`
without a cross-process lock. Concurrent refreshes can race or expose a partial
write. This remains a runtime risk; the application semaphore does not coordinate
containers. See the pinned upstream [refresh implementation](https://github.com/openai/codex/blob/rust-v0.162.1/codex-rs/login/src/auth/manager.rs)
and [file storage](https://github.com/openai/codex/blob/rust-v0.162.1/codex-rs/login/src/auth/storage.rs).

Stop API and worker before interactive login or account rotation. Setting
`SNAP_CODEX_CONCURRENCY=1` reduces refresh contention within each process but
does not serialize API against worker. A complete fix needs either one
credential-owning execution service or an advisory lock around every CLI
invocation sharing the volume, including login tools; the latter also serializes
inference. Do not mount a worker-owned home read-only in API and assume refresh
will work: API-side refresh would fail to persist its credentials.

The local worker compose file also mounts the host's entire `~/.codex` home.
Host CLI sessions can refresh the same account file concurrently; use a separately
logged-in, dedicated CLI home when isolating development from personal sessions.

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

## Facebook Page publishing

See [Facebook Page publishing](social-publishing.md) for the new Meta Business app setup, review workflow, caps and failure recovery. These optional server variables are forwarded to API and worker:

```dotenv
SNAP_META_APP_ID=
SNAP_META_APP_SECRET=
SNAP_META_REDIRECT_URL=https://admin.lankanewspaper.prabhavalabs.com/api/admin/social/facebook/callback
SNAP_META_LOGIN_CONFIG_ID=
SNAP_META_GRAPH_VERSION=v26.0
SNAP_SOCIAL_TOKEN_KEY=
SNAP_SOCIAL_PUBLIC_BASE_URL=https://lankanewspaper.prabhavalabs.com
SNAP_SOCIAL_CHROME_PATH=
```

Generate `SNAP_SOCIAL_TOKEN_KEY` once as base64 encoded 32 bytes and preserve it with deployment secrets. Both processes must have the same key and use shared media storage (the Compose worker now also mounts `media-data`). Chrome/Chromium and Sinhala fonts are included by the image renderer's Docker configuration. If renderer initialization fails, queue/review still work and publishing waits for a rendered image. No app configuration automatically enables selection or auto-publish; both default to off in migration 54.

The existing admin-origin `/api/` proxy carries the OAuth callback and session cookie. Manual post creation gets the existing long-inference nginx timeout, because it invokes the editorial model. After deployment, verify the Page connection and a reviewed post through the actual Meta app; Development-mode API success does not by itself prove public post visibility.
