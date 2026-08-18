# Remote narration model

The narration worker uses the VPS-hosted `qwen3:8b-q4_K_M` model through
Ollama's OpenAI-compatible API. Ollama listens only on `127.0.0.1:11434`;
Nginx exposes only `/v1/` and requires `Authorization: Bearer <token>`.

The container is limited to two CPUs, 7 GiB RAM, one loaded model, one parallel
request, and a 4,096-token context. These limits protect the other VPS workloads.
Narration calls allow up to ten minutes because CPU-only structured inference can
cross five minutes under load; the application still processes only one at a time.

## DNS

Create an `A` record for `llm.lankanewspaper.prabhavalabs.com` pointing to
`178.104.111.17`. Keep the record DNS-only until the origin certificate is issued.

## HTTPS

After DNS resolves to the VPS, install the certificate and redirect HTTP with:

```sh
certbot --nginx -d llm.lankanewspaper.prabhavalabs.com \
  --non-interactive --agree-tos --register-unsafely-without-email --redirect
```

Certbot uses the existing Let's Encrypt account and installs automatic renewal.

## Secret handling

Store the same random token in `/etc/lanka-llm/api-key` on the VPS and
`SNAP_LLM_API_KEY` in the local `.env`. Never commit either value. The Nginx
deployment renders a root-readable authentication snippet from the VPS secret.

The `vps-ollama` provider is seeded disabled. Enable it only after the authenticated
HTTPS request succeeds; the worker reads the token from `SNAP_LLM_API_KEY`.
