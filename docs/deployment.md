# Production deployment

Merges to `main` run tests, build immutable Linux/amd64 images in GitHub Actions, push them to GHCR, run database migrations, and health-check the full stack on the VPS.

The pipeline targets about two minutes. Go tests run in parallel with the release build. The build job compiles the Go binaries and both frontends on the runner (warm Go and pnpm caches) into `.ci-out/`, and `docker buildx bake` (`infra/production/docker-bake.hcl`, `package.Dockerfile`) packages the three images in parallel, so only small copy layers change per release. Deploy uploads `compose.yaml` and `deploy.sh` and runs the release over one SSH connection; health checks poll every second while containers start. `api.Dockerfile` and `frontend.Dockerfile` remain for self-contained builds from source.

**CI/CD does deployment only.** Server configuration (nginx sites, TLS certificates, `/opt/lanka-news-paper/.env`) is applied by hand over SSH before a release that needs it, and is never part of the workflow.

## Runtime layout

- `/opt/lanka-news-paper/.env`: production secrets, stored only on the VPS.
- `lanka-news-paper_media-data`: local upload fallback volume.
- PostgreSQL: the `lanka_news` database on the host's shared server (`platform-postgres` on the external `platform` Docker network, `127.0.0.1:5432` on the host), backed up nightly with every other app. Reachable only through an authenticated SSH tunnel.
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

### thulawanews.com (current)

| Host | Serves | Upstream |
| --- | --- | --- |
| `thulawanews.com` | Reader site | `127.0.0.1:18173` (web container) |
| `www.thulawanews.com` | Redirects to `thulawanews.com` | |
| `admin.thulawanews.com` | Admin portal and its same-origin `/api/` | `127.0.0.1:18174` (admin container) |
| `api.thulawanews.com` | Public read API; `/api/admin/` returns 404 | `127.0.0.1:18090` (API) |

DNS is managed in Cloudflare. Create `A` records to `178.104.111.17` and `AAAA` records to `2a01:4f8:1c18:8753::1` for `@`, `www`, `admin` and `api`, **DNS only (grey cloud)** so Let's Encrypt can validate over HTTP. After the certificate is issued they may be switched to proxied, with Cloudflare SSL mode **Full (strict)**.

The site is `infra/production/nginx-thulawanews.conf`, installed as `/etc/nginx/sites-available/thulawanews`. Before DNS existed only its port-80 block was enabled (ACME challenge and HTTPS redirect). Once DNS resolves:

```sh
certbot certonly --webroot -w /var/www/lanka-news-paper-acme --cert-name thulawanews.com \
  -d thulawanews.com -d www.thulawanews.com -d admin.thulawanews.com -d api.thulawanews.com
cp /opt/lanka-news-paper/nginx-thulawanews.final.conf /etc/nginx/sites-available/thulawanews
nginx -t && systemctl reload nginx
```

Then update `/opt/lanka-news-paper/.env`: add `https://thulawanews.com`, `https://www.thulawanews.com` and `https://admin.thulawanews.com` to `SNAP_ALLOWED_ORIGINS`, set `SNAP_NEWSLETTER_BASE_URL` and `SNAP_SOCIAL_PUBLIC_BASE_URL` to `https://thulawanews.com`. Restart with `IMAGE_TAG=$(cat .deployed-image) docker compose up -d api worker web admin`.

### lankanewspaper.prabhavalabs.com (previous)

Still served by `infra/production/nginx-site.conf` with certificate `lankanewspaper.prabhavalabs.com`, until it is redirected to the new domain.

## Operations

```sh
cd /opt/lanka-news-paper
docker compose ps
curl --fail http://127.0.0.1:18090/api/v1/health/ready
docker compose logs --tail=100 api worker
```

The deployed commit is recorded in `.deployed-image`. To redeploy a retained image manually, run `./deploy.sh <commit-sha>`.

### Log retention

Administrators set how long logs are kept in **Settings → Log retention** (default 7 days, 1–365). The daily `queue.history.cleanup` job, also runnable from that card, deletes rows older than the period from `crawl_attempts`, `ingestion_runs`, `article_pipeline_runs` (with their steps and logs), `article_pipeline_logs` and `llm_calls`, plus files under `$CODEX_HOME/log` and `$CODEX_HOME/sessions`. It deletes in batches of 5,000 rows and records the counts and any error on the card. Failed fetches for articles that never got a body are kept, because they stop the crawler from retrying those articles. Articles, stories, analyses, posts and the admin audit trail are never deleted. River prunes its own job rows (completed after 24 hours, discarded after 7 days).

Container logs rotate by size: `compose.yaml` caps every service at three 20 MB `json-file` logs. The cap applies when a container is recreated, which every deploy does for changed services. Host logs such as the systemd journal are outside this project and are managed on the server.

## Shared development data

Until the application has end users, a local API may use the deployed database and R2 bucket as a shared test environment. PostgreSQL remains private on the VPS; open a tunnel from a separate terminal:

```sh
ssh -N -o ExitOnForwardFailure=yes \
  -L 127.0.0.1:55433:127.0.0.1:5432 \
  <production-user>@178.104.111.17
```

Copy `infra/local/shared.env.example` to the repository-root `.env`, replace its placeholders with the production database password and bucket-scoped R2 credentials, then run the API and either frontend normally. The production worker remains the only ingestion scheduler.

`SNAP_ENV=shared-development` blocks `pnpm worker:dev` and `pnpm migrate`. Tests and schema migrations must continue to use an isolated local database. Admin actions from the local API still modify shared data intentionally.

## Facebook Page publishing

See [Facebook Page publishing](social-publishing.md) for the new Meta Business app setup, review workflow, caps and failure recovery. These optional server variables are forwarded to API and worker:

```dotenv
SNAP_LOOMCAST_URL=https://loomcast.prabhavalabs.com
SNAP_LOOMCAST_API_KEY=
SNAP_LOOMCAST_CHANNEL_ID=
SNAP_SOCIAL_PUBLIC_BASE_URL=https://lankanewspaper.prabhavalabs.com
SNAP_SOCIAL_CHROME_PATH=/usr/bin/chromium
SNAP_SOCIAL_CHROME_NO_SANDBOX=true
```

Facebook posts go out through Loomcast: set `SNAP_LOOMCAST_API_KEY` to the key of a Loomcast API client with publish access to the Thulawa News Facebook channel (see [social-publishing.md](social-publishing.md)). Both processes must have the same key and use shared media storage (the Compose worker now also mounts `media-data`). Chrome/Chromium and Sinhala fonts are included by the image renderer's Docker configuration. If renderer initialization fails, queue/review still work and publishing waits for a rendered image. No app configuration automatically enables selection or auto-publish; both default to off in migration 54.

The existing admin-origin `/api/` proxy carries the OAuth callback and session cookie. Manual post creation gets the existing long-inference nginx timeout, because it invokes the editorial model. After deployment, verify the Page connection and a reviewed post through the actual Meta app; Development-mode API success does not by itself prove public post visibility.
