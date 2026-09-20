# HeyBlog Nginx deployment

`heyblog.conf` serves the Web application on `www.heyblog.net` and the service API on
`api.heyblog.net`. The API container binds to `127.0.0.1:10201`; only Nginx is public.

Before enabling the API virtual host:

1. Point `api.heyblog.net` through EdgeOne to the existing origin.
2. Install a certificate at `/etc/letsencrypt/live/api.heyblog.net/`.
3. Run `nginx -t` and reload Nginx gracefully.

`/internal/v1/data-import` is authorized by the application with an INTERNAL API key carrying
`data_import.write`. Nginx does not apply a source-IP allowlist. Restricting direct origin access to
trusted edge infrastructure is separate deployment hardening and is not part of this configuration.

Browser JavaScript may call only future explicitly anonymous read routes under `/v1/*`. Service API
keys are server-side credentials and must never be embedded in browser bundles. Keep API CORS
credentials disabled; preflight intentionally rejects `Authorization` and all methods except
`GET`/`HEAD`.
