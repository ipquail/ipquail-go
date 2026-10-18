# Docker

The repository includes a multi-stage production `Dockerfile` for the
ipquail-go API backend. The image does not contain a database, migrations,
configuration files, or persistent application data.

The website's static files are not included in the image. As in the nginx
configuration in the [README](README.md), nginx serves `ipquail/public`
directly and proxies `/ip`, `/ptr`, `/asn`, and `/api/` to the Go backend.

## Build the image

```sh
docker build -t ipquail-go .
```

The final Alpine image contains the statically built `ipquail-go` binary and
CA certificates. It runs as the unprivileged user and group `65532:65532` and
declares port 3000. No volumes are required.

## Run behind nginx

ipquail-go listens on port 3000 on the container's network interfaces. Publish
it only on the host loopback interface when nginx runs on the host:

```sh
docker run --name ipquail-go \
  -p 127.0.0.1:3000:3000 \
  --restart unless-stopped \
  -d ipquail-go
```

`-p 3000:3000` also works, but publishes the backend on every host interface by
default. Do not use that form on an Internet-facing host: the backend trusts
`X-Real-IP` from loopback and private-network peers, and Docker's forwarding
can make a direct client's peer appear to be the private bridge gateway.

The included `compose.yaml` provides the equivalent configuration and defaults
to the latest GHCR image:

```sh
docker compose pull
docker compose up -d
```

Pin a release in production by overriding the complete image reference:

```sh
IPQUAIL_IMAGE=ghcr.io/ipquail/ipquail-go:1.2.3 docker compose up -d
```

To exercise a locally built image with the same Compose configuration:

```sh
docker build -t ipquail-go:local .
IPQUAIL_IMAGE=ipquail-go:local docker compose up -d
```

## Health check

The image checks `http://127.0.0.1:3000/health` every 30 seconds. The endpoint
returns `ok` when the HTTP process is responsive and does not perform DNS or
other external dependency checks. Compose inherits this image health check;
inspect its current state with:

```sh
docker compose ps
```

`/health` is intended for Docker and host-local monitoring through the backend
port. The nginx examples in the README deliberately do not expose it through
the public virtual hosts.

If nginx also runs in Docker, put both services on a dedicated private network,
proxy to `ipquail:3000`, and omit `ports` from the backend service. Keep
untrusted containers off that network.

The nginx proxy must continue to overwrite `X-Real-IP` with `$remote_addr`, as
shown in the README. The application is hostname-independent, so domains such
as `ipquail.com` or `ipaddr.ca` are selected through DNS, TLS, and nginx rather
than at image build time.

## Static website files

Deploy `ipquail/public` separately to the directory configured as nginx's
document root. Before deployment, generate the pinned Bootstrap and jQuery
files when needed:

```sh
npm ci
npm run build
```

The frontend currently uses the IPv4-only and IPv6-only `ipquail.com`
hostnames. If another domain should be fully independent, update those static
URLs and provide matching `4.` and `6.` DNS records; this does not require
rebuilding the Go binary.

## Publishing

Pushing a semantic-version tag runs the release workflow. After GoReleaser
publishes the release archives, the workflow builds Linux AMD64 and ARM64
images and pushes them to `ghcr.io/ipquail/ipquail-go`.

For a stable `v1.2.3` release, the image receives these tags:

```sh
ghcr.io/ipquail/ipquail-go:1.2.3
ghcr.io/ipquail/ipquail-go:1.2
ghcr.io/ipquail/ipquail-go:1
ghcr.io/ipquail/ipquail-go:latest
```

Prereleases receive only their full semantic-version tag and do not update
`latest`. Use the full version tag for reproducible production deployments.

The workflow publishes with GitHub's repository-scoped `GITHUB_TOKEN`; no
registry password or additional Actions secret is required. A newly created
GHCR package may initially be private. After the first release, check the
package settings on GitHub and change its visibility to public if anonymous
pulls are required.
