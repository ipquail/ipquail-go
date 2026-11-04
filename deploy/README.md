# Deployment files

These examples run the Go backend in Docker and use a web server installed on
the host for TLS and static files. The backend is published only on loopback;
the proxy configs overwrite `X-Real-IP` before forwarding API requests.

## Start the backend

From this directory, copy `.env.example` to `.env`, set `IPQUAIL_VERSION` to a
release tag, then start the service:

```sh
cp .env.example .env
# Edit .env and set IPQUAIL_VERSION to the release you want to deploy.
docker compose up -d
```

The image's built-in health check polls `/health`. Check its status with:

```sh
docker compose ps
```

There is no application configuration, migration step, or persistent data to
mount. The service runs read-only, drops Linux capabilities, and listens on
host loopback at `127.0.0.1:3000` for the host web server.

## Web server samples

The samples expect the static files at `/var/www/ipquail/public`; copy
`../ipquail/public` there or edit the document root in the config. Replace the
sample hostnames with your own.

- `nginx.conf`: HTTP server sample; add the TLS directives for your
  certificate and repeat its locations in the TLS server.
- `apache.conf`: HTTP virtual host sample; configure TLS separately.
- `Caddyfile`: HTTP and HTTPS site blocks; Caddy obtains certificates
  automatically when DNS and firewall settings allow it.

Port 80 intentionally remains available without an HTTPS redirect. The proxy
samples keep `/metrics` restricted to local clients and do not expose
`/health` publicly.
